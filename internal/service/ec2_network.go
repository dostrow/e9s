package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dostrow/e9s/internal/model"
)

// EC2NetworkAPI is the low-level VPC, subnet, and security-group behavior used
// by shared frontend workflows.
type EC2NetworkAPI interface {
	ListEC2SecurityGroups(context.Context) ([]model.EC2SecurityGroup, error)
	DescribeEC2SecurityGroup(context.Context, string) (*model.EC2SecurityGroup, error)
	ListEC2SecurityGroupAssociations(context.Context, string) ([]model.ResourceRef, error)
	ListEC2VPCs(context.Context) ([]model.EC2VPC, error)
	DescribeEC2VPC(context.Context, string) (*model.EC2VPC, error)
	ListEC2Subnets(context.Context) ([]model.EC2Subnet, error)
	DescribeEC2Subnet(context.Context, string) (*model.EC2Subnet, error)
}

// EC2Network exposes UI-neutral VPC, subnet, and security-group workflows.
type EC2Network struct {
	api EC2NetworkAPI
}

func NewEC2Network(api EC2NetworkAPI) *EC2Network {
	return &EC2Network{api: api}
}

func (s *EC2Network) SecurityGroups(ctx context.Context, filter, vpcID string) ([]model.EC2SecurityGroup, error) {
	groups, err := s.api.ListEC2SecurityGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("list EC2 security groups: %w", err)
	}
	return FilterEC2SecurityGroups(groups, filter, vpcID), nil
}

// FilterEC2SecurityGroups filters and deterministically orders an already
// loaded security-group result set without another AWS request.
func FilterEC2SecurityGroups(groups []model.EC2SecurityGroup, filter, vpcID string) []model.EC2SecurityGroup {
	filter, vpcID = normalizedFilter(filter), strings.TrimSpace(vpcID)
	filtered := make([]model.EC2SecurityGroup, 0, len(groups))
	for _, group := range groups {
		if vpcID != "" && group.VpcID != vpcID {
			continue
		}
		if filter == "" || containsAny(filter, group.GroupID, group.Name, group.Description, group.VpcID, group.OwnerID) || tagsMatch(group.Tags, filter) {
			filtered = append(filtered, group)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return compareNameID(filtered[i].Name, filtered[i].GroupID, filtered[j].Name, filtered[j].GroupID)
	})
	return filtered
}

func (s *EC2Network) SecurityGroup(ctx context.Context, groupID string) (*model.EC2SecurityGroup, error) {
	groupID, err := requireResourceID("read EC2 security group", "security group ID", groupID)
	if err != nil {
		return nil, err
	}
	group, err := s.api.DescribeEC2SecurityGroup(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("read EC2 security group %q: %w", groupID, err)
	}
	if group == nil {
		return nil, fmt.Errorf("read EC2 security group %q: security group was not found", groupID)
	}
	associations, err := s.api.ListEC2SecurityGroupAssociations(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("read associations for EC2 security group %q: %w", groupID, err)
	}
	group.Associations = associations
	sort.SliceStable(group.Rules, func(i, j int) bool {
		return securityGroupRuleSortKey(group.Rules[i]) < securityGroupRuleSortKey(group.Rules[j])
	})
	sort.SliceStable(group.Associations, func(i, j int) bool {
		left, right := group.Associations[i], group.Associations[j]
		return strings.Join([]string{left.Kind, left.Name, left.ID}, "\x00") < strings.Join([]string{right.Kind, right.Name, right.ID}, "\x00")
	})
	return group, nil
}

func (s *EC2Network) VPCs(ctx context.Context, filter string) ([]model.EC2VPC, error) {
	vpcs, err := s.api.ListEC2VPCs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list EC2 VPCs: %w", err)
	}
	return FilterEC2VPCs(vpcs, filter), nil
}

