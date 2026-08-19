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
	return filtered, nil
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
