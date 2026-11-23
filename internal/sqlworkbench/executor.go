package sqlworkbench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
	"github.com/jackc/pgx/v5/pgtype"
)

const DefaultMaxRows = 10000

var (
	// ErrNoTransaction is returned when a commit or rollback finds no
	// open transaction.
	ErrNoTransaction = errors.New("no transaction is open in this tab")
	// ErrTransactionOpen is returned when an action would discard a tab's
	// open transaction and the caller did not agree to that.
	ErrTransactionOpen = errors.New("a transaction is open in this tab; commit or roll it back first")
)

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

// Executor runs workbench SQL. Each query tab gets its own PostgreSQL
// session, keyed by the tab's ID, so a transaction never spans tabs; the
// object browser uses a separate read-only session per connection profile.
type Executor struct {
	optionsMu sync.RWMutex
	options   ExecutorOptions
	mu        sync.Mutex
	sessions  map[string]*directSession
}

// RunRequest is one execution in a query tab.
type RunRequest struct {
	Profile config.SQLConnection
	// SessionID identifies the query tab whose session runs the SQL.
	SessionID string
	Source    string
	// AllowWrites is the tab's write break-glass control.
	AllowWrites bool
}

// StatementFailure describes the statement that stopped a run.
type StatementFailure struct {
	Index   int // position among the run's statements; -1 before any ran
	SQL     string
	Message string
	Code    string // SQLSTATE, when PostgreSQL reported one
	Line    int    // line in the submitted source, when known
}