// FilterEC2VPCs filters and orders an already loaded VPC result set.
func FilterEC2VPCs(vpcs []model.EC2VPC, filter string) []model.EC2VPC {
	filter = normalizedFilter(filter)
	filtered := make([]model.EC2VPC, 0, len(vpcs))
	for _, vpc := range vpcs {
		if filter == "" || containsAny(filter, vpc.VpcID, vpc.Name, vpc.State, vpc.Tenancy,
			vpc.OwnerID, vpc.DHCPOptions, strings.Join(vpc.CIDRs, " "), strings.Join(vpc.IPv6CIDRs, " ")) || tagsMatch(vpc.Tags, filter) {
			filtered = append(filtered, vpc)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].IsDefault != filtered[j].IsDefault {
			return filtered[i].IsDefault
		}
		return compareNameID(filtered[i].Name, filtered[i].VpcID, filtered[j].Name, filtered[j].VpcID)
	})
	return filtered
}

func (s *EC2Network) VPC(ctx context.Context, vpcID string) (*model.EC2VPC, error) {
	vpcID, err := requireResourceID("read EC2 VPC", "VPC ID", vpcID)
	if err != nil {
		return nil, err
	}
	vpc, err := s.api.DescribeEC2VPC(ctx, vpcID)
	if err != nil {
		return nil, fmt.Errorf("read EC2 VPC %q: %w", vpcID, err)
	}
	if vpc == nil {
		return nil, fmt.Errorf("read EC2 VPC %q: VPC was not found", vpcID)
	}
	sort.Strings(vpc.CIDRs)
	sort.Strings(vpc.IPv6CIDRs)
	return vpc, nil
}

func (s *EC2Network) Subnets(ctx context.Context, filter, vpcID string) ([]model.EC2Subnet, error) {
	subnets, err := s.api.ListEC2Subnets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list EC2 subnets: %w", err)
	}
	return FilterEC2Subnets(subnets, filter, vpcID), nil
}

// FilterEC2Subnets filters and orders an already loaded subnet result set.
func FilterEC2Subnets(subnets []model.EC2Subnet, filter, vpcID string) []model.EC2Subnet {
	filter, vpcID = normalizedFilter(filter), strings.TrimSpace(vpcID)
	filtered := make([]model.EC2Subnet, 0, len(subnets))
	for _, subnet := range subnets {
		if vpcID != "" && subnet.VpcID != vpcID {
			continue
		}
		if filter == "" || containsAny(filter, subnet.SubnetID, subnet.Name, subnet.VpcID,
			subnet.State, subnet.AZ, subnet.AZID, subnet.CIDR, strings.Join(subnet.IPv6CIDRs, " ")) || tagsMatch(subnet.Tags, filter) {
			filtered = append(filtered, subnet)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].AZ != filtered[j].AZ {
			return filtered[i].AZ < filtered[j].AZ
		}
		return compareNameID(filtered[i].Name, filtered[i].SubnetID, filtered[j].Name, filtered[j].SubnetID)
	})
	return filtered
}

func (s *EC2Network) Subnet(ctx context.Context, subnetID string) (*model.EC2Subnet, error) {
	subnetID, err := requireResourceID("read EC2 subnet", "subnet ID", subnetID)
	if err != nil {
		return nil, err
	}
	subnet, err := s.api.DescribeEC2Subnet(ctx, subnetID)
	if err != nil {
		return nil, fmt.Errorf("read EC2 subnet %q: %w", subnetID, err)
	}
	if subnet == nil {
		return nil, fmt.Errorf("read EC2 subnet %q: subnet was not found", subnetID)
	}
	sort.Strings(subnet.IPv6CIDRs)
	return subnet, nil
}

func securityGroupRuleSortKey(rule model.EC2SGRule) string {
	return strings.Join([]string{rule.Direction, rule.Protocol, rule.PortRange, rule.Source, rule.RuleID}, "\x00")
}

func normalizedFilter(filter string) string {
	return strings.ToLower(strings.TrimSpace(filter))
}

func containsAny(filter string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), filter) {
			return true
		}
	}
	return false
}

func tagsMatch(tags map[string]string, filter string) bool {
	for key, value := range tags {
		if containsAny(filter, key, value) {
			return true
		}
	}
	return false
}

func compareNameID(leftName, leftID, rightName, rightID string) bool {
	leftName, rightName = strings.ToLower(leftName), strings.ToLower(rightName)
	if leftName != rightName {
		return leftName < rightName
	}
	return leftID < rightID
}

func requireResourceID(action, field, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s: %s is required", action, field)
	}
	return value, nil
}
