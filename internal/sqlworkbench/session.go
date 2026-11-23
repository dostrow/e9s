package sqlworkbench

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dostrow/e9s/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgconn/ctxwatch"
)

const (
	// DefaultIdleInTransactionTimeout ends a query tab's session on the
	// server when it sits in an open transaction this long.
	DefaultIdleInTransactionTimeout = 5 * time.Minute
	// cleanupTimeout bounds the ROLLBACK e9s sends on a session's behalf.
	cleanupTimeout = 15 * time.Second
	// cancelDeadlineDelay is how long a cancelled statement has to stop
	// after PostgreSQL receives the cancel request before e9s gives up on
	// the connection.
	cancelDeadlineDelay = 10 * time.Second
	// catalogStatementTimeout bounds object browser queries when the
	// profile sets no statement timeout.
	catalogStatementTimeout = 30 * time.Second
)

// TxState is a session's transaction state between runs.
type TxState int32

const (
	// TxIdle means no transaction is open: every statement commits as it
	// completes.
	TxIdle TxState = iota
	// TxOpen means a transaction is open and holds its changes and locks
	// until it is committed or rolled back.
	TxOpen
)

func (s TxState) String() string {
	if s == TxOpen {
		return "transaction open"
	}
	return "idle"
}

// SessionStatus describes a query tab's session for display.
type SessionStatus struct {
	Connected   bool
	State       TxState
	Since       time.Time     // when the open transaction began
	IdleTimeout time.Duration // server-side idle-in-transaction limit
}

// directSession is one PostgreSQL connection. Query tabs each own one, so a
// transaction is scoped to the tab that opened it; the object browser uses a
// separate read-only session per profile.
type directSession struct {
	profileName string
	profileKey  string
	catalog     bool
	connection  *pgx.Conn
	tunnel      *Tunnel
	idleTimeout time.Duration
	mu          sync.Mutex

	// readOnlyDefault mirrors the session's default_transaction_read_only
	// setting: 0 unknown, 1 on, -1 off. Guarded by mu.
	readOnlyDefault int8

	// state and since are read without mu so the UI can show them while a
	// run holds the session.
	state  atomic.Int32
	since  atomic.Int64
	closed atomic.Bool

	// notices collects the server's NOTICE and WARNING messages for the
	// statement being run.
	noticeMu sync.Mutex
	notices  []string
}

func (s *directSession) addNotice(notice *pgconn.Notice) {
	text := notice.Severity + ": " + notice.Message
	if notice.Detail != "" {
		text += "\nDETAIL: " + notice.Detail
	}
	if notice.Hint != "" {
		text += "\nHINT: " + notice.Hint
	}
	s.noticeMu.Lock()
	s.notices = append(s.notices, text)
	s.noticeMu.Unlock()
}

// takeNotices returns and clears the notices collected so far.
func (s *directSession) takeNotices() []string {
	s.noticeMu.Lock()
	defer s.noticeMu.Unlock()
	notices := s.notices
	s.notices = nil
	return notices
}

func (s *directSession) txState() TxState { return TxState(s.state.Load()) }

func (s *directSession) status() SessionStatus {
	status := SessionStatus{Connected: !s.closed.Load(), State: s.txState(), IdleTimeout: s.idleTimeout}
	if status.State == TxOpen {
		status.Since = time.Unix(0, s.since.Load())
	}
	return status
}

func (s *directSession) setState(state TxState) {
	if state == TxOpen && s.txState() != TxOpen {
		s.since.Store(time.Now().UnixNano())
	}
	s.state.Store(int32(state))
}

