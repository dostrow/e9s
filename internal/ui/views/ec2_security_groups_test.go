package views

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

func TestEC2SecurityGroupsFiltersAndSelects(t *testing.T) {
	view := NewEC2SecurityGroups().SetGroups([]model.EC2SecurityGroup{
		{GroupID: "sg-2", Name: "workers", VpcID: "vpc-2"},
		{GroupID: "sg-1", Name: "api", VpcID: "vpc-1", Tags: map[string]string{"Team": "Platform"}},
	})
	view.filter = "platform"
	selected := view.SelectedGroup()
	if selected == nil || selected.GroupID != "sg-1" {
		t.Fatalf("SelectedGroup() = %#v", selected)
	}
	if got := view.View(); !strings.Contains(got, "sg-1") || strings.Contains(got, "sg-2") {
		t.Fatalf("filtered View() = %q", got)
	}
}

func TestEC2SecurityGroupDetailShowsOpenHintForAssociations(t *testing.T) {
	view := NewEC2SecurityGroupDetail(&model.EC2SecurityGroup{
		GroupID:      "sg-1",
		Associations: []model.ResourceRef{{Kind: "ec2-instance", ID: "i-1"}},
	})
	got := view.View()
	if !strings.Contains(got, "i-1") || !strings.Contains(got, "[o] open linked resource") {
		t.Fatalf("View() = %q", got)
	}
}