// RunOutcome is everything a run produced, including results from the
// statements that completed before a failure.
type RunOutcome struct {
	Statements int
	Results    []model.SQLQueryResult
	Failure    *StatementFailure
	// Transaction is the tab's state after e9s settled the session.
	Transaction TxState
	// Events lists what e9s did on the session's behalf, such as rolling
	// back a failed transaction.
	Events []string
	// ConnectionLost reports that the session's connection is gone.
	ConnectionLost bool
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

func tabSessionKey(id string) string          { return "tab\x00" + id }
func catalogSessionKey(profile string) string { return "catalog\x00" + profile }

func writeGuard(readOnlyErr error, globalWrites, tabWrites bool) error {
	if readOnlyErr == nil || (globalWrites && tabWrites) {
		return nil
	}
	if globalWrites {
		return fmt.Errorf("%w; enable writes for this tab to continue", readOnlyErr)
	}
	return fmt.Errorf("%w; writes are disabled globally in settings", readOnlyErr)
}

// Run executes source in the tab's session. A returned error means nothing
// ran: the SQL was refused or no connection could be made. A statement that
// fails is reported in the outcome together with the results before it.
func (e *Executor) Run(ctx context.Context, request RunRequest) (RunOutcome, error) {
	options := e.optionsSnapshot()
	statements := SplitStatements(request.Source)
	if len(statements) == 0 {
		return RunOutcome{}, fmt.Errorf("query is empty")
	}
	if strings.TrimSpace(request.SessionID) == "" {
		return RunOutcome{}, fmt.Errorf("SQL runs need a query tab")
	}
	readOnlyErr := ValidateReadOnly(request.Source)
	if err := writeGuard(readOnlyErr, options.AllowWrites, request.AllowWrites); err != nil {
		return RunOutcome{Statements: len(statements)}, err
	}
	key := tabSessionKey(request.SessionID)
	session := e.existingSession(key)
	if session == nil || session.txState() == TxIdle {
		// An open transaction keeps its session: re-resolving credentials
		// could only fail or replace the connection that holds it.
		resolved, err := ResolveConnection(ctx, request.Profile, options.PGPassFiles, options.AuthProvider, options.Prompt)
		if err != nil {
			return RunOutcome{Statements: len(statements)}, err
		}
		if resolved.DataAPI {
			return e.runDataAPI(ctx, resolved, statements, request.Source, options)
		}
		session, err = e.acquire(ctx, key, resolved, false, options)
		if err != nil {
			return RunOutcome{Statements: len(statements)}, err
		}
	}
	before := session.txState()
	outcome := session.run(ctx, statements, request.Source, request.AllowWrites, options.MaxRows)
	if !outcome.ConnectionLost {
		return outcome, nil
	}
	e.forget(key, session)
	// Run read-only SQL again on a fresh connection, which is useful after
	// an expired IAM token or tunnel. Never repeat SQL that may have changed
	// data, and never when a transaction was open: its work is gone, and
	// repeating only the later statements would run them outside it.
	if readOnlyErr != nil || before != TxIdle || outcome.Transaction != TxIdle || ctx.Err() != nil || hasTransactionControl(statements) {
		return outcome, nil
	}
	resolved, err := ResolveConnection(ctx, request.Profile, options.PGPassFiles, options.AuthProvider, options.Prompt)
	if err != nil {
		return outcome, nil
	}
	session, err = e.acquire(ctx, key, resolved, false, options)
	if err != nil {
		return outcome, nil
	}
	retried := session.run(ctx, statements, request.Source, request.AllowWrites, options.MaxRows)
	retried.Events = append([]string{"The connection was lost, so e9s reconnected and ran the read-only statements again."}, retried.Events...)
	if retried.ConnectionLost {
		e.forget(key, session)
	}
	return retried, nil
}

func hasTransactionControl(statements []Statement) bool {
	for _, statement := range statements {
		if IsTransactionControl(statement.SQL) {
			return true
		}
	}
	return false
}

// runDataAPI runs each statement as its own Data API call. The Data API
// gives every call its own transaction, so transaction control cannot span
// statements and is refused instead of silently doing nothing.
func (e *Executor) runDataAPI(ctx context.Context, connection ResolvedConnection, statements []Statement, source string, options ExecutorOptions) (RunOutcome, error) {
	if options.DataAPI == nil {
		return RunOutcome{}, fmt.Errorf("RDS Data API execution is unavailable")
	}
	if strings.TrimSpace(connection.Profile.ResourceARN) == "" || strings.TrimSpace(connection.Profile.SecretARN) == "" {
		return RunOutcome{}, fmt.Errorf("data API connections require resource_arn and secret_arn")
	}
	for _, statement := range statements {
		if IsTransactionControl(statement.SQL) {
			return RunOutcome{Statements: len(statements)}, fmt.Errorf("line %d: the RDS Data API runs each statement in its own transaction, so %s cannot span statements; use a direct connection for transactional scripts",
				StatementLine(source, statement.Start), strings.ToUpper(SQLWords(statement.SQL)[0]))
		}
	}
	outcome := RunOutcome{Statements: len(statements)}
	for index, statement := range statements {
		result, err := options.DataAPI.ExecuteSQLData(ctx, model.SQLQueryRequest{ResourceARN: connection.Profile.ResourceARN,
			SecretARN: connection.Profile.SecretARN, Database: connection.Database, SQL: statement.SQL, MaxRows: options.MaxRows})
		if err != nil {
			outcome.Failure = newStatementFailure(index, statement, source, err)
			break
		}
		result.SQL = statement.SQL
		outcome.Results = append(outcome.Results, result)
	}
	return outcome, nil
}

// EndTransaction commits or rolls back the tab's open transaction and
// returns PostgreSQL's command tag. COMMIT of a failed transaction reports
// ROLLBACK.
func (e *Executor) EndTransaction(sessionID string, commit bool) (string, error) {
	key := tabSessionKey(sessionID)
	session := e.existingSession(key)
	if session == nil {
		return "", ErrNoTransaction
	}
	tag, err := session.endTransaction(commit)
	if session.closed.Load() {
		e.forget(key, session)
	}
	return tag, err
}

// SessionStatus reports the tab's session state. It never waits for a run.
func (e *Executor) SessionStatus(sessionID string) SessionStatus {
	session := e.existingSession(tabSessionKey(sessionID))
	if session == nil {
		return SessionStatus{}
	}
	return session.status()
}

// OpenTransactions lists the query tabs whose sessions hold an open
// transaction.
func (e *Executor) OpenTransactions() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var ids []string
	for key, session := range e.sessions {
		if id, ok := strings.CutPrefix(key, "tab\x00"); ok && session.txState() == TxOpen {
			ids = append(ids, id)
		}
	}
	return ids
}

// CloseSession closes the tab's connection. PostgreSQL rolls back any open
// transaction when the connection ends.
func (e *Executor) CloseSession(sessionID string) {
	key := tabSessionKey(sessionID)
	e.mu.Lock()
	session := e.sessions[key]
	delete(e.sessions, key)
	e.mu.Unlock()
	if session != nil {
		session.close()
	}
}

