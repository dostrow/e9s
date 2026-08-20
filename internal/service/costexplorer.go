package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

const CostCacheTTL = 24 * time.Hour

type CostExplorerAPI interface {
	FetchCostReport(context.Context, model.CostQuery, bool) (model.CostReport, error)
	FetchCostForecast(context.Context, model.CostQuery) (float64, float64, float64, string, error)
	FetchCostAnomalies(context.Context, time.Time, time.Time) (model.CostAnomalyReport, error)
}

type cachedCostReport struct {
	report  model.CostReport
	expires time.Time
}

type cachedAnomalies struct {
	report  model.CostAnomalyReport
	expires time.Time
}

type CostExplorer struct {
	api       CostExplorerAPI
	mu        sync.Mutex
	reports   map[string]cachedCostReport
	anomalies map[string]cachedAnomalies
	now       func() time.Time
}

func NewCostExplorer(api CostExplorerAPI) *CostExplorer {
	return &CostExplorer{api: api, reports: make(map[string]cachedCostReport), anomalies: make(map[string]cachedAnomalies), now: time.Now}
}

func NormalizeCostQuery(query model.CostQuery) (model.CostQuery, error) {
	now := time.Now().UTC()
	if query.End.IsZero() {
		query.End = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	}
	if query.Start.IsZero() {
		query.Start = query.End.AddDate(0, 0, -30)
	}
	query.Start = query.Start.UTC()
	query.End = query.End.UTC()
	if !query.Start.Before(query.End) {
		return model.CostQuery{}, fmt.Errorf("cost query start must be before end")
	}
	if strings.TrimSpace(query.Metric) == "" {
		query.Metric = "UnblendedCost"
	}
	if strings.TrimSpace(query.GroupBy) == "" {
		query.GroupBy = "SERVICE"
	}
	return query, nil
}

func (s *CostExplorer) Report(ctx context.Context, query model.CostQuery, resources, forecast, force bool) (model.CostReport, model.CostCacheStatus, error) {
	query, err := NormalizeCostQuery(query)
	if err != nil {
		return model.CostReport{}, model.CostCacheStatus{}, err
	}
	key := costReportCacheKey(query, resources, forecast)
	now := s.now()
	if !force {
		s.mu.Lock()
		cached, ok := s.reports[key]
		s.mu.Unlock()
		if ok && now.Before(cached.expires) {
			return cached.report, costStatus(cached.report.CachedAt, cached.report.DataThrough, cached.report.Pages, true), nil
		}
	}
	report, err := s.api.FetchCostReport(ctx, query, resources)
	if err != nil {
		if resources {
			return model.CostReport{}, model.CostCacheStatus{}, fmt.Errorf("load resource-level Cost Explorer data (requires resource-level data opt-in and supported filters): %w", err)
		}
		return model.CostReport{}, model.CostCacheStatus{}, fmt.Errorf("load Cost Explorer data: %w", err)
	}
	if forecast {
		report.Forecast, report.ForecastLower, report.ForecastUpper, report.ForecastUnit, err = s.api.FetchCostForecast(ctx, query)
		if err != nil {
			return model.CostReport{}, model.CostCacheStatus{}, fmt.Errorf("load Cost Explorer forecast: %w", err)
		}
		report.Pages++
	}
	report.CachedAt = now
	s.mu.Lock()
	s.reports[key] = cachedCostReport{report: report, expires: now.Add(CostCacheTTL)}
	s.mu.Unlock()
	return report, costStatus(report.CachedAt, report.DataThrough, report.Pages, false), nil
}

func (s *CostExplorer) Anomalies(ctx context.Context, start, end time.Time, force bool) (model.CostAnomalyReport, model.CostCacheStatus, error) {
	now := s.now()
	if end.IsZero() {
		end = now.UTC()
	}
	if start.IsZero() {
		start = end.AddDate(0, 0, -90)
	}
	key := start.UTC().Format("2006-01-02") + "|" + end.UTC().Format("2006-01-02")
	if !force {
		s.mu.Lock()
		cached, ok := s.anomalies[key]
		s.mu.Unlock()
		if ok && now.Before(cached.expires) {
			return cached.report, costStatus(cached.report.CachedAt, cached.report.DataThrough, cached.report.Pages, true), nil
		}
	}
	report, err := s.api.FetchCostAnomalies(ctx, start, end)
	if err != nil {
		return model.CostAnomalyReport{}, model.CostCacheStatus{}, fmt.Errorf("load Cost Explorer anomalies: %w", err)
	}
	report.CachedAt = now
	s.mu.Lock()
	s.anomalies[key] = cachedAnomalies{report: report, expires: now.Add(CostCacheTTL)}
	s.mu.Unlock()
	return report, costStatus(report.CachedAt, report.DataThrough, report.Pages, false), nil
}

func costReportCacheKey(query model.CostQuery, resources, forecast bool) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%t|%t", query.Start.Format("2006-01-02"), query.End.Format("2006-01-02"), query.Metric,
		strings.ToUpper(query.GroupBy), query.ServiceFilter, query.BillingView, resources, forecast)
}

func costStatus(cachedAt, dataThrough time.Time, pages int, fromCache bool) model.CostCacheStatus {
	return model.CostCacheStatus{CachedAt: cachedAt, DataThrough: dataThrough, Pages: pages, FromCache: fromCache}
}
