package model

import "time"

// LambdaFunction is the UI-neutral Lambda configuration used by both frontends.
type LambdaFunction struct {
	Name         string
	ARN          string
	Runtime      string
	Handler      string
	Description  string
	MemoryMB     int
	TimeoutSec   int
	CodeSize     int64
	State        string
	PackageType  string
	LastModified time.Time
	LogGroup     string
	EnvVars      []EnvVar
	RawEnvVars   map[string]string
}
