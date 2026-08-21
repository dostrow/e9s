package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/dostrow/e9s/internal/model"
)

func TestApplyTaskNetworkInterfaces(t *testing.T) {
	tasks := []model.Task{{NetworkInterfaceID: "eni-1", SubnetID: "subnet-from-ecs"}}
	applyTaskNetworkInterfaces(tasks, []ec2types.NetworkInterface{{
		NetworkInterfaceId: aws.String("eni-1"), VpcId: aws.String("vpc-1"),
		SubnetId: aws.String("subnet-1"), PrivateIpAddress: aws.String("10.0.0.8"),
		Groups: []ec2types.GroupIdentifier{{GroupId: aws.String("sg-1"), GroupName: aws.String("web")}},
	}})
	got := tasks[0]
	if got.VpcID != "vpc-1" || got.SubnetID != "subnet-1" || got.PrivateIP != "10.0.0.8" || len(got.SecurityGroups) != 1 || got.SecurityGroups[0].ID != "sg-1" {
		t.Fatalf("enriched task = %#v", got)
	}
}
