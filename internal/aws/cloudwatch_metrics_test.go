package aws

import (
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/dostrow/e9s/internal/model"
)

func TestMetricPeriodBoundsSeriesSize(t *testing.T) {
	tests := []struct {
		window time.Duration
		want   time.Duration
	}{
		{15 * time.Minute, time.Minute},
		{time.Hour, time.Minute},
		{6 * time.Hour, 2 * time.Minute},
		{24 * time.Hour, 5 * time.Minute},
		{7 * 24 * time.Hour, 34 * time.Minute},
		{14 * 24 * time.Hour, 68 * time.Minute},
		{30 * 24 * time.Hour, 145 * time.Minute},
	}
	for _, test := range tests {
		if got := metricPeriod(test.window, 300); got != test.want {
			t.Errorf("metricPeriod(%s, 300) = %s, want %s", test.window, got, test.want)
		}
	}
}

func TestCloudWatchMetricQueriesValidateAndConvert(t *testing.T) {
	queries, err := cloudWatchMetricQueries([]model.MetricQuery{{
		ID: "cpu", Label: "CPU", Namespace: "AWS/EC2", MetricName: "CPUUtilization",
		Dimensions: []model.MetricDimension{{Name: "InstanceId", Value: "i-1"}},
		Statistic:  "Maximum",
	}}, 300)
	if err != nil {
		t.Fatal(err)
	}
	if len(queries) != 1 || awssdk.ToString(queries[0].Id) != "cpu" {
		t.Fatalf("queries = %#v", queries)
	}
	metric := queries[0].MetricStat
	if metric == nil || metric.Period == nil || *metric.Period != 300 || awssdk.ToString(metric.Stat) != "Maximum" {
		t.Fatalf("metric stat = %#v", metric)
	}
	if len(metric.Metric.Dimensions) != 1 || awssdk.ToString(metric.Metric.Dimensions[0].Value) != "i-1" {
		t.Fatalf("dimensions = %#v", metric.Metric.Dimensions)
	}
	if _, err := cloudWatchMetricQueries([]model.MetricQuery{{ID: "cpu"}, {ID: "cpu"}}, 60); err == nil {
		t.Fatal("duplicate query ID was accepted")
	}
}

func TestMergeMetricResultsSortsLaterAndScalesDuringSnapshotAssembly(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	target := map[string]map[time.Time]float64{"latency": {}}
	specs := []model.MetricQuery{{ID: "latency", Scale: 1000}}
	mergeMetricResults(target, specs, []cwtypes.MetricDataResult{{
		Id:         awssdk.String("latency"),
		Timestamps: []time.Time{now, now.Add(-time.Minute)},
		Values:     []float64{0.002, 0.001},
	}})
	if got := target["latency"][now]; got != 2 {
		t.Fatalf("scaled value = %v, want 2", got)
	}
}
