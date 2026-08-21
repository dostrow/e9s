package aws

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/dostrow/e9s/internal/model"
)

const costDateLayout = "2006-01-02"

func (c *Client) FetchCostReport(ctx context.Context, query model.CostQuery, resources bool) (model.CostReport, error) {
	if c.CostExplorer == nil {
		return model.CostReport{}, fmt.Errorf("cost explorer client is unavailable")
	}
	metric := strings.TrimSpace(query.Metric)
	if metric == "" {
		metric = "UnblendedCost"
	}
	groupBy := strings.ToUpper(strings.TrimSpace(query.GroupBy))
	if groupBy == "" {
		groupBy = "SERVICE"
	}
	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod:  costInterval(query.Start, query.End),
		Granularity: cetypes.GranularityDaily,
		Metrics:     []string{metric},
		GroupBy: []cetypes.GroupDefinition{{
			Type: cetypes.GroupDefinitionTypeDimension,
			Key:  awssdk.String(groupBy),
		}},
	}
	if query.BillingView != "" {
		input.BillingViewArn = awssdk.String(query.BillingView)
	}
	if query.ServiceFilter != "" {
		input.Filter = costServiceFilter(query.ServiceFilter)
	}

	report := model.CostReport{Query: query, CachedAt: time.Now()}
	groups := make(map[string]*model.CostGroup)
	var token *string
	for {
		input.NextPageToken = token
		var results []cetypes.ResultByTime
		var next *string
		if resources {
			serviceName := query.ServiceFilter
			if serviceName == "" {
				serviceName = "Amazon Elastic Compute Cloud - Compute"
			}
			resourceInput := &costexplorer.GetCostAndUsageWithResourcesInput{
				TimePeriod: input.TimePeriod, Granularity: input.Granularity, Metrics: input.Metrics,
				GroupBy: []cetypes.GroupDefinition{{Type: cetypes.GroupDefinitionTypeDimension, Key: awssdk.String("RESOURCE_ID")}},
				Filter:  costServiceFilter(serviceName), NextPageToken: token, BillingViewArn: input.BillingViewArn,
			}
			page, err := c.CostExplorer.GetCostAndUsageWithResources(ctx, resourceInput)
			if err != nil {
				return model.CostReport{}, err
			}
			results, next = page.ResultsByTime, page.NextPageToken
			report.ResourceDataOptIn = true
		} else {
			page, err := c.CostExplorer.GetCostAndUsage(ctx, input)
			if err != nil {
				return model.CostReport{}, err
			}
			results, next = page.ResultsByTime, page.NextPageToken
		}
		report.Pages++
		accumulateCostResults(&report, groups, results, metric)
		if next == nil || *next == "" {
			break
		}
		token = next
	}
	for _, group := range groups {
		report.Groups = append(report.Groups, *group)
	}
	sort.Slice(report.Groups, func(i, j int) bool { return report.Groups[i].Amount > report.Groups[j].Amount })
	return report, nil
}

func (c *Client) FetchCostForecast(ctx context.Context, query model.CostQuery) (total, lower, upper float64, unit string, err error) {
	now := time.Now().UTC()
	end := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	input := &costexplorer.GetCostForecastInput{
		TimePeriod: costInterval(now, end), Granularity: cetypes.GranularityDaily,
		Metric: cetypes.MetricUnblendedCost, PredictionIntervalLevel: awssdk.Int32(80),
	}
	if query.ServiceFilter != "" {
		input.Filter = costServiceFilter(query.ServiceFilter)
	}
	if query.BillingView != "" {
		input.BillingViewArn = awssdk.String(query.BillingView)
	}
	out, err := c.CostExplorer.GetCostForecast(ctx, input)
	if err != nil {
		return 0, 0, 0, "", err
	}
	if out.Total != nil {
		total = parseCostAmount(out.Total.Amount)
		unit = awssdk.ToString(out.Total.Unit)
	}
	for _, point := range out.ForecastResultsByTime {
		lower += parseCostAmount(point.PredictionIntervalLowerBound)
		upper += parseCostAmount(point.PredictionIntervalUpperBound)
	}
	return total, lower, upper, unit, nil
}

