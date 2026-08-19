package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

type fakeEC2NetworkAPI struct {
	groups  []model.EC2SecurityGroup
	group   *model.EC2SecurityGroup
	vpcs    []model.EC2VPC
	vpc     *model.EC2VPC
	subnets []model.EC2Subnet
	subnet  *model.EC2Subnet
	err     error
	id      string
}

func (f *fakeEC2NetworkAPI) ListEC2SecurityGroups(context.Context) ([]model.EC2SecurityGroup, error) {
	return append([]model.EC2SecurityGroup(nil), f.groups...), f.err
}
func (f *fakeEC2NetworkAPI) DescribeEC2SecurityGroup(_ context.Context, id string) (*model.EC2SecurityGroup, error) {
	f.id = id
	return f.group, f.err
}
func (f *fakeEC2NetworkAPI) ListEC2VPCs(context.Context) ([]model.EC2VPC, error) {
	return append([]model.EC2VPC(nil), f.vpcs...), f.err
}
func (f *fakeEC2NetworkAPI) DescribeEC2VPC(_ context.Context, id string) (*model.EC2VPC, error) {
	f.id = id
	return f.vpc, f.err
}
func (f *fakeEC2NetworkAPI) ListEC2Subnets(context.Context) ([]model.EC2Subnet, error) {
	return append([]model.EC2Subnet(nil), f.subnets...), f.err
}
func (f *fakeEC2NetworkAPI) DescribeEC2Subnet(_ context.Context, id string) (*model.EC2Subnet, error) {
	f.id = id
	return f.subnet, f.err
}

func TestEC2NetworkFiltersAndSortsResources(t *testing.T) {
	api := &fakeEC2NetworkAPI{
		groups: []model.EC2SecurityGroup{
			{GroupID: "sg-2", Name: "workers", VpcID: "vpc-2"},
			{GroupID: "sg-1", Name: "api", VpcID: "vpc-1", Tags: map[string]string{"Team": "Platform"}},
		},
		vpcs: []model.EC2VPC{
			{VpcID: "vpc-2", Name: "zeta"},
			{VpcID: "vpc-1", Name: "default", IsDefault: true},
		},
		subnets: []model.EC2Subnet{
			{SubnetID: "subnet-2", Name: "private", VpcID: "vpc-1", AZ: "us-east-2b"},
			{SubnetID: "subnet-1", Name: "public", VpcID: "vpc-1", AZ: "us-east-2a", CIDR: "10.0.1.0/24"},
			{SubnetID: "subnet-3", Name: "other", VpcID: "vpc-2", AZ: "us-east-2a"},
		},
	}
	service := NewEC2Network(api)
	groups, err := service.SecurityGroups(context.Background(), "platform", "vpc-1")
	if err != nil || len(groups) != 1 || groups[0].GroupID != "sg-1" {
		t.Fatalf("SecurityGroups() = %#v, %v", groups, err)
	}
	vpcs, err := service.VPCs(context.Background(), "")
	if err != nil || len(vpcs) != 2 || vpcs[0].VpcID != "vpc-1" {
		t.Fatalf("VPCs() = %#v, %v", vpcs, err)
	}
	subnets, err := service.Subnets(context.Background(), "10.0", "vpc-1")
	if err != nil || len(subnets) != 1 || subnets[0].SubnetID != "subnet-1" {
		t.Fatalf("Subnets() = %#v, %v", subnets, err)
	}
}

func TestEC2NetworkDetailsValidateAndSort(t *testing.T) {
	api := &fakeEC2NetworkAPI{
		group: &model.EC2SecurityGroup{GroupID: "sg-1", Rules: []model.EC2SGRule{
			{RuleID: "sgr-2", Direction: "outbound"},
			{RuleID: "sgr-1", Direction: "inbound"},
		}},
		vpc:    &model.EC2VPC{VpcID: "vpc-1", CIDRs: []string{"10.1.0.0/16", "10.0.0.0/16"}},
		subnet: &model.EC2Subnet{SubnetID: "subnet-1", IPv6CIDRs: []string{"b", "a"}},
	}
	service := NewEC2Network(api)
	group, err := service.SecurityGroup(context.Background(), " sg-1 ")
	if err != nil || api.id != "sg-1" || group.Rules[0].RuleID != "sgr-1" {
		t.Fatalf("SecurityGroup() = %#v, id %q, %v", group, api.id, err)
	}
	vpc, err := service.VPC(context.Background(), "vpc-1")
	if err != nil || vpc.CIDRs[0] != "10.0.0.0/16" {
		t.Fatalf("VPC() = %#v, %v", vpc, err)
	}
	subnet, err := service.Subnet(context.Background(), "subnet-1")
	if err != nil || subnet.IPv6CIDRs[0] != "a" {
		t.Fatalf("Subnet() = %#v, %v", subnet, err)
	}
	if _, err := service.SecurityGroup(context.Background(), " "); err == nil {
		t.Fatal("SecurityGroup(empty) succeeded")
	}
}

