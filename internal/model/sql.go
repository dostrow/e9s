package model

// SQLCredentials is intentionally transient. It must never be serialized into
// configuration or restored workbench state.
type SQLCredentials struct {
	Username string
	Password string
	Database string
	Host     string
	Port     int
}

type SQLQueryRequest struct {
	ResourceARN string
	SecretARN   string
	Database    string
	SQL         string
	MaxRows     int
	// TransactionID runs the statement inside a Data API transaction.
	TransactionID string
}

type SQLQueryResult struct {
	// SQL is the statement that produced this result, as it was sent.
	SQL string
	// Line is the line of the submitted source the statement starts on.
	Line         int
	Columns      []string
	Rows         [][]string
	CommandTag   string
	RowsAffected int64
	Truncated    bool
	DurationMS   int64
}
