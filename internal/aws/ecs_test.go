package aws

import (
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/dostrow/e9s/internal/model"
)

func TestBuildRunTaskInput(t *testing.T) {
	input := buildRunTaskInput(model.RunTaskRequest{
		Cluster:              "prod",
		TaskDefinition:       "nightly:7",
		LaunchType:           "FARGATE",
		Count:                2,
		Subnets:              []string{"subnet-a", "subnet-b"},
		SecurityGroups:       []string{"sg-a"},
		AssignPublicIP:       true,
		EnableExecuteCommand: true,
		Group:                "family:nightly",
	})
	if *input.Cluster != "prod" || *input.TaskDefinition != "nightly:7" || *input.Count != 2 || input.LaunchType != types.LaunchTypeFargate {
		t.Fatalf("buildRunTaskInput() = %#v", input)
	}
	if !input.EnableExecuteCommand || *input.StartedBy != "e9s" || *input.Group != "family:nightly" {
		t.Fatalf("metadata = %#v", input)
	}
	network := input.NetworkConfiguration.AwsvpcConfiguration
	if !reflect.DeepEqual(network.Subnets, []string{"subnet-a", "subnet-b"}) || !reflect.DeepEqual(network.SecurityGroups, []string{"sg-a"}) || network.AssignPublicIp != types.AssignPublicIpEnabled {
		t.Fatalf("network configuration = %#v", network)
	}
}

func TestBuildRunTaskInputUsesClusterDefaults(t *testing.T) {
	input := buildRunTaskInput(model.RunTaskRequest{Cluster: "prod", TaskDefinition: "batch:1", Count: 1})
	if input.LaunchType != "" || input.NetworkConfiguration != nil {
		t.Fatalf("defaults unexpectedly overridden: %#v", input)
	}
}
