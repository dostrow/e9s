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
