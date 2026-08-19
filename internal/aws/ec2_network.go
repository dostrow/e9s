package aws

import (
	"context"
	"fmt"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/dostrow/e9s/internal/model"
)

// ListEC2SecurityGroups returns every security group in the active region.
func (c *Client) ListEC2SecurityGroups(ctx context.Context) ([]model.EC2SecurityGroup, error) {
	var groups []model.EC2SecurityGroup
	paginator := ec2.NewDescribeSecurityGroupsPaginator(c.EC2, &ec2.DescribeSecurityGroupsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, group := range page.SecurityGroups {
			groups = append(groups, securityGroupFromSDK(group))
		}
	}
	return groups, nil
}

// DescribeEC2SecurityGroup returns a security group and rule identifiers.
func (c *Client) DescribeEC2SecurityGroup(ctx context.Context, groupID string) (*model.EC2SecurityGroup, error) {
	out, err := c.EC2.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{GroupIds: []string{groupID}})
	if err != nil {
		return nil, err
	}
	if len(out.SecurityGroups) == 0 {
		return nil, fmt.Errorf("security group %s not found", groupID)
	}
	group := securityGroupFromSDK(out.SecurityGroups[0])
	rules, err := c.describeEC2SecurityGroupRules(ctx, groupID)
	if err != nil {
		return nil, err
	}
	group.Rules = rules
	return &group, nil
}

// ListEC2SecurityGroupAssociations returns network interfaces and their
// attached instances for one security group.
func (c *Client) ListEC2SecurityGroupAssociations(ctx context.Context, groupID string) ([]model.ResourceRef, error) {
	input := &ec2.DescribeNetworkInterfacesInput{Filters: []ec2types.Filter{{
		Name: awssdk.String("group-id"), Values: []string{groupID},
	}}}
	paginator := ec2.NewDescribeNetworkInterfacesPaginator(c.EC2, input)
	var associations []model.ResourceRef
	seen := make(map[string]struct{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, networkInterface := range page.NetworkInterfaces {
			ref := model.ResourceRef{
				Kind: "network-interface",
				ID:   derefStrAws(networkInterface.NetworkInterfaceId),
				Name: derefStrAws(networkInterface.Description),
			}
			if networkInterface.Attachment != nil {
				if instanceID := derefStrAws(networkInterface.Attachment.InstanceId); instanceID != "" {
					ref = model.ResourceRef{Kind: "ec2-instance", ID: instanceID, Name: ref.Name}
				}
			}
			key := ref.Kind + "\x00" + ref.ID
			if ref.ID == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			associations = append(associations, ref)
		}
	}
	return associations, nil
}

func (c *Client) describeEC2SecurityGroupRules(ctx context.Context, groupID string) ([]model.EC2SGRule, error) {
	input := &ec2.DescribeSecurityGroupRulesInput{Filters: []ec2types.Filter{{
		Name: awssdk.String("group-id"), Values: []string{groupID},
	}}}
	var rules []model.EC2SGRule
	paginator := ec2.NewDescribeSecurityGroupRulesPaginator(c.EC2, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, rule := range page.SecurityGroupRules {
			rules = append(rules, securityGroupRuleFromSDK(rule))
		}
	}
	return rules, nil
}

// ListEC2VPCs returns every VPC in the active region.
func (c *Client) ListEC2VPCs(ctx context.Context) ([]model.EC2VPC, error) {
	var vpcs []model.EC2VPC
	paginator := ec2.NewDescribeVpcsPaginator(c.EC2, &ec2.DescribeVpcsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, vpc := range page.Vpcs {
			vpcs = append(vpcs, vpcFromSDK(vpc))
		}
	}
	return vpcs, nil
}

// DescribeEC2VPC returns one VPC by ID.
func (c *Client) DescribeEC2VPC(ctx context.Context, vpcID string) (*model.EC2VPC, error) {
	out, err := c.EC2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{VpcIds: []string{vpcID}})
	if err != nil {
		return nil, err
	}
	if len(out.Vpcs) == 0 {
		return nil, fmt.Errorf("VPC %s not found", vpcID)
	}
	vpc := vpcFromSDK(out.Vpcs[0])
	return &vpc, nil
}

// ListEC2Subnets returns every subnet in the active region.
func (c *Client) ListEC2Subnets(ctx context.Context) ([]model.EC2Subnet, error) {
	var subnets []model.EC2Subnet
	paginator := ec2.NewDescribeSubnetsPaginator(c.EC2, &ec2.DescribeSubnetsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, subnet := range page.Subnets {
			subnets = append(subnets, subnetFromSDK(subnet))
		}
	}
	return subnets, nil
}

// DescribeEC2Subnet returns one subnet by ID.
func (c *Client) DescribeEC2Subnet(ctx context.Context, subnetID string) (*model.EC2Subnet, error) {
	out, err := c.EC2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{SubnetIds: []string{subnetID}})
	if err != nil {
		return nil, err
	}
	if len(out.Subnets) == 0 {
		return nil, fmt.Errorf("subnet %s not found", subnetID)
	}
	subnet := subnetFromSDK(out.Subnets[0])
	return &subnet, nil
}

