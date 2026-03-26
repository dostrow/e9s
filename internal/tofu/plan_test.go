package tofu

import (
	"testing"
)

func TestParsePlan_CreateUpdateDelete(t *testing.T) {
	jsonData := `{
		"resource_changes": [
			{
				"address": "aws_s3_bucket.new",
				"type": "aws_s3_bucket",
				"name": "new",
				"change": {
					"actions": ["create"],
					"before": null,
					"after": {"bucket": "my-bucket", "acl": "private"}
				}
			},
			{
				"address": "aws_ecs_service.api",
				"type": "aws_ecs_service",
				"name": "api",
				"change": {
					"actions": ["update"],
					"before": {"desired_count": 2, "name": "api"},
					"after": {"desired_count": 4, "name": "api"}
				}
			},
			{
				"address": "aws_security_group.old",
				"type": "aws_security_group",
				"name": "old",
				"change": {
					"actions": ["delete"],
					"before": {"name": "old-sg"},
					"after": null
				}
			}
		]
	}`

	plan, err := ParsePlan(jsonData)
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}

	if plan.CreateCount != 1 {
		t.Errorf("CreateCount = %d, want 1", plan.CreateCount)
	}
	if plan.UpdateCount != 1 {
		t.Errorf("UpdateCount = %d, want 1", plan.UpdateCount)
	}
	if plan.DeleteCount != 1 {
		t.Errorf("DeleteCount = %d, want 1", plan.DeleteCount)
	}
	if len(plan.Changes) != 3 {
		t.Fatalf("len(Changes) = %d, want 3", len(plan.Changes))
	}

	// Should be sorted: delete, update, create
	if plan.Changes[0].Action != "delete" {
		t.Errorf("Changes[0].Action = %q, want delete", plan.Changes[0].Action)
	}
	if plan.Changes[1].Action != "update" {
		t.Errorf("Changes[1].Action = %q, want update", plan.Changes[1].Action)
	}
	if plan.Changes[2].Action != "create" {
		t.Errorf("Changes[2].Action = %q, want create", plan.Changes[2].Action)
	}
}

func TestParsePlan_Replace(t *testing.T) {
	jsonData := `{
		"resource_changes": [{
			"address": "aws_instance.web",
			"type": "aws_instance",
			"name": "web",
			"change": {
				"actions": ["delete", "create"],
				"before": {"ami": "ami-old"},
				"after": {"ami": "ami-new"}
			}
		}]
	}`

	plan, err := ParsePlan(jsonData)
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}

	if plan.ReplaceCount != 1 {
		t.Errorf("ReplaceCount = %d, want 1", plan.ReplaceCount)
	}
	if plan.Changes[0].Action != "replace" {
		t.Errorf("Action = %q, want replace", plan.Changes[0].Action)
	}
}

func TestParsePlan_NoOp(t *testing.T) {
	jsonData := `{
		"resource_changes": [{
			"address": "aws_vpc.main",
			"type": "aws_vpc",
			"name": "main",
			"change": {
				"actions": ["no-op"],
				"before": {"cidr_block": "10.0.0.0/16"},
				"after": {"cidr_block": "10.0.0.0/16"}
			}
		}]
	}`

	plan, err := ParsePlan(jsonData)
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}

	if len(plan.Changes) != 0 {
		t.Errorf("len(Changes) = %d, want 0 (no-ops excluded)", len(plan.Changes))
	}
	if plan.NoOpCount != 1 {
		t.Errorf("NoOpCount = %d, want 1", plan.NoOpCount)
	}
}

func TestComputeDiffs(t *testing.T) {
	before := map[string]any{"a": "old", "b": "same", "c": "removed"}
	after := map[string]any{"a": "new", "b": "same", "d": "added"}

	diffs := computeDiffs(before, after)

	diffMap := make(map[string]AttrDiff)
	for _, d := range diffs {
		diffMap[d.Path] = d
	}

	if d, ok := diffMap["a"]; !ok || d.Action != "change" || d.Before != "old" || d.After != "new" {
		t.Errorf("'a' diff incorrect: %+v", d)
	}
	if _, ok := diffMap["b"]; ok {
		t.Error("'b' should not appear in diffs (unchanged)")
	}
	if d, ok := diffMap["c"]; !ok || d.Action != "remove" {
		t.Errorf("'c' should be remove: %+v", d)
	}
	if d, ok := diffMap["d"]; !ok || d.Action != "add" {
		t.Errorf("'d' should be add: %+v", d)
	}
}

func TestFormatPlanSummary(t *testing.T) {
	p := &PlanResult{CreateCount: 2, UpdateCount: 1, DeleteCount: 0}
	s := FormatPlanSummary(p)
	if s != "2 to create, 1 to update" {
		t.Errorf("FormatPlanSummary = %q", s)
	}
}

func TestFormatPlanSummary_NoChanges(t *testing.T) {
	p := &PlanResult{}
	s := FormatPlanSummary(p)
	if s != "No changes" {
		t.Errorf("FormatPlanSummary = %q, want 'No changes'", s)
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		input any
		want  string
	}{
		{nil, "<nil>"},
		{"hello", "hello"},
		{float64(42), "42"},
		{float64(3.14), "3.14"},
		{true, "true"},
		{false, "false"},
	}
	for _, tt := range tests {
		got := formatValue(tt.input)
		if got != tt.want {
			t.Errorf("formatValue(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestActionOrder(t *testing.T) {
	if actionOrder("delete") >= actionOrder("replace") {
		t.Error("delete should sort before replace")
	}
	if actionOrder("replace") >= actionOrder("update") {
		t.Error("replace should sort before update")
	}
	if actionOrder("update") >= actionOrder("create") {
		t.Error("update should sort before create")
	}
}
