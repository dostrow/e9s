package aws

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

func TestLatestMetricValueUsesNewestTimestamp(t *testing.T) {
	now := time.Now()
	result := cwtypes.MetricDataResult{
		Values:     []float64{20, 40, 30},
		Timestamps: []time.Time{now.Add(-2 * time.Minute), now, now.Add(-time.Minute)},
	}
	value, ok := latestMetricValue(result)
	if !ok || value != 40 {
		t.Fatalf("latestMetricValue() = %v, %v", value, ok)
	}
}

func TestMatchesServiceDimensions(t *testing.T) {
	dimensions := []cwtypes.Dimension{
		{Name: aws.String("ClusterName"), Value: aws.String("prod")},
		{Name: aws.String("ServiceName"), Value: aws.String("api")},
	}
	if !matchesDimensions(dimensions, "prod", "api") {
		t.Fatal("matchesDimensions() = false, want true")
	}
	if matchesDimensions(dimensions, "prod", "worker") {
		t.Fatal("matchesDimensions() matched wrong service")
	}
}

func TestTaskUtilizationMetricQueries(t *testing.T) {
	dimensions := []cwtypes.Dimension{
		{Name: aws.String("ClusterName"), Value: aws.String("prod")},
		{Name: aws.String("ServiceName"), Value: aws.String("api")},
		{Name: aws.String("TaskId"), Value: aws.String("task-1")},
	}
	queries := utilizationMetricQueries("ECS/ContainerInsights", "TaskCpuUtilization", "TaskMemoryUtilization", dimensions)
	if len(queries) != 4 {
		t.Fatalf("len(utilizationMetricQueries()) = %d, want 4", len(queries))
	}
	for i, query := range queries {
		if query.Namespace != "ECS/ContainerInsights" {
			t.Fatalf("query %d namespace = %q", i, query.Namespace)
		}
		if len(query.Dimensions) != 3 || query.Dimensions[2].Name != "TaskId" {
			t.Fatalf("query %d dimensions = %#v", i, query.Dimensions)
		}
	}
	if got := queries[0].MetricName; got != "TaskCpuUtilization" {
		t.Fatalf("CPU metric name = %q", got)
	}
	if got := queries[2].MetricName; got != "TaskMemoryUtilization" {
		t.Fatalf("memory metric name = %q", got)
	}
}

func TestTaskDefinitionFamily(t *testing.T) {
	for input, want := range map[string]string{
		"api:7": "api",
		"arn:aws:ecs:us-east-1:123456789012:task-definition/api:7": "api",
		"nightly": "nightly",
	} {
		if got := taskDefinitionFamily(input); got != want {
			t.Errorf("taskDefinitionFamily(%q) = %q, want %q", input, got, want)
		}
	}
}
