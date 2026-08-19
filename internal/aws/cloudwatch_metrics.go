package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/dostrow/e9s/internal/model"
)

const defaultMetricMaxPoints = 300

// GetMetricSeries retrieves complete, chronologically ordered CloudWatch time
// series. It follows pagination and de-duplicates timestamped samples so the
// result can be rendered consistently by both frontends.
func (c *Client) GetMetricSeries(ctx context.Context, request model.MetricRequest) (*model.MetricSnapshot, error) {
	if c.CW == nil {
		return nil, fmt.Errorf("CloudWatch client is unavailable")
	}
	if request.EndTime.IsZero() {
		request.EndTime = time.Now()
	}
	if request.StartTime.IsZero() || !request.StartTime.Before(request.EndTime) {
		return nil, fmt.Errorf("metric start time must be before end time")
	}
	if len(request.Queries) == 0 {
		return nil, fmt.Errorf("at least one metric query is required")
	}
	if request.MaxPoints <= 0 {
		request.MaxPoints = defaultMetricMaxPoints
	}
	period := request.Period
	if period <= 0 {
		period = metricPeriod(request.EndTime.Sub(request.StartTime), request.MaxPoints)
	}
	periodSeconds := int32(max(60, int(period/time.Second)))
	period = time.Duration(periodSeconds) * time.Second

	queries, err := cloudWatchMetricQueries(request.Queries, periodSeconds)
	if err != nil {
		return nil, err
	}
	values := make(map[string]map[time.Time]float64, len(request.Queries))
	for _, query := range request.Queries {
		values[query.ID] = make(map[time.Time]float64)
	}

	var nextToken *string
	for {
		out, err := c.CW.GetMetricData(ctx, &cloudwatch.GetMetricDataInput{
			StartTime:         &request.StartTime,
			EndTime:           &request.EndTime,
			MetricDataQueries: queries,
			NextToken:         nextToken,
		})
		if err != nil {
			return nil, err
		}
		mergeMetricResults(values, request.Queries, out.MetricDataResults)
		nextToken = out.NextToken
		if nextToken == nil || strings.TrimSpace(*nextToken) == "" {
			break
		}
	}

	snapshot := &model.MetricSnapshot{
		StartTime: request.StartTime,
		EndTime:   request.EndTime,
		Period:    period,
		Series:    make([]model.MetricSeries, 0, len(request.Queries)),
	}
	for _, query := range request.Queries {
		series := model.MetricSeries{
			ID:        query.ID,
			Label:     query.Label,
			Unit:      query.Unit,
			Statistic: query.Statistic,
		}
		for timestamp, value := range values[query.ID] {
			series.Points = append(series.Points, model.MetricPoint{Timestamp: timestamp, Value: value})
		}
		sort.Slice(series.Points, func(i, j int) bool {
			return series.Points[i].Timestamp.Before(series.Points[j].Timestamp)
		})
		snapshot.Series = append(snapshot.Series, series)
	}
	return snapshot, nil
}

func metricPeriod(window time.Duration, maxPoints int) time.Duration {
	if maxPoints <= 0 {
		maxPoints = defaultMetricMaxPoints
	}
	seconds := int64(window / time.Second)
	if seconds < 60 {
		return time.Minute
	}
	periods := (seconds + int64(maxPoints) - 1) / int64(maxPoints)
	minimum := int64(60)
	if window > 15*24*time.Hour {
		minimum = 300
	}
	if window > 63*24*time.Hour {
		minimum = 3600
	}
	periods = ((periods + minimum - 1) / minimum) * minimum
	if periods < minimum {
		periods = minimum
	}
	return time.Duration(periods) * time.Second
}

func cloudWatchMetricQueries(specs []model.MetricQuery, period int32) ([]cwtypes.MetricDataQuery, error) {
	seen := make(map[string]struct{}, len(specs))
	queries := make([]cwtypes.MetricDataQuery, 0, len(specs))
	for _, spec := range specs {
		id := strings.TrimSpace(spec.ID)
		if id == "" {
			return nil, fmt.Errorf("metric query ID is required")
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("duplicate metric query ID %q", id)
		}
		seen[id] = struct{}{}
		if strings.TrimSpace(spec.Namespace) == "" || strings.TrimSpace(spec.MetricName) == "" {
			return nil, fmt.Errorf("metric query %q requires a namespace and metric name", id)
		}
		statistic := strings.TrimSpace(spec.Statistic)
		if statistic == "" {
			statistic = "Average"
		}
		dimensions := make([]cwtypes.Dimension, 0, len(spec.Dimensions))
		for _, dimension := range spec.Dimensions {
			dimensions = append(dimensions, cwtypes.Dimension{
				Name: awssdk.String(dimension.Name), Value: awssdk.String(dimension.Value),
			})
		}
		queries = append(queries, cwtypes.MetricDataQuery{
			Id:         awssdk.String(id),
			Label:      awssdk.String(spec.Label),
			ReturnData: awssdk.Bool(true),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace: awssdk.String(spec.Namespace), MetricName: awssdk.String(spec.MetricName), Dimensions: dimensions,
				},
				Period: &period,
				Stat:   awssdk.String(statistic),
			},
		})
	}
	return queries, nil
}

func mergeMetricResults(target map[string]map[time.Time]float64, specs []model.MetricQuery, results []cwtypes.MetricDataResult) {
	scaleByID := make(map[string]float64, len(specs))
	for _, spec := range specs {
		scale := spec.Scale
		if scale == 0 {
			scale = 1
		}
		scaleByID[spec.ID] = scale
	}
	for _, result := range results {
		id := awssdk.ToString(result.Id)
		points, ok := target[id]
		if !ok {
			continue
		}
		count := min(len(result.Timestamps), len(result.Values))
		for i := 0; i < count; i++ {
			points[result.Timestamps[i]] = result.Values[i] * scaleByID[id]
		}
	}
}
