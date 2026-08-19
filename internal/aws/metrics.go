package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/dostrow/e9s/internal/model"
)

type ServiceMetrics = model.ServiceMetrics
type AlarmState = model.AlarmState

// GetServiceMetrics fetches CPU and memory utilization for an ECS service.
func (c *Client) GetServiceMetrics(ctx context.Context, clusterName, serviceName string, period time.Duration) (*ServiceMetrics, error) {
	dims := []cwtypes.Dimension{
		{Name: aws.String("ClusterName"), Value: aws.String(clusterName)},
		{Name: aws.String("ServiceName"), Value: aws.String(serviceName)},
	}
	return c.getUtilizationMetrics(ctx, period, "AWS/ECS", "CPUUtilization", "MemoryUtilization", dims)
}

// GetTaskMetrics fetches task-level CPU and memory utilization from ECS
// Container Insights with enhanced observability.
func (c *Client) GetTaskMetrics(ctx context.Context, clusterName, serviceName, taskID, taskDefinition string, period time.Duration) (*ServiceMetrics, error) {
	if strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("task ID is required")
	}
	dims := []cwtypes.Dimension{
		{Name: aws.String("ClusterName"), Value: aws.String(clusterName)},
	}
	if serviceName != "" {
		dims = append(dims,
			cwtypes.Dimension{Name: aws.String("ServiceName"), Value: aws.String(serviceName)},
			cwtypes.Dimension{Name: aws.String("TaskId"), Value: aws.String(taskID)},
		)
	} else {
		family := taskDefinitionFamily(taskDefinition)
		if family == "" {
			return nil, fmt.Errorf("task definition family is required for standalone task metrics")
		}
		dims = append(dims,
			cwtypes.Dimension{Name: aws.String("TaskDefinitionFamily"), Value: aws.String(family)},
			cwtypes.Dimension{Name: aws.String("TaskId"), Value: aws.String(taskID)},
		)
	}
	return c.getUtilizationMetrics(ctx, period, "ECS/ContainerInsights", "TaskCpuUtilization", "TaskMemoryUtilization", dims)
}

func (c *Client) getUtilizationMetrics(ctx context.Context, period time.Duration, namespace, cpuMetric, memoryMetric string, dims []cwtypes.Dimension) (*ServiceMetrics, error) {
	now := time.Now()
	start := now.Add(-period)
	queries := utilizationMetricQueries(namespace, cpuMetric, memoryMetric, dims)
	snapshot, err := c.GetMetricSeries(ctx, model.MetricRequest{
		StartTime: start,
		EndTime:   now,
		MaxPoints: defaultMetricMaxPoints,
		Queries:   queries,
	})
	if err != nil {
		return nil, err
	}

	m := &ServiceMetrics{
		Timestamp: now,
		StartTime: snapshot.StartTime,
		EndTime:   snapshot.EndTime,
		Period:    snapshot.Period,
		Series:    snapshot.Series,
	}
	for _, series := range snapshot.Series {
		val, ok := latestSeriesValue(series)
		if !ok {
			continue
		}
		switch series.ID {
		case "cpu_avg":
			m.CPUAvg = val
			m.CPUAvgAvailable = true
		case "cpu_max":
			m.CPUMax = val
			m.CPUMaxAvailable = true
		case "mem_avg":
			m.MemAvg = val
			m.MemAvgAvailable = true
		case "mem_max":
			m.MemMax = val
			m.MemMaxAvailable = true
		}
	}
	return m, nil
}

func utilizationMetricQueries(namespace, cpuMetric, memoryMetric string, dims []cwtypes.Dimension) []model.MetricQuery {
	dimensions := make([]model.MetricDimension, 0, len(dims))
	for _, dimension := range dims {
		dimensions = append(dimensions, model.MetricDimension{
			Name: aws.ToString(dimension.Name), Value: aws.ToString(dimension.Value),
		})
	}
	specs := []struct {
		id, label, metric, stat string
	}{
		{"cpu_avg", "CPU average", cpuMetric, "Average"},
		{"cpu_max", "CPU maximum", cpuMetric, "Maximum"},
		{"mem_avg", "Memory average", memoryMetric, "Average"},
		{"mem_max", "Memory maximum", memoryMetric, "Maximum"},
	}
	queries := make([]model.MetricQuery, 0, len(specs))
	for _, spec := range specs {
		queries = append(queries, model.MetricQuery{
			ID: spec.id, Label: spec.label, Namespace: namespace, MetricName: spec.metric,
			Dimensions: dimensions, Statistic: spec.stat, Unit: "%",
		})
	}
	return queries
}

func latestSeriesValue(series model.MetricSeries) (float64, bool) {
	if len(series.Points) == 0 {
		return 0, false
	}
	return series.Points[len(series.Points)-1].Value, true
}

func taskDefinitionFamily(taskDefinition string) string {
	value := strings.TrimSpace(taskDefinition)
	if slash := strings.LastIndex(value, "/"); slash >= 0 {
		value = value[slash+1:]
	}
	if colon := strings.LastIndex(value, ":"); colon > 0 {
		value = value[:colon]
	}
	return value
}

func latestMetricValue(result cwtypes.MetricDataResult) (float64, bool) {
	if len(result.Values) == 0 {
		return 0, false
	}
	if len(result.Timestamps) != len(result.Values) {
		return result.Values[0], true
	}
	latest := 0
	for i := 1; i < len(result.Timestamps); i++ {
		if result.Timestamps[i].After(result.Timestamps[latest]) {
			latest = i
		}
	}
	return result.Values[latest], true
}

// ListAlarms returns CloudWatch alarms that match the given ECS service dimensions.
func (c *Client) ListAlarms(ctx context.Context, clusterName, serviceName string) ([]AlarmState, error) {
	// Search for alarms on the ECS namespace with matching dimensions.
	// CloudWatch doesn't provide a direct filter by dimension, so we list
	// alarms for the ECS namespace and filter client-side.
	var alarms []AlarmState
	paginator := cloudwatch.NewDescribeAlarmsPaginator(c.CW, &cloudwatch.DescribeAlarmsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, a := range page.MetricAlarms {
			if !matchesDimensions(a.Dimensions, clusterName, serviceName) {
				continue
			}
			alarms = append(alarms, AlarmState{
				Name:       derefStrAws(a.AlarmName),
				State:      string(a.StateValue),
				MetricName: derefStrAws(a.MetricName),
				UpdatedAt:  derefTimeAws(a.StateUpdatedTimestamp),
			})
		}
	}
	return alarms, nil
}

func matchesDimensions(dims []cwtypes.Dimension, cluster, service string) bool {
	hasCluster := false
	hasService := false
	for _, d := range dims {
		if d.Name != nil && d.Value != nil {
			if *d.Name == "ClusterName" && *d.Value == cluster {
				hasCluster = true
			}
			if *d.Name == "ServiceName" && *d.Value == service {
				hasService = true
			}
		}
	}
	return hasCluster && hasService
}

func derefTimeAws(t *time.Time) time.Time {
	if t != nil {
		return *t
	}
	return time.Time{}
}
