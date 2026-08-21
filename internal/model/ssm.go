package model

import "time"

// Parameter is the UI-neutral representation of an SSM Parameter Store value.
// SecureString values returned by list operations remain encrypted/redacted;
// callers must explicitly request Detail when decrypted content is required.
type Parameter struct {
	Name         string
	Value        string
	Type         string // String, StringList, SecureString
	Version      int64
	LastModified time.Time
}
