package views

import (
	"strings"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/aws"
)

func TestTaskDefDetailSummaryIncludesCoreFields(t *testing.T) {
	td := &aws.TaskDefSummary{
		ARN:              "arn:aws:ecs:::task-definition/api:12",
		Family:           "api",
		Revision:         12,
		Status:           "ACTIVE",
		CPU:              "256",
		Memory:           "512",
		NetworkMode:      "awsvpc",
		TaskRoleArn:      "arn:task-role",
		ExecutionRoleArn: "arn:exec-role",
		RegisteredAt:     time.Unix(1700000000, 0),
		Containers: []aws.TaskDefContainer{
			{Name: "api", Image: "repo/api:12"},
		},
		RawJSON: "{}",
	}
	m := NewTaskDefDetail(td)
	lines := strings.Join(m.summaryLines(), "\n")
	if !strings.Contains(lines, "arn:aws:ecs:::task-definition/api:12") {
		t.Fatalf("summary missing ARN: %s", lines)
	}
	if !strings.Contains(lines, "awsvpc") {
		t.Fatalf("summary missing network mode: %s", lines)
	}
	if !strings.Contains(lines, "repo/api:12") {
		t.Fatalf("summary missing container image: %s", lines)
	}
}

func TestTaskDefDetailJSONLines(t *testing.T) {
	m := NewTaskDefDetail(&aws.TaskDefSummary{RawJSON: "{\n  \"family\": \"api\"\n}"})
	lines := m.jsonLines()
	if len(lines) < 2 {
		t.Fatalf("expected json lines, got %#v", lines)
	}
	if !strings.HasPrefix(lines[0], "  {") {
		t.Fatalf("json line not indented: %#v", lines)
	}
}
