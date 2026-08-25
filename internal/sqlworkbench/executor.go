package sqlworkbench

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const DefaultMaxRows = 10000

type DataAPI interface {
	ExecuteSQLData(context.Context, model.SQLQueryRequest) (model.SQLQueryResult, error)
}

type ExecutorOptions struct {
	AuthProvider AuthProvider
	DataAPI      DataAPI
	Prompt       PasswordPrompt
	PGPassFiles  []string
	AllowWrites  bool
	AWSProfile   string
	AWSRegion    string
	MaxRows      int
}

type Executor struct {
	optionsMu sync.RWMutex
	options   ExecutorOptions
	mu        sync.Mutex
	sessions  map[string]*directSession
}

type directSession struct {
	profileKey string
	connection *pgx.Conn
	tunnel     *Tunnel
	mu         sync.Mutex
}

func NewExecutor(options ExecutorOptions) *Executor {
	if options.MaxRows <= 0 {
		options.MaxRows = DefaultMaxRows
	}
	return &Executor{options: options, sessions: make(map[string]*directSession)}
}

// SetPolicy applies runtime safety and credential-source settings to future
// executions without discarding healthy database sessions.
func (e *Executor) SetPolicy(allowWrites bool, pgpassFiles []string) {
	e.optionsMu.Lock()
	e.options.AllowWrites = allowWrites
	e.options.PGPassFiles = append(e.options.PGPassFiles[:0], pgpassFiles...)
	e.optionsMu.Unlock()
}

func (e *Executor) Execute(ctx context.Context, profile config.SQLConnection, source string, tabAllowsWrites bool) ([]model.SQLQueryResult, error) {
	options := e.optionsSnapshot()
	statements := SplitStatements(source)
	if len(statements) == 0 {
		return nil, fmt.Errorf("query is empty")
	}
	readOnlyErr := ValidateReadOnly(source)
	if readOnlyErr != nil && !(options.AllowWrites && tabAllowsWrites) {
		if options.AllowWrites {
			return nil, fmt.Errorf("%w; enable writes for this tab to continue", readOnlyErr)
		}
		return nil, fmt.Errorf("%w; writes are disabled globally in settings", readOnlyErr)
	}
	resolved, err := ResolveConnection(ctx, profile, options.PGPassFiles, options.AuthProvider, options.Prompt)
	if err != nil {
		return nil, err
	}
	if resolved.DataAPI {
		return e.executeDataAPI(ctx, resolved, statements, options)
	}
	session, err := e.session(ctx, resolved, options)
	if err != nil {
		return nil, err
	}
	results, err := session.execute(ctx, statements, options.MaxRows)
	if err == nil {
		return results, nil
	}
	// A superseded GUI catalog/inspection request is expected cancellation,
	// not evidence that the shared PostgreSQL session is unhealthy.
	if ctx.Err() != nil {
		return results, ctx.Err()
	}
	if readOnlyErr != nil {
		// Never automatically repeat a statement that may have changed data.
		return results, err
	}
	// A dropped connection is retried once after credentials are freshly
	// resolved. This is especially useful for expired IAM tokens and tunnels.
	e.dropSession(profile.Name)
	resolved, resolveErr := ResolveConnection(ctx, profile, options.PGPassFiles, options.AuthProvider, options.Prompt)
	if resolveErr != nil {
		return results, fmt.Errorf("query failed and credentials could not be refreshed: %v (refresh: %w)", err, resolveErr)
	}
	session, connectErr := e.session(ctx, resolved, options)
	if connectErr != nil {
		return results, fmt.Errorf("query failed and reconnect did not succeed: %v (reconnect: %w)", err, connectErr)
	}
	return session.execute(ctx, statements, options.MaxRows)
}

