package model

import "time"

// CodeBuildProject is the UI-neutral project summary used by both frontends.
type CodeBuildProject struct {
	Name         string
	Description  string
	Source       string
	LastModified time.Time
}

// CodeBuildBuild is the UI-neutral summary of a project build.
type CodeBuildBuild struct {
	ID            string
	BuildNumber   int64
	Status        string
	StartTime     time.Time
	EndTime       time.Time
	Duration      time.Duration
	Initiator     string
	SourceVersion string
	CurrentPhase  string
}

// CodeBuildDetail contains the complete inspectable state for one build.
type CodeBuildDetail struct {
	CodeBuildBuild
	ProjectName   string
	ARN           string
	Source        CodeBuildSource
	Phases        []CodeBuildPhase
	LogGroupName  string
	LogStreamName string
	Environment   []CodeBuildEnvVar
}

type CodeBuildSource struct {
	Type     string
	Location string
	Version  string
}

type CodeBuildPhase struct {
	Name     string
	Status   string
	Duration time.Duration
	Contexts []string
}

type CodeBuildEnvVar struct {
	Name  string
	Value string
	Type  string
}
