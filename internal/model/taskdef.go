package model

import "time"

type TaskDefRef struct {
	ARN      string
	Family   string
	Revision int
}

type TaskDefSummary struct {
	ARN                     string
	Family                  string
	Revision                int
	Status                  string
	CPU                     string
	Memory                  string
	NetworkMode             string
	TaskRoleArn             string
	ExecutionRoleArn        string
	RequiresCompatibilities []string
	RegisteredAt            time.Time
	Containers              []TaskDefContainer
	RawJSON                 string
}

type TaskDefContainer struct {
	Name       string
	Image      string
	CPU        int
	Memory     int
	Essential  bool
	EnvVars    []EnvVar
	EnvVarKeys []string
}

type EnvVar struct {
	Name          string
	Value         string
	ResolvedValue string
	Source        string
}
