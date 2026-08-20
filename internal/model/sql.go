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
}

type SQLQueryResult struct {
	Columns      []string
	Rows         [][]string
	CommandTag   string
	RowsAffected int64
	Truncated    bool
	DurationMS   int64
}
