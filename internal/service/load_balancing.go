package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dostrow/e9s/internal/model"
)

// LoadBalancingAPI is the low-level ELBv2 behavior used by shared frontend
// workflows.
type LoadBalancingAPI interface {
	ListEC2LoadBalancers(context.Context) ([]model.EC2LoadBalancer, error)
	DescribeEC2LoadBalancer(context.Context, string) (*model.EC2LoadBalancer, error)
	ListEC2TargetGroups(context.Context) ([]model.EC2TargetGroup, error)
	DescribeEC2TargetGroup(context.Context, string) (*model.EC2TargetGroup, error)
}

func (s *LoadBalancing) ListTargetGroups(ctx context.Context, filter string) ([]model.EC2TargetGroup, error) {
	targetGroups, err := s.api.ListEC2TargetGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("list EC2 target groups: %w", err)
	}
	return FilterEC2TargetGroups(targetGroups, filter), nil
}

func FilterEC2TargetGroups(targetGroups []model.EC2TargetGroup, filter string) []model.EC2TargetGroup {
	filter = normalizedFilter(filter)
	filtered := make([]model.EC2TargetGroup, 0, len(targetGroups))
	for _, targetGroup := range targetGroups {
		values := []string{targetGroup.ARN, targetGroup.Name, targetGroup.Protocol, targetGroup.ProtocolVersion,
			targetGroup.TargetType, targetGroup.VpcID, targetGroup.HealthCheckProtocol, targetGroup.HealthCheckPath}
		values = append(values, targetGroup.LoadBalancerARNs...)
		if filter == "" || containsAny(filter, values...) {
			filtered = append(filtered, targetGroup)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return compareNameID(filtered[i].Name, filtered[i].ARN, filtered[j].Name, filtered[j].ARN)
	})
	return filtered
}

func (s *LoadBalancing) TargetGroup(ctx context.Context, arn string) (*model.EC2TargetGroup, error) {
	arn, err := requireResourceID("read EC2 target group", "target group ARN", arn)
	if err != nil {
		return nil, err
	}
	targetGroup, err := s.api.DescribeEC2TargetGroup(ctx, arn)
	if err != nil {
		return nil, fmt.Errorf("read EC2 target group %q: %w", arn, err)
	}
	if targetGroup == nil {
		return nil, fmt.Errorf("read EC2 target group %q: target group was not found", arn)
	}
	sort.Strings(targetGroup.LoadBalancerARNs)
	sort.SliceStable(targetGroup.Targets, func(i, j int) bool {
		left, right := targetGroup.Targets[i], targetGroup.Targets[j]
		return strings.Join([]string{left.ID, fmt.Sprint(left.Port), left.AZ}, "\x00") <
			strings.Join([]string{right.ID, fmt.Sprint(right.Port), right.AZ}, "\x00")
	})
	return targetGroup, nil
}

// LoadBalancing exposes ALB/NLB discovery and composed detail workflows.
type LoadBalancing struct {
	api LoadBalancingAPI
}

func NewLoadBalancing(api LoadBalancingAPI) *LoadBalancing {
	return &LoadBalancing{api: api}
}

func (s *LoadBalancing) List(ctx context.Context, filter string) ([]model.EC2LoadBalancer, error) {
	loadBalancers, err := s.api.ListEC2LoadBalancers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list EC2 load balancers: %w", err)
	}
	return FilterEC2LoadBalancers(loadBalancers, filter), nil
}

// FilterEC2LoadBalancers filters and consistently orders an already-loaded
// load-balancer collection. Frontends use it for responsive local filtering.
func FilterEC2LoadBalancers(loadBalancers []model.EC2LoadBalancer, filter string) []model.EC2LoadBalancer {
	filter = normalizedFilter(filter)
	filtered := make([]model.EC2LoadBalancer, 0, len(loadBalancers))
	for _, loadBalancer := range loadBalancers {
		values := []string{loadBalancer.ARN, loadBalancer.Name, loadBalancer.Type,
			loadBalancer.Scheme, loadBalancer.State, loadBalancer.DNSName,
			loadBalancer.VpcID, loadBalancer.IPAddressType}
		values = append(values, loadBalancer.SecurityGroups...)
		for _, zone := range loadBalancer.AvailabilityZones {
			values = append(values, zone.AZ, zone.SubnetID, zone.IPv4, zone.IPv6)
		}
		if filter == "" || containsAny(filter, values...) {
			filtered = append(filtered, loadBalancer)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Type != filtered[j].Type {
			return filtered[i].Type < filtered[j].Type
		}
		return compareNameID(filtered[i].Name, filtered[i].ARN, filtered[j].Name, filtered[j].ARN)
	})
	return filtered
}

func (s *LoadBalancing) Detail(ctx context.Context, arn string) (*model.EC2LoadBalancer, error) {
	arn, err := requireResourceID("read EC2 load balancer", "load balancer ARN", arn)
	if err != nil {
		return nil, err
	}
	loadBalancer, err := s.api.DescribeEC2LoadBalancer(ctx, arn)
	if err != nil {
		return nil, fmt.Errorf("read EC2 load balancer %q: %w", arn, err)
	}
	if loadBalancer == nil {
		return nil, fmt.Errorf("read EC2 load balancer %q: load balancer was not found", arn)
	}
	sort.SliceStable(loadBalancer.AvailabilityZones, func(i, j int) bool {
		return loadBalancer.AvailabilityZones[i].AZ < loadBalancer.AvailabilityZones[j].AZ
	})
	sort.Strings(loadBalancer.SecurityGroups)
	sort.SliceStable(loadBalancer.Listeners, func(i, j int) bool {
		if loadBalancer.Listeners[i].Port != loadBalancer.Listeners[j].Port {
			return loadBalancer.Listeners[i].Port < loadBalancer.Listeners[j].Port
		}
		return loadBalancer.Listeners[i].Protocol < loadBalancer.Listeners[j].Protocol
	})
	sort.SliceStable(loadBalancer.TargetGroups, func(i, j int) bool {
		return compareNameID(loadBalancer.TargetGroups[i].Name, loadBalancer.TargetGroups[i].ARN,
			loadBalancer.TargetGroups[j].Name, loadBalancer.TargetGroups[j].ARN)
	})
	for i := range loadBalancer.TargetGroups {
		sort.SliceStable(loadBalancer.TargetGroups[i].Targets, func(left, right int) bool {
			leftTarget, rightTarget := loadBalancer.TargetGroups[i].Targets[left], loadBalancer.TargetGroups[i].Targets[right]
			return strings.Join([]string{leftTarget.ID, fmt.Sprint(leftTarget.Port), leftTarget.AZ}, "\x00") <
				strings.Join([]string{rightTarget.ID, fmt.Sprint(rightTarget.Port), rightTarget.AZ}, "\x00")
		})
	}
	return loadBalancer, nil
}
