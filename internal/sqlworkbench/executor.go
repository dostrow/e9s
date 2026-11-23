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

// DataAPITransactions is implemented by Data API clients that can hold a
// transaction across statements.
type DataAPITransactions interface {
	BeginSQLDataTransaction(context.Context, model.SQLQueryRequest) (string, error)
	EndSQLDataTransaction(ctx context.Context, request model.SQLQueryRequest, commit bool) (string, error)
}

// DataAPITransactionTimeout is how long the Data API keeps an idle
// transaction before ending it.
const DataAPITransactionTimeout = 3 * time.Minute

// dataAPITransaction is a query tab's open Data API transaction.
type dataAPITransaction struct {
	request model.SQLQueryRequest // with TransactionID set
	since   time.Time
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
	// dataTx holds query tabs' open Data API transactions by tab ID.
	dataTx map[string]*dataAPITransaction
}

// RunRequest is one execution in a query tab.
type RunRequest struct {
	Profile config.SQLConnection
	// SessionID identifies the query tab whose session runs the SQL.
	SessionID string
	Source    string
	// AllowWrites is the tab's write break-glass control.
	AllowWrites bool
	Mode        RunMode
	// Progress, if set, is called before each statement runs, on the
	// goroutine running the SQL.
	Progress func(Progress)
}

// Progress reports which statement a run is executing.
type Progress struct {
	Index     int // 0-based among the run's statements
	Total     int
	Line      int
	Statement string
}

// StatementFailure describes the statement that stopped a run.
type StatementFailure struct {
	Index   int // position among the run's statements; -1 before any ran
	SQL     string
	Heading string
	Notices []string
	Message string
	Code    string // SQLSTATE, when PostgreSQL reported one
	Line    int    // line in the submitted source, when known
}

// RunOutcome is everything a run produced, including results from the
// statements that completed before a failure.
type RunOutcome struct {
	Mode       RunMode
	Started    time.Time
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
	// TrailingHeading is psql \echo text after the script's last statement.
	TrailingHeading string
	// Cancelled reports that the run was cancelled before it finished.
	Cancelled bool
}

func NewExecutor(options ExecutorOptions) *Executor {
	if options.MaxRows <= 0 {
		options.MaxRows = DefaultMaxRows
	}
	return &Executor{options: options, sessions: make(map[string]*directSession), dataTx: make(map[string]*dataAPITransaction)}
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
	started := time.Now()
	if request.Progress != nil {
		ctx = context.WithValue(ctx, progressKey{}, request.Progress)
	}
	script, err := ParseScript(request.Source)
	if err != nil {
		return RunOutcome{Started: started, Mode: request.Mode}, err
	}
	outcome, err := e.run(ctx, request, script.Statements)
	if ctx.Err() != nil && (outcome.Failure != nil || err != nil) {
		outcome.Cancelled = true
	}
	outcome.Started = started
	outcome.Mode = request.Mode
	outcome.TrailingHeading = script.TrailingHeading
	if event := script.IgnoredEvent(); event != "" && err == nil {
		outcome.Events = append([]string{event}, outcome.Events...)
	}
	return outcome, err
}

