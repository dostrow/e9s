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

	// readOnlyTransaction records that e9s made the open transaction
	// read-only because its tab was locked; it cannot be made writable again.
	readOnlyTransaction atomic.Bool

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
	if !s.catalog {
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
	s.settle(&outcome, TxIdle)
	if mode == RunInTransaction && outcome.Transaction == TxOpen {
		outcome.Events = append(outcome.Events, "The transaction is still open. Review the results, then commit or roll it back.")
	}
	return outcome
}

// applyWriteMode makes the server enforce the tab's write lock as well, so
// a function called from a SELECT cannot change data in a locked tab. It
// runs before every run rather than trusting an earlier setting, which SQL
// can change. An open transaction in a locked tab is made read-only; it is
// left alone when the tab allows writes, because a transaction cannot be
// made writable once it has run a query.
func (s *directSession) applyWriteMode(ctx context.Context, allowWrites bool) error {
	if s.txState() == TxOpen {
		if allowWrites {
			return nil
		}
		if _, err := s.connection.Exec(ctx, "SET LOCAL transaction_read_only = on"); err != nil {
			return err
		}
		s.readOnlyTransaction.Store(true)
		return nil
	}
	statement := "SET SESSION default_transaction_read_only = on"
	if allowWrites {
		statement = "SET SESSION default_transaction_read_only = off"
	}
	_, err := s.connection.Exec(ctx, statement)
	return err
}

func (s *directSession) query(ctx context.Context, sql string, maxRows int) (model.SQLQueryResult, error) {
	started := time.Now()
	// Text format returns each value as PostgreSQL itself renders it, as
	// psql shows it: infinity, intervals, ranges, arrays and numerics come
	// out exactly, which a verification report depends on.
	rows, err := s.connection.Query(ctx, sql, pgx.QueryResultFormats{pgx.TextFormatCode})
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
		values := rows.RawValues()
		formatted := make([]string, len(values))
		for index, value := range values {
			if value == nil {
				formatted[index] = "NULL"
			} else {
				formatted[index] = string(value)
			}
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
// records anything e9s did on the session's behalf. before is the state the
// session was in when the run started.
func (s *directSession) settle(outcome *RunOutcome, before TxState) {
	status := s.connection.PgConn().TxStatus()
	if s.connection.IsClosed() {
		s.closed.Store(true)
		outcome.ConnectionLost = true
		switch {
		case outcome.Failure != nil && isCommit(outcome.Failure.SQL):
			outcome.Events = append(outcome.Events, "The connection was lost while COMMIT was running. PostgreSQL may or may not have committed the transaction: check the data before running anything again.")
		case status != 'I':
			outcome.Events = append(outcome.Events, "The connection was lost while a transaction was open. PostgreSQL rolled it back: none of its changes were kept.")
		default:
			outcome.Events = append(outcome.Events, "The connection was lost.")
		}
		s.endedTransaction()
		outcome.Transaction = TxIdle
		return
	}
	switch {
	case status == 'E':
		s.rollbackOnBehalf(outcome, "The transaction failed, so e9s rolled it back: none of its changes were kept and its locks were released.")
	case status == 'T' && outcome.Failure != nil && before == TxIdle:
		// The run opened this transaction and stopped before its last
		// statement, for example because it was cancelled between two
		// statements. Committing it would apply half the script.
		s.rollbackOnBehalf(outcome, "The run stopped before it finished, so e9s rolled back the transaction it had opened: none of its changes were kept.")
	case status == 'T':
		s.setState(TxOpen)
		if outcome.Failure != nil {
			outcome.Events = append(outcome.Events, "The run stopped before it finished; the statements after the one that stopped it did not run. The transaction that was open before this run is still open.")
		}
	default:
		s.endedTransaction()
	}
	outcome.Transaction = s.txState()
}

// rollbackOnBehalf rolls back the open transaction and records event, or
// closes the connection if the rollback fails so that PostgreSQL rolls it
// back instead.
func (s *directSession) rollbackOnBehalf(outcome *RunOutcome, event string) {
	if err := s.rollback(); err != nil {
		outcome.Events = append(outcome.Events, "e9s could not roll back the transaction ("+describeError(err)+"), so it closed the connection and PostgreSQL rolled it back.")
		s.discardConnection()
		outcome.ConnectionLost = true
	} else {
		outcome.Events = append(outcome.Events, event)
	}
	s.endedTransaction()
}

// endedTransaction records that no transaction is open any more.
func (s *directSession) endedTransaction() {
	s.setState(TxIdle)
	s.readOnlyTransaction.Store(false)
}

func isCommit(sql string) bool {
	words := SQLWords(sql)
	return len(words) > 0 && (words[0] == "commit" || words[0] == "end")
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
		s.endedTransaction()
		if commit {
			return "", fmt.Errorf("the connection was lost while committing; PostgreSQL may or may not have committed the transaction, so check the data before running anything again")
		}
		return "", fmt.Errorf("the connection was lost before the transaction ended, so PostgreSQL rolled it back: none of its changes were kept")
	}
	if err != nil {
		if s.connection.PgConn().TxStatus() == 'E' {
			_ = s.rollback()
		}
		if s.connection.PgConn().TxStatus() == 'T' {
			s.setState(TxOpen)
		} else {
			s.endedTransaction()
		}
		return "", fmt.Errorf("%s failed: %s", statement, describeError(err))
	}
	s.endedTransaction()
	return tag.String(), nil
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
