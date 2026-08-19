package service

import (
	"reflect"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

func TestECSTaskResourceRefsIncludesTaskAndServiceInfrastructure(t *testing.T) {
	task := model.Task{
		EC2InstanceID: "i-1", VpcID: "vpc-1", SubnetID: "subnet-1",
		SecurityGroups: []model.EC2SecurityGroupRef{{ID: "sg-1", Name: "web"}},
		VolumeIDs:      []string{"vol-1", "vol-1"},
	}
	parent := &model.Service{TargetGroups: []model.ResourceRef{{Kind: "ec2-target-group", ID: "tg-1"}}}
	got := ECSTaskResourceRefs(task, parent)
	want := []model.ResourceRef{
		{Kind: "ec2-instance", ID: "i-1"}, {Kind: "ec2-vpc", ID: "vpc-1"},
		{Kind: "ec2-subnet", ID: "subnet-1"}, {Kind: "ec2-security-group", ID: "sg-1", Name: "web"},
		{Kind: "ec2-volume", ID: "vol-1"}, {Kind: "ec2-target-group", ID: "tg-1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ECSTaskResourceRefs() = %#v, want %#v", got, want)
	}
}
