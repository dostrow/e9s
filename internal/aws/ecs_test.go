package aws

import (
	"reflect"
	"testing"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	aastypes "github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/dostrow/e9s/internal/model"
)

func TestStoppedTasksInput(t *testing.T) {
	input := stoppedTasksInput("prod", "next-page", 50)
	if sdkaws.ToString(input.Cluster) != "prod" || input.DesiredStatus != types.DesiredStatusStopped {
		t.Fatalf("stoppedTasksInput() = %#v", input)
	}
	if sdkaws.ToString(input.NextToken) != "next-page" || sdkaws.ToInt32(input.MaxResults) != 50 {
		t.Fatalf("pagination = %#v", input)
	}
	if got := sdkaws.ToInt32(stoppedTasksInput("prod", "", 500).MaxResults); got != 100 {
		t.Fatalf("clamped max results = %d", got)
	}
}

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

func TestScaleInSuspendedStatePreservesOtherControls(t *testing.T) {
	scaleOut, scheduled, scaleIn := true, true, false
	current := &aastypes.SuspendedState{
		DynamicScalingInSuspended:  &scaleIn,
		DynamicScalingOutSuspended: &scaleOut,
		ScheduledScalingSuspended:  &scheduled,
	}
	updated := scaleInSuspendedState(current, true)
	if !*updated.DynamicScalingInSuspended || !*updated.DynamicScalingOutSuspended || !*updated.ScheduledScalingSuspended {
		t.Fatalf("scaleInSuspendedState() = %#v", updated)
	}
	if *current.DynamicScalingInSuspended {
		t.Fatal("scaleInSuspendedState mutated the source state")
	}
}