// Reconnect replaces the tab's connection and the profile's object browser
// connection. It refuses to discard an open transaction unless asked to.
func (e *Executor) Reconnect(ctx context.Context, profile config.SQLConnection, sessionID string, discardTransaction bool) error {
	if session := e.existingSession(tabSessionKey(sessionID)); session != nil && session.txState() == TxOpen && !discardTransaction {
		return ErrTransactionOpen
	}
	e.CloseSession(sessionID)
	e.dropSession(catalogSessionKey(profile.Name))
	options := e.optionsSnapshot()
	resolved, err := ResolveConnection(ctx, profile, options.PGPassFiles, options.AuthProvider, options.Prompt)
	if err != nil {
		return err
	}
	if resolved.DataAPI {
		return nil
	}
	session, err := e.acquire(ctx, tabSessionKey(sessionID), resolved, false, options)
	if err != nil {
		return err
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.connection.Ping(ctx)
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

// catalogQuery runs one internal read-only statement on the profile's
// object browser session, which never shares a query tab's transaction.
func (e *Executor) catalogQuery(ctx context.Context, profile config.SQLConnection, query string) (model.SQLQueryResult, error) {
	options := e.optionsSnapshot()
	statements := SplitStatements(query)
	if len(statements) == 0 {
		return model.SQLQueryResult{}, fmt.Errorf("query is empty")
	}
	if err := ValidateReadOnly(query); err != nil {
		return model.SQLQueryResult{}, err
	}
	key := catalogSessionKey(profile.Name)
	for attempt := 0; ; attempt++ {
		resolved, err := ResolveConnection(ctx, profile, options.PGPassFiles, options.AuthProvider, options.Prompt)
		if err != nil {
			return model.SQLQueryResult{}, err
		}
		var outcome RunOutcome
		if resolved.DataAPI {
			outcome, err = e.runDataAPI(ctx, resolved, statements, query, options)
			if err != nil {
				return model.SQLQueryResult{}, err
			}
		} else {
			session, err := e.acquire(ctx, key, resolved, true, options)
			if err != nil {
				return model.SQLQueryResult{}, err
			}
			outcome = session.run(ctx, statements, query, false, options.MaxRows)
			if outcome.ConnectionLost {
				e.forget(key, session)
				if attempt == 0 && ctx.Err() == nil {
					continue
				}
			}
		}
		if outcome.Failure != nil {
			return model.SQLQueryResult{}, errors.New(outcome.Failure.Message)
		}
		if len(outcome.Results) == 0 {
			return model.SQLQueryResult{}, nil
		}
		return outcome.Results[len(outcome.Results)-1], nil
	}
}

func (e *Executor) existingSession(key string) *directSession {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sessions[key]
}

// acquire returns the session for key, connecting when there is none or its
// connection is gone or its settings changed.
func (e *Executor) acquire(ctx context.Context, key string, connection ResolvedConnection, catalog bool, options ExecutorOptions) (*directSession, error) {
	profileKey := connectionKey(connection)
	e.mu.Lock()
	stale := e.sessions[key]
	if stale != nil {
		if stale.profileKey == profileKey && stale.alive() {
			e.mu.Unlock()
			return stale, nil
		}
		if stale.txState() == TxOpen && stale.alive() {
			e.mu.Unlock()
			return nil, fmt.Errorf("the settings for %s changed while a transaction is open; commit or roll it back first", connection.Profile.Name)
		}
		delete(e.sessions, key)
	}
	e.mu.Unlock()
	if stale != nil {
		stale.close()
	}
	session, err := connectSession(ctx, connection, profileKey, catalog, options)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	if existing := e.sessions[key]; existing != nil {
		e.mu.Unlock()
		session.close()
		return existing, nil
	}
	e.sessions[key] = session
	e.mu.Unlock()
	return session, nil
}

// forget removes session from the map if it is still the one stored.
func (e *Executor) forget(key string, session *directSession) {
	e.mu.Lock()
	if e.sessions[key] == session {
		delete(e.sessions, key)
	}
	e.mu.Unlock()
	session.close()
}

func (e *Executor) dropSession(key string) {
	e.mu.Lock()
	session := e.sessions[key]
	delete(e.sessions, key)
	e.mu.Unlock()
	if session != nil {
		session.close()
	}
}

func (e *Executor) optionsSnapshot() ExecutorOptions {
	e.optionsMu.RLock()
	defer e.optionsMu.RUnlock()
	options := e.options
	options.PGPassFiles = append([]string(nil), e.options.PGPassFiles...)
	return options
}

func connectionKey(connection ResolvedConnection) string {
	encoded, _ := json.Marshal(struct {
		Host, Database, User, SSLMode, SSLRootCert, Auth string
		Port                                             int
		IdleInTransactionSecs                            int
		Tunnel                                           *config.SSMTunnel
	}{connection.Host, connection.Database, connection.User, connection.SSLMode, connection.SSLRootCert, connection.AuthMode, connection.Port,
		connection.Profile.IdleInTransactionSecs, connection.Profile.SSMTunnel})
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