type fakeEBSAPI struct {
	volumes []model.EC2Volume
	volume  *model.EC2Volume
	err     error
	id      string
}

func (f *fakeEBSAPI) ListEBSVolumes(context.Context) ([]model.EC2Volume, error) {
	return append([]model.EC2Volume(nil), f.volumes...), f.err
}
func (f *fakeEBSAPI) DescribeEBSVolume(_ context.Context, id string) (*model.EC2Volume, error) {
	f.id = id
	return f.volume, f.err
}

func TestEBSFiltersUnattachedFirstAndSortsAttachments(t *testing.T) {
	api := &fakeEBSAPI{volumes: []model.EC2Volume{
		{VolumeID: "vol-2", Name: "attached", Attachments: []model.EC2VolumeAttachment{{InstanceID: "i-2"}}},
		{VolumeID: "vol-1", Name: "orphan", Tags: map[string]string{"Owner": "Platform"}},
	}}
	service := NewEBS(api)
	volumes, err := service.List(context.Background(), "")
	if err != nil || len(volumes) != 2 || volumes[0].VolumeID != "vol-1" {
		t.Fatalf("List() = %#v, %v", volumes, err)
	}
	filtered, err := service.List(context.Background(), "platform")
	if err != nil || len(filtered) != 1 || filtered[0].VolumeID != "vol-1" {
		t.Fatalf("List(platform) = %#v, %v", filtered, err)
	}
	api.volume = &model.EC2Volume{VolumeID: "vol-2", Attachments: []model.EC2VolumeAttachment{
		{InstanceID: "i-2", DeviceName: "/dev/xvdb"}, {InstanceID: "i-1", DeviceName: "/dev/xvda"},
	}}
	detail, err := service.Detail(context.Background(), " vol-2 ")
	if err != nil || api.id != "vol-2" || detail.Attachments[0].InstanceID != "i-1" {
		t.Fatalf("Detail() = %#v, id %q, %v", detail, api.id, err)
	}
}

type fakeLoadBalancingAPI struct {
	loadBalancers []model.EC2LoadBalancer
	detail        *model.EC2LoadBalancer
	err           error
	arn           string
}

func (f *fakeLoadBalancingAPI) ListEC2LoadBalancers(context.Context) ([]model.EC2LoadBalancer, error) {
	return append([]model.EC2LoadBalancer(nil), f.loadBalancers...), f.err
}
func (f *fakeLoadBalancingAPI) DescribeEC2LoadBalancer(_ context.Context, arn string) (*model.EC2LoadBalancer, error) {
	f.arn = arn
	return f.detail, f.err
}

func TestLoadBalancingFiltersAndSortsComposedDetail(t *testing.T) {
	api := &fakeLoadBalancingAPI{loadBalancers: []model.EC2LoadBalancer{
		{ARN: "nlb", Name: "network", Type: "network", VpcID: "vpc-2"},
		{ARN: "alb", Name: "application", Type: "application", DNSName: "api.example", SecurityGroups: []string{"sg-1"}},
	}}
	service := NewLoadBalancing(api)
	loadBalancers, err := service.List(context.Background(), "sg-1")
	if err != nil || len(loadBalancers) != 1 || loadBalancers[0].ARN != "alb" {
		t.Fatalf("List() = %#v, %v", loadBalancers, err)
	}
	api.detail = &model.EC2LoadBalancer{ARN: "alb",
		Listeners: []model.EC2Listener{{Port: 443}, {Port: 80}},
		TargetGroups: []model.EC2TargetGroup{
			{Name: "z", Targets: []model.EC2TargetHealth{{ID: "i-2"}, {ID: "i-1"}}},
			{Name: "a"},
		},
	}
	detail, err := service.Detail(context.Background(), " alb ")
	if err != nil || api.arn != "alb" || detail.Listeners[0].Port != 80 || detail.TargetGroups[0].Name != "a" || detail.TargetGroups[1].Targets[0].ID != "i-1" {
		t.Fatalf("Detail() = %#v, arn %q, %v", detail, api.arn, err)
	}
}

func TestEC2ResourceServicesWrapErrors(t *testing.T) {
	denied := errors.New("denied")
	if _, err := NewEC2Network(&fakeEC2NetworkAPI{err: denied}).VPCs(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "list EC2 VPCs") {
		t.Fatalf("VPCs() error = %v", err)
	}
	if _, err := NewEBS(&fakeEBSAPI{err: denied}).List(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "list EBS volumes") {
		t.Fatalf("EBS List() error = %v", err)
	}
	if _, err := NewLoadBalancing(&fakeLoadBalancingAPI{err: denied}).List(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "list EC2 load balancers") {
		t.Fatalf("LoadBalancing List() error = %v", err)
	}
}