func connectSession(ctx context.Context, connection ResolvedConnection, key string, catalog bool, options ExecutorOptions) (*directSession, error) {
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
	connectionURL.RawQuery = query.Encode()
	pgxConfig, err := pgx.ParseConfig(connectionURL.String())
	if err != nil {
		return nil, err
	}
	idleTimeout := DefaultIdleInTransactionTimeout
	if seconds := connection.Profile.IdleInTransactionSecs; seconds > 0 {
		idleTimeout = time.Duration(seconds) * time.Second
	}
	pgxConfig.RuntimeParams["application_name"] = "e9s-sql-workbench"
	pgxConfig.RuntimeParams["idle_in_transaction_session_timeout"] = strconv.FormatInt(idleTimeout.Milliseconds(), 10)
	if seconds := connection.Profile.StatementTimeoutSecs; seconds > 0 {
		pgxConfig.RuntimeParams["statement_timeout"] = strconv.Itoa(seconds * 1000)
	}
	if catalog {
		// The object browser only reads; the server enforces that too.
		pgxConfig.RuntimeParams["application_name"] = "e9s-sql-catalog"
		pgxConfig.RuntimeParams["default_transaction_read_only"] = "on"
		if connection.Profile.StatementTimeoutSecs <= 0 {
			// Metadata queries are small; never let one hang the browser.
			pgxConfig.RuntimeParams["statement_timeout"] = strconv.Itoa(int(catalogStatementTimeout.Milliseconds()))
		}
	}
	// Unnamed statements: a cached plan for a statement over a temporary
	// table would fail once a later run recreated that table differently.
	pgxConfig.DefaultQueryExecMode = pgx.QueryExecModeDescribeExec
	// Cancelling a run asks PostgreSQL to stop the statement and keeps the
	// connection, and so any open transaction, intact. pgx's default would
	// drop the connection without telling the server.
	pgxConfig.BuildContextWatcherHandler = func(conn *pgconn.PgConn) ctxwatch.Handler {
		return &pgconn.CancelRequestContextWatcherHandler{Conn: conn, DeadlineDelay: cancelDeadlineDelay}
	}
	session := &directSession{profileName: connection.Profile.Name, profileKey: key, catalog: catalog, idleTimeout: idleTimeout}
	pgxConfig.OnNotice = func(_ *pgconn.PgConn, notice *pgconn.Notice) { session.addNotice(notice) }
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
	session.connection, session.tunnel = client, tunnel
	return session, nil
}

// alive reports whether the connection is usable, without waiting for a run
// that holds the session.
func (s *directSession) alive() bool {
	if s.closed.Load() {
		return false
	}
	if !s.mu.TryLock() {
		return true
	}
	defer s.mu.Unlock()
	if s.connection.IsClosed() {
		s.closed.Store(true)
	}
	return !s.closed.Load()
}

// run executes statements in order and stops at the first failure. It then
// settles the session: a failed transaction is rolled back, and a lost
// connection is reported. The caller holds no lock.
func (s *directSession) run(ctx context.Context, statements []Statement, source string, allowWrites bool, maxRows int) RunOutcome {
	s.mu.Lock()
	defer s.mu.Unlock()
	outcome := RunOutcome{Statements: len(statements)}
	before := s.txState()
	if !s.catalog && before == TxIdle {
		if err := s.applyWriteMode(ctx, allowWrites); err != nil {
			outcome.Failure = &StatementFailure{Index: -1, Message: "prepare the session: " + describeError(err)}
			s.settle(&outcome, before)
			return outcome
		}
	}
	s.runStatements(ctx, statements, source, maxRows, &outcome)
	s.settle(&outcome, before)
	return outcome
}

func (s *directSession) runStatements(ctx context.Context, statements []Statement, source string, maxRows int, outcome *RunOutcome) {
	s.takeNotices() // from e9s's own statements, such as its BEGIN
	for index, statement := range statements {
		reportProgress(ctx, index, len(statements), statement, source)
		result, err := s.query(ctx, statement.SQL, maxRows)
		if err != nil {
			outcome.Failure = newStatementFailure(index, statement, source, err)
			outcome.Failure.Notices = s.takeNotices()
			return
		}
		result.SQL = statement.SQL
		result.Line = StatementLine(source, statement.Start)
		result.Heading = statement.Heading
		result.Notices = s.takeNotices()
		outcome.Results = append(outcome.Results, result)
	}
}

