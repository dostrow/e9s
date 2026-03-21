package aws

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type ServiceMetrics struct {
	CPUAvg    float64
	CPUMax    float64
	MemAvg    float64
	MemMax    float64
	Timestamp time.Time
}

type AlarmState struct {
	Name       string
	State      string // OK, ALARM, INSUFFICIENT_DATA
	MetricName string
	UpdatedAt  time.Time
}

// GetServiceMetrics fetches CPU and memory utilization for an ECS service.
func (c *Client) GetServiceMetrics(ctx context.Context, clusterName, serviceName string, period time.Duration) (*ServiceMetrics, error) {
	now := time.Now()
	start := now.Add(-period)
	periodSec := int32(60)

	dims := []cwtypes.Dimension{
		{Name: aws.String("ClusterName"), Value: aws.String(clusterName)},
		{Name: aws.String("ServiceName"), Value: aws.String(serviceName)},
	}

	out, err := c.CW.GetMetricData(ctx, &cloudwatch.GetMetricDataInput{
		StartTime: &start,
		EndTime:   &now,
		MetricDataQueries: []cwtypes.MetricDataQuery{
			{
				Id: aws.String("cpu_avg"),
				MetricStat: &cwtypes.MetricStat{
					Metric: &cwtypes.Metric{
						Namespace:  aws.String("AWS/ECS"),
						MetricName: aws.String("CPUUtilization"),
						Dimensions: dims,
					},
					Period: &periodSec,
					Stat:   aws.String("Average"),
				},
			},
			{
				Id: aws.String("cpu_max"),
				MetricStat: &cwtypes.MetricStat{
					Metric: &cwtypes.Metric{
						Namespace:  aws.String("AWS/ECS"),
						MetricName: aws.String("CPUUtilization"),
						Dimensions: dims,
					},
					Period: &periodSec,
					Stat:   aws.String("Maximum"),
				},
			},
			{
				Id: aws.String("mem_avg"),
				MetricStat: &cwtypes.MetricStat{
					Metric: &cwtypes.Metric{
						Namespace:  aws.String("AWS/ECS"),
						MetricName: aws.String("MemoryUtilization"),
						Dimensions: dims,
					},
					Period: &periodSec,
					Stat:   aws.String("Average"),
				},
			},
			{
				Id: aws.String("mem_max"),
				MetricStat: &cwtypes.MetricStat{
					Metric: &cwtypes.Metric{
						Namespace:  aws.String("AWS/ECS"),
						MetricName: aws.String("MemoryUtilization"),
						Dimensions: dims,
					},
					Period: &periodSec,
					Stat:   aws.String("Maximum"),
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	m := &ServiceMetrics{Timestamp: now}
	for _, r := range out.MetricDataResults {
		if r.Id == nil || len(r.Values) == 0 {
			continue
		}
		val := r.Values[len(r.Values)-1] // most recent
		switch *r.Id {
		case "cpu_avg":
			m.CPUAvg = val
		case "cpu_max":
			m.CPUMax = val
		case "mem_avg":
			m.MemAvg = val
		case "mem_max":
			m.MemMax = val
		}
	}
	return m, nil
}

// ListAlarms returns CloudWatch alarms that match the given ECS service dimensions.
func (c *Client) ListAlarms(ctx context.Context, clusterName, serviceName string) ([]AlarmState, error) {
	// Search for alarms on the ECS namespace with matching dimensions.
	// CloudWatch doesn't provide a direct filter by dimension, so we list
	// alarms for the ECS namespace and filter client-side.
	out, err := c.CW.DescribeAlarms(ctx, &cloudwatch.DescribeAlarmsInput{})
	if err != nil {
		return nil, err
	}

	var alarms []AlarmState
	for _, a := range out.MetricAlarms {
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