func (c *Client) FetchCostAnomalies(ctx context.Context, start, end time.Time) (model.CostAnomalyReport, error) {
	input := &costexplorer.GetAnomaliesInput{
		DateInterval: &cetypes.AnomalyDateInterval{StartDate: awssdk.String(costDate(start)), EndDate: awssdk.String(costDate(end))},
		MaxResults:   awssdk.Int32(100),
	}
	report := model.CostAnomalyReport{CachedAt: time.Now(), DataThrough: end}
	for {
		out, err := c.CostExplorer.GetAnomalies(ctx, input)
		if err != nil {
			return model.CostAnomalyReport{}, err
		}
		report.Pages++
		for _, item := range out.Anomalies {
			anomaly := model.CostAnomaly{ID: awssdk.ToString(item.AnomalyId), Service: awssdk.ToString(item.DimensionValue), Feedback: string(item.Feedback)}
			anomaly.Start, _ = time.Parse(costDateLayout, awssdk.ToString(item.AnomalyStartDate))
			anomaly.End, _ = time.Parse(costDateLayout, awssdk.ToString(item.AnomalyEndDate))
			if item.Impact != nil {
				anomaly.Impact, anomaly.MaximumImpact = item.Impact.TotalImpact, item.Impact.MaxImpact
				anomaly.ActualSpend = awssdk.ToFloat64(item.Impact.TotalActualSpend)
				anomaly.ExpectedSpend = awssdk.ToFloat64(item.Impact.TotalExpectedSpend)
				anomaly.ImpactPercent = awssdk.ToFloat64(item.Impact.TotalImpactPercentage)
			}
			if len(item.RootCauses) > 0 {
				root := item.RootCauses[0]
				if value := awssdk.ToString(root.Service); value != "" {
					anomaly.Service = value
				}
				anomaly.Region, anomaly.Account, anomaly.UsageType = awssdk.ToString(root.Region), awssdk.ToString(root.LinkedAccountName), awssdk.ToString(root.UsageType)
			}
			report.Anomalies = append(report.Anomalies, anomaly)
		}
		if out.NextPageToken == nil || *out.NextPageToken == "" {
			break
		}
		input.NextPageToken = out.NextPageToken
	}
	sort.Slice(report.Anomalies, func(i, j int) bool { return report.Anomalies[i].Impact > report.Anomalies[j].Impact })
	return report, nil
}

func costInterval(start, end time.Time) *cetypes.DateInterval {
	return &cetypes.DateInterval{Start: awssdk.String(costDate(start)), End: awssdk.String(costDate(end))}
}

func costDate(value time.Time) string { return value.UTC().Format(costDateLayout) }

func costServiceFilter(service string) *cetypes.Expression {
	return &cetypes.Expression{Dimensions: &cetypes.DimensionValues{Key: cetypes.DimensionService, Values: []string{service}}}
}

func accumulateCostResults(report *model.CostReport, groups map[string]*model.CostGroup, results []cetypes.ResultByTime, metric string) {
	for _, result := range results {
		start, _ := time.Parse(costDateLayout, awssdk.ToString(result.TimePeriod.Start))
		end, _ := time.Parse(costDateLayout, awssdk.ToString(result.TimePeriod.End))
		through := end.Add(-time.Nanosecond)
		if through.After(report.DataThrough) {
			report.DataThrough = through
		}
		for _, value := range result.Groups {
			name := strings.Join(value.Keys, " / ")
			metricValue := value.Metrics[metric]
			amount := parseCostAmount(metricValue.Amount)
			unit := awssdk.ToString(metricValue.Unit)
			group := groups[name]
			if group == nil {
				group = &model.CostGroup{Name: name, Unit: unit}
				groups[name] = group
			}
			group.Amount += amount
			group.Points = append(group.Points, model.CostPoint{Start: start, End: end, Amount: amount, Unit: unit, Estimated: result.Estimated})
			report.Total += amount
			if report.Unit == "" {
				report.Unit = unit
			}
		}
	}
}

func parseCostAmount(value *string) float64 {
	amount, _ := strconv.ParseFloat(awssdk.ToString(value), 64)
	return amount
}