// runInTransaction runs plan inside a transaction that e9s opens. A dry run
// always rolls it back; otherwise a transaction that succeeded stays open
// for the caller to commit or roll back. The session must be idle.
func (s *directSession) runInTransaction(ctx context.Context, plan transactionPlan, source string, allowWrites bool, maxRows int, mode RunMode) RunOutcome {
	s.mu.Lock()
	defer s.mu.Unlock()
	outcome := RunOutcome{Statements: len(plan.body), Mode: mode}
	if event := plan.event(mode); event != "" {
		outcome.Events = append(outcome.Events, event)
	}
	if err := s.applyWriteMode(ctx, allowWrites); err != nil {
		outcome.Failure = &StatementFailure{Index: -1, Message: "prepare the session: " + describeError(err)}
		s.settle(&outcome, TxIdle)
		return outcome
	}
	if _, err := s.connection.Exec(ctx, plan.begin); err != nil {
		outcome.Failure = &StatementFailure{Index: -1, SQL: plan.begin, Message: "start the transaction: " + describeError(err)}
		s.settle(&outcome, TxIdle)
		return outcome
	}
	s.runStatements(ctx, plan.body, source, maxRows, &outcome)
	if mode == RunDryRun && !s.connection.IsClosed() && s.connection.PgConn().TxStatus() == 'T' {
		if err := s.rollback(); err != nil {
			outcome.Events = append(outcome.Events, "The dry run could not be rolled back ("+describeError(err)+"). e9s closed the connection, so PostgreSQL rolled it back.")
			s.discardConnection()
		} else {
			outcome.Events = append(outcome.Events, "Dry run: e9s rolled the transaction back, so none of its changes were kept.")
		}
	}
	s.settle(&outcome, TxOpen)
	if mode == RunInTransaction && outcome.Transaction == TxOpen {
		outcome.Events = append(outcome.Events, "The transaction is still open. Review the results, then commit or roll it back.")
	}
	return outcome
}

// applyWriteMode makes the server enforce the tab's write lock as well, so a
// function called from a SELECT cannot change data in a locked tab.
func (s *directSession) applyWriteMode(ctx context.Context, allowWrites bool) error {
	want := int8(1)
	statement := "SET SESSION default_transaction_read_only = on"
	if allowWrites {
		want, statement = -1, "SET SESSION default_transaction_read_only = off"
	}
	if s.readOnlyDefault == want {
		return nil
	}
	if _, err := s.connection.Exec(ctx, statement); err != nil {
		s.readOnlyDefault = 0
		return err
	}
	s.readOnlyDefault = want
	return nil
}

func (s *directSession) query(ctx context.Context, sql string, maxRows int) (model.SQLQueryResult, error) {
	started := time.Now()
	rows, err := s.connection.Query(ctx, sql)
	if err != nil {
		return model.SQLQueryResult{}, err
	}
	defer rows.Close()
	result := model.SQLQueryResult{}
	for _, field := range rows.FieldDescriptions() {
		result.Columns = append(result.Columns, field.Name)
	}
	for rows.Next() {
		if len(result.Rows) >= maxRows {
			// Keep reading so the statement completes and the session stays
			// usable; only the display is capped.
			result.Truncated = true
			continue
		}
		values, err := rows.Values()
		if err != nil {
			return model.SQLQueryResult{}, err
		}
		formatted := make([]string, len(values))
		for index, value := range values {
			formatted[index] = SQLValueString(value)
		}
		result.Rows = append(result.Rows, formatted)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return model.SQLQueryResult{}, err
	}
	result.CommandTag = rows.CommandTag().String()
	result.RowsAffected = rows.CommandTag().RowsAffected()
	result.DurationMS = time.Since(started).Milliseconds()
	return result, nil
}