func (e *Executor) run(ctx context.Context, request RunRequest, statements []Statement) (RunOutcome, error) {
	options := e.optionsSnapshot()
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
	var plan transactionPlan
	if request.Mode != RunAsWritten {
		var err error
		if plan, err = planTransaction(statements, request.Source); err != nil {
			return RunOutcome{Statements: len(statements), Mode: request.Mode}, err
		}
		if e.SessionStatus(request.SessionID).State == TxOpen {
			return RunOutcome{Statements: len(statements), Mode: request.Mode},
				fmt.Errorf("a transaction is already open in this tab; commit or roll it back before a %s", request.Mode)
		}
	}
	if transaction := e.dataTransaction(request.SessionID); transaction != nil {
		// Statements join the tab's open Data API transaction.
		return e.runDataAPI(ctx, request.SessionID, transaction.request, statements, request.Source, options)
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
			base, err := dataAPIRequest(resolved, options)
			if err != nil {
				return RunOutcome{}, err
			}
			if request.Mode != RunAsWritten {
				return e.runDataAPIInTransaction(ctx, request.SessionID, base, plan, request.Source, options, request.Mode)
			}
			return e.runDataAPI(ctx, request.SessionID, base, statements, request.Source, options)
		}
		session, err = e.acquire(ctx, key, resolved, false, options)
		if err != nil {
			return RunOutcome{Statements: len(statements)}, err
		}
	}
	if request.Mode != RunAsWritten {
		outcome := session.runInTransaction(ctx, plan, request.Source, request.AllowWrites, options.MaxRows, request.Mode)
		if outcome.ConnectionLost {
			e.forget(key, session)
		}
		return outcome, nil
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

// progressKey carries a run's Progress callback to the loops that execute
// its statements.
type progressKey struct{}

// reportProgress tells the run's caller which statement is about to run.
func reportProgress(ctx context.Context, index, total int, statement Statement, source string) {
	if report, ok := ctx.Value(progressKey{}).(func(Progress)); ok {
		report(Progress{Index: index, Total: total, Line: StatementLine(source, statement.Start), Statement: FirstLine(statement.SQL)})
	}
}

func hasTransactionControl(statements []Statement) bool {
	for _, statement := range statements {
		if IsTransactionControl(statement.SQL) {
			return true
		}
	}
	return false
}

func dataAPIRequest(connection ResolvedConnection, options ExecutorOptions) (model.SQLQueryRequest, error) {
	if options.DataAPI == nil {
		return model.SQLQueryRequest{}, fmt.Errorf("RDS Data API execution is unavailable")
	}
	if strings.TrimSpace(connection.Profile.ResourceARN) == "" || strings.TrimSpace(connection.Profile.SecretARN) == "" {
		return model.SQLQueryRequest{}, fmt.Errorf("data API connections require resource_arn and secret_arn")
	}
	return model.SQLQueryRequest{ResourceARN: connection.Profile.ResourceARN, SecretARN: connection.Profile.SecretARN,
		Database: connection.Database, MaxRows: options.MaxRows}, nil
}

// runDataAPI runs each statement as its own Data API call, inside the tab's
// open Data API transaction when base names one. Transaction control
// statements cannot span Data API calls, so they are refused instead of
// silently doing nothing.
func (e *Executor) runDataAPI(ctx context.Context, sessionID string, base model.SQLQueryRequest, statements []Statement, source string, options ExecutorOptions) (RunOutcome, error) {
	for _, statement := range statements {
		if IsTransactionControl(statement.SQL) {
			return RunOutcome{Statements: len(statements)}, fmt.Errorf("line %d: the RDS Data API runs each statement on its own, so %s cannot span statements; use Dry run or Run in transaction, or a direct connection",
				StatementLine(source, statement.Start), strings.ToUpper(SQLWords(statement.SQL)[0]))
		}
	}
	outcome := RunOutcome{Statements: len(statements)}
	if base.TransactionID != "" {
		outcome.Transaction = TxOpen
	}
	e.runDataAPIStatements(ctx, base, statements, source, options, &outcome)
	if base.TransactionID != "" && outcome.Failure != nil {
		// A failed statement ends the usefulness of the transaction, as it
		// would on a direct connection: roll it back.
		outcome.Transaction = TxIdle
		if _, err := e.endDataTransaction(sessionID, false); err != nil {
			outcome.Events = append(outcome.Events, "The transaction failed and e9s could not roll it back ("+err.Error()+"); the Data API ends it after "+DataAPITransactionTimeout.String()+" idle.")
		} else {
			outcome.Events = append(outcome.Events, "The transaction failed, so e9s rolled it back: none of its changes were kept and its locks were released.")
		}
	}
	return outcome, nil
}

func (e *Executor) runDataAPIStatements(ctx context.Context, base model.SQLQueryRequest, statements []Statement, source string, options ExecutorOptions, outcome *RunOutcome) {
	for index, statement := range statements {
		reportProgress(ctx, index, len(statements), statement, source)
		request := base
		request.SQL = statement.SQL
		result, err := options.DataAPI.ExecuteSQLData(ctx, request)
		if err != nil {
			outcome.Failure = newStatementFailure(index, statement, source, err)
			return
		}
		result.SQL = statement.SQL
		result.Line = StatementLine(source, statement.Start)
		result.Heading = statement.Heading
		outcome.Results = append(outcome.Results, result)
	}
}

// runDataAPIInTransaction runs plan inside a Data API transaction.
func (e *Executor) runDataAPIInTransaction(ctx context.Context, sessionID string, base model.SQLQueryRequest, plan transactionPlan, source string, options ExecutorOptions, mode RunMode) (RunOutcome, error) {
	transactions, ok := options.DataAPI.(DataAPITransactions)
	if !ok {
		return RunOutcome{}, fmt.Errorf("this Data API client cannot hold a transaction across statements")
	}
	if !strings.EqualFold(strings.Join(SQLWords(plan.begin), " "), "begin") && !strings.EqualFold(strings.Join(SQLWords(plan.begin), " "), "start transaction") {
		return RunOutcome{}, fmt.Errorf("the RDS Data API cannot start a transaction with options; remove them from %q", plan.begin)
	}
	outcome := RunOutcome{Statements: len(plan.body), Mode: mode}
	if event := plan.event(mode); event != "" {
		outcome.Events = append(outcome.Events, event)
	}
	id, err := transactions.BeginSQLDataTransaction(ctx, base)
	if err != nil {
		return outcome, fmt.Errorf("start the Data API transaction: %w", err)
	}
	transaction := base
	transaction.TransactionID = id
	e.runDataAPIStatements(ctx, transaction, plan.body, source, options, &outcome)
	if mode == RunInTransaction && outcome.Failure == nil {
		e.mu.Lock()
		e.dataTx[sessionID] = &dataAPITransaction{request: transaction, since: time.Now()}
		e.mu.Unlock()
		outcome.Transaction = TxOpen
		outcome.Events = append(outcome.Events, fmt.Sprintf("The transaction is still open. Review the results, then commit or roll it back; the Data API ends it after %s idle.", DataAPITransactionTimeout))
		return outcome, nil
	}
	cleanup, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	if _, err := transactions.EndSQLDataTransaction(cleanup, transaction, false); err != nil {
		outcome.Events = append(outcome.Events, "e9s could not roll back the Data API transaction ("+err.Error()+"); the Data API ends it after "+DataAPITransactionTimeout.String()+" idle.")
	} else if outcome.Failure != nil {
		outcome.Events = append(outcome.Events, "The transaction failed, so e9s rolled it back: none of its changes were kept and its locks were released.")
	} else {
		outcome.Events = append(outcome.Events, "Dry run: e9s rolled the transaction back, so none of its changes were kept.")
	}
	return outcome, nil
}

func (e *Executor) dataTransaction(sessionID string) *dataAPITransaction {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.dataTx[sessionID]
}

// endDataTransaction commits or rolls back the tab's Data API transaction.
func (e *Executor) endDataTransaction(sessionID string, commit bool) (string, error) {
	e.mu.Lock()
	transaction := e.dataTx[sessionID]
	delete(e.dataTx, sessionID)
	e.mu.Unlock()
	if transaction == nil {
		return "", ErrNoTransaction
	}
	transactions, ok := e.optionsSnapshot().DataAPI.(DataAPITransactions)
	if !ok {
		return "", fmt.Errorf("this Data API client cannot end a transaction")
	}
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	status, err := transactions.EndSQLDataTransaction(ctx, transaction.request, commit)
	if err != nil {
		return "", fmt.Errorf("end the Data API transaction: %w", err)
	}
	return status, nil
}

// EndTransaction commits or rolls back the tab's open transaction and
// returns PostgreSQL's command tag. COMMIT of a failed transaction reports
// ROLLBACK.
func (e *Executor) EndTransaction(sessionID string, commit bool) (string, error) {
	if e.dataTransaction(sessionID) != nil {
		return e.endDataTransaction(sessionID, commit)
	}
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
	if transaction := e.dataTransaction(sessionID); transaction != nil {
		return SessionStatus{Connected: true, State: TxOpen, Since: transaction.since, IdleTimeout: DataAPITransactionTimeout}
	}
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
	for id := range e.dataTx {
		ids = append(ids, id)
	}
	return ids
}

// CloseSession closes the tab's connection. PostgreSQL rolls back any open
// transaction when the connection ends.
func (e *Executor) CloseSession(sessionID string) {
	if e.dataTransaction(sessionID) != nil {
		// The Data API has no connection to close: end the transaction.
		_, _ = e.endDataTransaction(sessionID, false)
	}
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
	if e.SessionStatus(sessionID).State == TxOpen && !discardTransaction {
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
	var dataTabs []string
	for id := range e.dataTx {
		dataTabs = append(dataTabs, id)
	}
	e.mu.Unlock()
	for _, id := range dataTabs {
		_, _ = e.endDataTransaction(id, false)
	}
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
			base, err := dataAPIRequest(resolved, options)
			if err != nil {
				return model.SQLQueryResult{}, err
			}
			if outcome, err = e.runDataAPI(ctx, "", base, statements, query, options); err != nil {
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
		IdleInTransactionSecs, StatementTimeoutSecs      int
		Tunnel                                           *config.SSMTunnel
	}{connection.Host, connection.Database, connection.User, connection.SSLMode, connection.SSLRootCert, connection.AuthMode, connection.Port,
		connection.Profile.IdleInTransactionSecs, connection.Profile.StatementTimeoutSecs, connection.Profile.SSMTunnel})
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