func (e *Executor) Ping(ctx context.Context, profile config.SQLConnection) error {
	options := e.optionsSnapshot()
	resolved, err := ResolveConnection(ctx, profile, options.PGPassFiles, options.AuthProvider, options.Prompt)
	if err != nil {
		return err
	}
	if resolved.DataAPI {
		return nil
	}
	session, err := e.session(ctx, resolved, options)
	if err != nil {
		return err
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.connection.Ping(ctx)
}

func (e *Executor) Reconnect(ctx context.Context, profile config.SQLConnection) error {
	e.dropSession(profile.Name)
	return e.Ping(ctx, profile)
}

func (e *Executor) Close() {
	e.mu.Lock()
	sessions := e.sessions
	e.sessions = make(map[string]*directSession)
	e.mu.Unlock()
	for _, session := range sessions {
		session.close()
	}
}

func (e *Executor) executeDataAPI(ctx context.Context, connection ResolvedConnection, statements []Statement, options ExecutorOptions) ([]model.SQLQueryResult, error) {
	if options.DataAPI == nil {
		return nil, fmt.Errorf("RDS Data API execution is unavailable")
	}
	if strings.TrimSpace(connection.Profile.ResourceARN) == "" || strings.TrimSpace(connection.Profile.SecretARN) == "" {
		return nil, fmt.Errorf("data API connections require resource_arn and secret_arn")
	}
	results := make([]model.SQLQueryResult, 0, len(statements))
	for _, statement := range statements {
		result, err := options.DataAPI.ExecuteSQLData(ctx, model.SQLQueryRequest{ResourceARN: connection.Profile.ResourceARN,
			SecretARN: connection.Profile.SecretARN, Database: connection.Database, SQL: statement.SQL, MaxRows: options.MaxRows})
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

func (e *Executor) session(ctx context.Context, connection ResolvedConnection, options ExecutorOptions) (*directSession, error) {
	key := connectionKey(connection)
	e.mu.Lock()
	if session := e.sessions[connection.Profile.Name]; session != nil && session.profileKey == key {
		e.mu.Unlock()
		return session, nil
	}
	stale := e.sessions[connection.Profile.Name]
	delete(e.sessions, connection.Profile.Name)
	e.mu.Unlock()
	if stale != nil {
		stale.close()
	}
	session, err := e.connect(ctx, connection, key, options)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	if existing := e.sessions[connection.Profile.Name]; existing != nil {
		e.mu.Unlock()
		session.close()
		return existing, nil
	}
	e.sessions[connection.Profile.Name] = session
	e.mu.Unlock()
	return session, nil
}

func (e *Executor) connect(ctx context.Context, connection ResolvedConnection, key string, options ExecutorOptions) (*directSession, error) {
	connectionURL := &url.URL{Scheme: "postgres", Host: net.JoinHostPort(connection.Host, strconv.Itoa(connection.Port)), Path: "/" + connection.Database,
		User: url.UserPassword(connection.User, connection.Password)}
	query := connectionURL.Query()
	query.Set("sslmode", connection.SSLMode)
	if connection.SSLRootCert != "" {
		query.Set("sslrootcert", connection.SSLRootCert)
	}
	if connection.Profile.ConnectSecs > 0 {
		query.Set("connect_timeout", strconv.Itoa(connection.Profile.ConnectSecs))
	}
	query.Set("application_name", "e9s-sql-workbench")
	connectionURL.RawQuery = query.Encode()
	pgxConfig, err := pgx.ParseConfig(connectionURL.String())
	if err != nil {
		return nil, err
	}
	var tunnel *Tunnel
	if connection.Profile.SSMTunnel != nil {
		tunnel, err = StartSSMTunnel(ctx, *connection.Profile.SSMTunnel, connection.Host, connection.Port,
			TunnelOptions{AWSProfile: options.AWSProfile, AWSRegion: options.AWSRegion})
		if err != nil {
			return nil, err
		}
		localAddress := tunnel.LocalAddress
		pgxConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", localAddress)
		}
	}
	client, err := pgx.ConnectConfig(ctx, pgxConfig)
	if err != nil {
		if tunnel != nil {
			tunnel.Close()
		}
		return nil, err
	}
	return &directSession{profileKey: key, connection: client, tunnel: tunnel}, nil
}

func (e *Executor) optionsSnapshot() ExecutorOptions {
	e.optionsMu.RLock()
	defer e.optionsMu.RUnlock()
	options := e.options
	options.PGPassFiles = append([]string(nil), e.options.PGPassFiles...)
	return options
}

func (e *Executor) dropSession(profileName string) {
	e.mu.Lock()
	session := e.sessions[profileName]
	delete(e.sessions, profileName)
	e.mu.Unlock()
	if session != nil {
		session.close()
	}
}

func (s *directSession) execute(ctx context.Context, statements []Statement, maxRows int) ([]model.SQLQueryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	results := make([]model.SQLQueryResult, 0, len(statements))
	for _, statement := range statements {
		started := time.Now()
		rows, err := s.connection.Query(ctx, statement.SQL)
		if err != nil {
			return results, err
		}
		result := model.SQLQueryResult{DurationMS: time.Since(started).Milliseconds()}
		for _, field := range rows.FieldDescriptions() {
			result.Columns = append(result.Columns, field.Name)
		}
		for rows.Next() {
			if len(result.Rows) >= maxRows {
				result.Truncated = true
				rows.Close()
				break
			}
			values, err := rows.Values()
			if err != nil {
				rows.Close()
				return results, err
			}
			formatted := make([]string, len(values))
			for index, value := range values {
				formatted[index] = SQLValueString(value)
			}
			result.Rows = append(result.Rows, formatted)
		}
		if err := rows.Err(); err != nil {
			return results, err
		}
		result.CommandTag = rows.CommandTag().String()
		result.RowsAffected = rows.CommandTag().RowsAffected()
		result.DurationMS = time.Since(started).Milliseconds()
		results = append(results, result)
	}
	return results, nil
}

func (s *directSession) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.connection != nil {
		_ = s.connection.Close(context.Background())
	}
	if s.tunnel != nil {
		s.tunnel.Close()
	}
}

func connectionKey(connection ResolvedConnection) string {
	encoded, _ := json.Marshal(struct {
		Host, Database, User, SSLMode, SSLRootCert, Auth string
		Port                                             int
		Tunnel                                           *config.SSMTunnel
	}{connection.Host, connection.Database, connection.User, connection.SSLMode, connection.SSLRootCert, connection.AuthMode, connection.Port, connection.Profile.SSMTunnel})
	return string(encoded)
}

func SQLValueString(value any) string {
	switch typed := value.(type) {
	case nil:
		return "NULL"
	case string:
		return typed
	case []byte:
		return string(typed)
	case [16]byte:
		// pgx decodes PostgreSQL uuid values to their raw fixed-size byte
		// representation. Render that representation in the canonical form
		// users expect instead of allowing JSON to turn it into 16 integers.
		return pgtype.UUID{Bytes: typed, Valid: true}.String()
	case pgtype.UUID:
		if !typed.Valid {
			return "NULL"
		}
		return typed.String()
	case time.Time:
		return typed.Format(time.RFC3339Nano)
	default:
		if encoded, err := json.Marshal(typed); err == nil && string(encoded) != "{}" {
			return string(encoded)
		}
		return fmt.Sprint(typed)
	}
}
