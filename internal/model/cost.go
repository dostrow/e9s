package model

import "time"

type CostQuery struct {
	Start         time.Time
	End           time.Time
	Metric        string
	GroupBy       string
	ServiceFilter string
	BillingView   string
}

type CostPoint struct {
	Start     time.Time
	End       time.Time
	Amount    float64
	Unit      string
	Estimated bool
}

type CostGroup struct {
	Name   string
	Amount float64
	Unit   string
	Points []CostPoint
}

type CostReport struct {
	Query              CostQuery
	Groups             []CostGroup
	Total              float64
	Unit               string
	Forecast           float64
	ForecastLower      float64
	ForecastUpper      float64
	ForecastUnit       string
	CachedAt           time.Time
	DataThrough        time.Time
	Pages              int
	ResourceDataOptIn  bool
	ResourceDataNotice string
}

type CostAnomaly struct {
	ID            string
	Start         time.Time
	End           time.Time
	Service       string
	Region        string
	Account       string
	UsageType     string
	ActualSpend   float64
	ExpectedSpend float64
	Impact        float64
	ImpactPercent float64
	MaximumImpact float64
	Feedback      string
}

type CostAnomalyReport struct {
	Anomalies   []CostAnomaly
	CachedAt    time.Time
	DataThrough time.Time
	Pages       int
}

type CostCacheStatus struct {
	CachedAt    time.Time
	DataThrough time.Time
	Pages       int
	FromCache   bool
}
