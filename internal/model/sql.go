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