// settle leaves the session idle or with a usable open transaction, and
// records anything e9s did on the session's behalf.
func (s *directSession) settle(outcome *RunOutcome, before TxState) {
	if s.connection.IsClosed() {
		s.closed.Store(true)
		outcome.ConnectionLost = true
		if before == TxOpen || s.connection.PgConn().TxStatus() != 'I' {
			outcome.Events = append(outcome.Events, "The connection was lost while a transaction was open. PostgreSQL rolled it back: none of its changes were kept.")
		} else {
			outcome.Events = append(outcome.Events, "The connection was lost.")
		}
		s.setState(TxIdle)
		outcome.Transaction = TxIdle
		return
	}
	switch s.connection.PgConn().TxStatus() {
	case 'E':
		if err := s.rollback(); err != nil {
			outcome.Events = append(outcome.Events, "The transaction failed and could not be rolled back ("+describeError(err)+"). e9s closed the connection, so PostgreSQL rolled it back.")
			s.discardConnection()
			outcome.ConnectionLost = true
		} else {
			outcome.Events = append(outcome.Events, "The transaction failed, so e9s rolled it back: none of its changes were kept and its locks were released.")
		}
		s.setState(TxIdle)
	case 'T':
		s.setState(TxOpen)
	default:
		s.setState(TxIdle)
	}
	outcome.Transaction = s.txState()
}

// rollback ends the open transaction with a context of its own: the run's
// context may be the one that was cancelled.
func (s *directSession) rollback() error {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	_, err := s.connection.Exec(ctx, "ROLLBACK")
	return err
}

// endTransaction commits or rolls back the open transaction.
func (s *directSession) endTransaction(commit bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.txState() != TxOpen {
		return "", ErrNoTransaction
	}
	statement := "ROLLBACK"
	if commit {
		statement = "COMMIT"
	}
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	tag, err := s.connection.Exec(ctx, statement)
	if s.connection.IsClosed() {
		s.closed.Store(true)
		s.setState(TxIdle)
		return "", fmt.Errorf("the connection was lost before the transaction ended, so PostgreSQL rolled it back: none of its changes were kept")
	}
	if err != nil {
		if s.connection.PgConn().TxStatus() == 'E' {
			_ = s.rollback()
		}
		s.setState(boolToTx(s.connection.PgConn().TxStatus() == 'T'))
		return "", fmt.Errorf("%s failed: %s", statement, describeError(err))
	}
	s.setState(TxIdle)
	return tag.String(), nil
}

func boolToTx(open bool) TxState {
	if open {
		return TxOpen
	}
	return TxIdle
}

func (s *directSession) discardConnection() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = s.connection.Close(ctx)
	s.closed.Store(true)
}

func (s *directSession) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.connection != nil {
		_ = s.connection.Close(context.Background())
	}
	s.closed.Store(true)
	s.setState(TxIdle)
	if s.tunnel != nil {
		s.tunnel.Close()
	}
}

func newStatementFailure(index int, statement Statement, source string, err error) *StatementFailure {
	failure := &StatementFailure{Index: index, SQL: statement.SQL, Heading: statement.Heading, Message: describeError(err),
		Line: StatementLine(source, statement.Start)}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		failure.Code = pgErr.Code
		if pgErr.Position > 0 {
			// Position counts characters from 1 within the statement.
			runes := []rune(statement.SQL)
			position := min(int(pgErr.Position)-1, len(runes))
			failure.Line += strings.Count(string(runes[:position]), "\n")
		}
	}
	return failure
}

// describeError renders a PostgreSQL error with its detail and hint.
func describeError(err error) string {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err.Error()
	}
	parts := []string{pgErr.Error()}
	if pgErr.Detail != "" {
		parts = append(parts, "DETAIL: "+pgErr.Detail)
	}
	if pgErr.Hint != "" {
		parts = append(parts, "HINT: "+pgErr.Hint)
	}
	if pgErr.Where != "" {
		parts = append(parts, "CONTEXT: "+pgErr.Where)
	}
	return strings.Join(parts, "\n")
}