func securityGroupFromSDK(group ec2types.SecurityGroup) model.EC2SecurityGroup {
	tags, name := ec2Tags(group.Tags)
	result := model.EC2SecurityGroup{
		GroupID:     derefStrAws(group.GroupId),
		Name:        derefStrAws(group.GroupName),
		Description: derefStrAws(group.Description),
		VpcID:       derefStrAws(group.VpcId),
		OwnerID:     derefStrAws(group.OwnerId),
		Tags:        tags,
	}
	if name != "" {
		result.Name = name
	}
	for _, permission := range group.IpPermissions {
		result.Rules = append(result.Rules, sgRulesFromPerm("inbound", permission)...)
	}
	for _, permission := range group.IpPermissionsEgress {
		result.Rules = append(result.Rules, sgRulesFromPerm("outbound", permission)...)
	}
	return result
}

func securityGroupRuleFromSDK(rule ec2types.SecurityGroupRule) model.EC2SGRule {
	direction := "inbound"
	if rule.IsEgress != nil && *rule.IsEgress {
		direction = "outbound"
	}
	protocol := derefStrAws(rule.IpProtocol)
	if protocol == "-1" {
		protocol = "All"
	}
	portRange := formatEC2PortRange(protocol, rule.FromPort, rule.ToPort)
	source := firstNonEmpty(
		derefStrAws(rule.CidrIpv4), derefStrAws(rule.CidrIpv6),
		derefStrAws(rule.PrefixListId),
	)
	if source == "" && rule.ReferencedGroupInfo != nil {
		source = derefStrAws(rule.ReferencedGroupInfo.GroupId)
	}
	if source == "" {
		source = "*"
	}
	return model.EC2SGRule{
		RuleID:      derefStrAws(rule.SecurityGroupRuleId),
		Direction:   direction,
		Protocol:    protocol,
		PortRange:   portRange,
		Source:      source,
		Description: derefStrAws(rule.Description),
	}
}

func vpcFromSDK(vpc ec2types.Vpc) model.EC2VPC {
	tags, name := ec2Tags(vpc.Tags)
	result := model.EC2VPC{
		VpcID:       derefStrAws(vpc.VpcId),
		Name:        name,
		State:       string(vpc.State),
		IsDefault:   vpc.IsDefault != nil && *vpc.IsDefault,
		Tenancy:     string(vpc.InstanceTenancy),
		OwnerID:     derefStrAws(vpc.OwnerId),
		DHCPOptions: derefStrAws(vpc.DhcpOptionsId),
		Tags:        tags,
	}
	for _, association := range vpc.CidrBlockAssociationSet {
		if cidr := derefStrAws(association.CidrBlock); cidr != "" {
			result.CIDRs = append(result.CIDRs, cidr)
		}
	}
	if len(result.CIDRs) == 0 && vpc.CidrBlock != nil {
		result.CIDRs = append(result.CIDRs, *vpc.CidrBlock)
	}
	for _, association := range vpc.Ipv6CidrBlockAssociationSet {
		if cidr := derefStrAws(association.Ipv6CidrBlock); cidr != "" {
			result.IPv6CIDRs = append(result.IPv6CIDRs, cidr)
		}
	}
	return result
}

func subnetFromSDK(subnet ec2types.Subnet) model.EC2Subnet {
	tags, name := ec2Tags(subnet.Tags)
	result := model.EC2Subnet{
		SubnetID:           derefStrAws(subnet.SubnetId),
		Name:               name,
		VpcID:              derefStrAws(subnet.VpcId),
		State:              string(subnet.State),
		AZ:                 derefStrAws(subnet.AvailabilityZone),
		AZID:               derefStrAws(subnet.AvailabilityZoneId),
		CIDR:               derefStrAws(subnet.CidrBlock),
		MapPublicIP:        subnet.MapPublicIpOnLaunch != nil && *subnet.MapPublicIpOnLaunch,
		DefaultForAZ:       subnet.DefaultForAz != nil && *subnet.DefaultForAz,
		AssignIPv6OnCreate: subnet.AssignIpv6AddressOnCreation != nil && *subnet.AssignIpv6AddressOnCreation,
		OwnerID:            derefStrAws(subnet.OwnerId),
		Tags:               tags,
	}
	if subnet.AvailableIpAddressCount != nil {
		result.AvailableIPs = *subnet.AvailableIpAddressCount
	}
	for _, association := range subnet.Ipv6CidrBlockAssociationSet {
		if cidr := derefStrAws(association.Ipv6CidrBlock); cidr != "" {
			result.IPv6CIDRs = append(result.IPv6CIDRs, cidr)
		}
	}
	return result
}

func ec2Tags(tags []ec2types.Tag) (map[string]string, string) {
	values := make(map[string]string, len(tags))
	name := ""
	for _, tag := range tags {
		key, value := derefStrAws(tag.Key), derefStrAws(tag.Value)
		if key == "" {
			continue
		}
		values[key] = value
		if key == "Name" {
			name = value
		}
	}
	return values, name
}

func formatEC2PortRange(protocol string, from, to *int32) string {
	if protocol == "All" || from == nil || to == nil {
		return "All"
	}
	if *from == 0 && *to == 0 {
		return "All"
	}
	if *from == *to {
		return fmt.Sprintf("%d", *from)
	}
	return fmt.Sprintf("%d-%d", *from, *to)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
