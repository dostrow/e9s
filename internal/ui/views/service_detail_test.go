package views

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

func TestServiceDetailShowsConfiguredResources(t *testing.T) {
	view := NewServiceDetail(&model.Service{
		Name:           "api",
		SecurityGroups: []model.EC2SecurityGroupRef{{ID: "sg-123"}},
		TargetGroups:   []model.ResourceRef{{Kind: "ec2-target-group", ID: "arn:target-group:api"}},
	}).View()
	for _, want := range []string{"Configured Resources", "sg-123", "arn:target-group:api", "[o] open linked resource"} {
		if !strings.Contains(view, want) {
			t.Fatalf("service detail omitted %q:\n%s", want, view)
		}
	}
}

func TestServiceDetailStatesWhenECSReturnsNoTargetGroups(t *testing.T) {
	view := NewServiceDetail(&model.Service{Name: "api"}).View()
	if !strings.Contains(view, "Target Groups:") || !strings.Contains(view, "none returned by ECS") {
		t.Fatalf("service detail did not make empty target groups explicit:\n%s", view)
	}
}
