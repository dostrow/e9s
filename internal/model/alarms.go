package model

import "time"

const (
	AlarmStateOK               = "OK"
	AlarmStateAlarm            = "ALARM"
	AlarmStateInsufficientData = "INSUFFICIENT_DATA"
)

// Alarm is the UI-neutral summary used by alarm browsers.
type Alarm struct {
	Name           string
	State          string
	StateReason    string
	StateUpdatedAt time.Time
	MetricName     string
	Namespace      string
	ActionsEnabled bool
	ARN            string
}

// AlarmDetail contains the configuration and recent history for an alarm.
type AlarmDetail struct {
	Alarm
	Description         string
	ComparisonOp        string
	Threshold           float64
	EvalPeriods         int
	Period              int
	Statistic           string
	TreatMissing        string
	Dimensions          map[string]string
	AlarmActions        []string
	OKActions           []string
	InsufficientActions []string
	History             []AlarmHistoryItem
}

// AlarmHistoryItem is a recent configuration, action, or state transition.
type AlarmHistoryItem struct {
	Timestamp time.Time
	Type      string
	Summary   string
}
