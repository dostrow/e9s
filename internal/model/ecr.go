package model

import "time"

// ECRRepo is the UI-neutral repository summary shared by both frontends.
type ECRRepo struct {
	Name           string
	URI            string
	ARN            string
	ScanOnPush     bool
	TagMutability  string
	EncryptionType string
	CreatedAt      time.Time
}

// ECRImage is the UI-neutral image and scan summary.
type ECRImage struct {
	Digest       string
	Tags         []string
	PushedAt     time.Time
	SizeBytes    int64
	MediaType    string
	ScanStatus   string
	ScanSeverity map[string]int32
}

// ECRFinding is one vulnerability reported for an image.
type ECRFinding struct {
	Name        string
	Severity    string
	Description string
	URI         string
	Package     string
	Version     string
}
