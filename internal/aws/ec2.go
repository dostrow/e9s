package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/dostrow/e9s/internal/model"
)

type EC2Instance = model.EC2Instance
type EC2SecurityGroupRef = model.EC2SecurityGroupRef
type EC2InstanceDetail = model.EC2InstanceDetail
type EC2Volume = model.EC2Volume
type EC2SGRule = model.EC2SGRule

// ListEC2Instances returns EC2 instances, optionally filtered by name substring.
func (c *Client) ListEC2Instances(ctx context.Context, _ string) ([]EC2Instance, error) {
	input := &ec2.DescribeInstancesInput{}

	var instances []EC2Instance

	paginator := ec2.NewDescribeInstancesPaginator(c.EC2, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, res := range page.Reservations {
			for _, inst := range res.Instances {
				instances = append(instances, instanceFromSDK(inst))
			}
		}
	}

	return instances, nil
}

// DescribeEC2Instance fetches full detail for a single instance.
func (c *Client) DescribeEC2Instance(ctx context.Context, instanceID string) (*EC2InstanceDetail, error) {
	out, err := c.EC2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return nil, err
	}
	if len(out.Reservations) == 0 || len(out.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}

	inst := out.Reservations[0].Instances[0]
	detail := &EC2InstanceDetail{
		EC2Instance: instanceFromSDK(inst),
	}
	detail.Architecture = string(inst.Architecture)
	detail.RootDeviceType = string(inst.RootDeviceType)
	detail.RootDeviceName = derefStrAws(inst.RootDeviceName)
	if inst.EbsOptimized != nil {
		detail.EBSOptimized = *inst.EbsOptimized
	}
	if inst.Monitoring != nil {
		detail.Monitoring = string(inst.Monitoring.State)
	}

	// Fetch volumes
	for _, bdm := range inst.BlockDeviceMappings {
		vol := EC2Volume{
			DeviceName: derefStrAws(bdm.DeviceName),
		}
		if bdm.Ebs != nil {
			vol.VolumeID = derefStrAws(bdm.Ebs.VolumeId)
			vol.State = string(bdm.Ebs.Status)
		}
		detail.Volumes = append(detail.Volumes, vol)
	}

	// Enrich volumes with size/type
	if len(detail.Volumes) > 0 {
		var volIDs []string
		for _, v := range detail.Volumes {
			if v.VolumeID != "" {
				volIDs = append(volIDs, v.VolumeID)
			}
		}
		if len(volIDs) > 0 {
			volOut, err := c.EC2.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
				VolumeIds: volIDs,
			})
			if err == nil {
				volMap := make(map[string]ec2types.Volume)
				for _, v := range volOut.Volumes {
					if v.VolumeId != nil {
						volMap[*v.VolumeId] = v
					}
				}
				for i, v := range detail.Volumes {
					if vol, ok := volMap[v.VolumeID]; ok {
						if vol.Size != nil {
							detail.Volumes[i].Size = *vol.Size
						}
						detail.Volumes[i].VolumeType = string(vol.VolumeType)
					}
				}
			}
		}
	}

	// Fetch security group rules
	var sgIDs []string
	for _, sg := range detail.SecurityGroups {
		sgIDs = append(sgIDs, sg.ID)
	}
	if len(sgIDs) > 0 {
		sgOut, err := c.EC2.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
			GroupIds: sgIDs,
		})
		if err == nil {
			for _, sg := range sgOut.SecurityGroups {
				for _, rule := range sg.IpPermissions {
					detail.SecurityGroupRules = append(detail.SecurityGroupRules, sgRulesFromPerm("inbound", rule)...)
				}
				for _, rule := range sg.IpPermissionsEgress {
					detail.SecurityGroupRules = append(detail.SecurityGroupRules, sgRulesFromPerm("outbound", rule)...)
				}
			}
		}
	}

	return detail, nil
}

// GetConsoleOutput fetches the instance's serial console output.
func (c *Client) GetConsoleOutput(ctx context.Context, instanceID string) (string, error) {
	out, err := c.EC2.GetConsoleOutput(ctx, &ec2.GetConsoleOutputInput{
		InstanceId: &instanceID,
	})
	if err != nil {
		return "", err
	}
	if out.Output == nil {
		return "(no console output available)", nil
	}
	return *out.Output, nil
}

// StartEC2Instance starts a stopped instance.
func (c *Client) StartEC2Instance(ctx context.Context, instanceID string) error {
	_, err := c.EC2.StartInstances(ctx, &ec2.StartInstancesInput{
		InstanceIds: []string{instanceID},
	})
	return err
}

// StopEC2Instance stops a running instance.
func (c *Client) StopEC2Instance(ctx context.Context, instanceID string) error {
	_, err := c.EC2.StopInstances(ctx, &ec2.StopInstancesInput{
		InstanceIds: []string{instanceID},
	})
	return err
}

// RebootEC2Instance reboots a running instance.
func (c *Client) RebootEC2Instance(ctx context.Context, instanceID string) error {
	_, err := c.EC2.RebootInstances(ctx, &ec2.RebootInstancesInput{
		InstanceIds: []string{instanceID},
	})
	return err
}

// TerminateEC2Instance terminates an instance.
func (c *Client) TerminateEC2Instance(ctx context.Context, instanceID string) error {
	_, err := c.EC2.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	})
	return err
}

// StartSSMSession initiates an SSM session to an EC2 instance and returns
// the session info needed for session-manager-plugin.
func (c *Client) StartSSMSession(ctx context.Context, instanceID string) (*ExecSession, error) {
	out, err := c.SSM.StartSession(ctx, &ssm.StartSessionInput{
		Target: &instanceID,
	})
	if err != nil {
		return nil, fmt.Errorf("start-session failed: %w", err)
	}

	return &ExecSession{
		SessionID:  derefStrAws(out.SessionId),
		StreamURL:  derefStrAws(out.StreamUrl),
		TokenValue: derefStrAws(out.TokenValue),
		Region:     c.Region(),
		Target:     instanceID,
	}, nil
}

func instanceFromSDK(inst ec2types.Instance) EC2Instance {
	i := EC2Instance{
		InstanceID: derefStrAws(inst.InstanceId),
		Type:       string(inst.InstanceType),
		PrivateIP:  derefStrAws(inst.PrivateIpAddress),
		PublicIP:   derefStrAws(inst.PublicIpAddress),
		VpcID:      derefStrAws(inst.VpcId),
		SubnetID:   derefStrAws(inst.SubnetId),
		KeyName:    derefStrAws(inst.KeyName),
		AMI:        derefStrAws(inst.ImageId),
		Platform:   derefStrAws(inst.PlatformDetails),
		Tags:       make(map[string]string),
	}
	if inst.State != nil {
		i.State = string(inst.State.Name)
	}
	if inst.Placement != nil {
		i.AZ = derefStrAws(inst.Placement.AvailabilityZone)
	}
	if inst.LaunchTime != nil {
		i.LaunchTime = *inst.LaunchTime
	}
	if inst.IamInstanceProfile != nil {
		arn := derefStrAws(inst.IamInstanceProfile.Arn)
		// Extract role name from ARN
		if parts := strings.Split(arn, "/"); len(parts) > 1 {
			i.IAMRole = parts[len(parts)-1]
		} else {
			i.IAMRole = arn
		}
	}
	for _, t := range inst.Tags {
		if t.Key != nil && t.Value != nil {
			i.Tags[*t.Key] = *t.Value
			if *t.Key == "Name" {
				i.Name = *t.Value
			}
		}
	}
	for _, sg := range inst.SecurityGroups {
		i.SecurityGroups = append(i.SecurityGroups, EC2SecurityGroupRef{
			ID:   derefStrAws(sg.GroupId),
			Name: derefStrAws(sg.GroupName),
		})
	}
	return i
}

func sgRulesFromPerm(direction string, perm ec2types.IpPermission) []EC2SGRule {
	proto := derefStrAws(perm.IpProtocol)
	if proto == "-1" {
		proto = "All"
	}

	portRange := "All"
	if perm.FromPort != nil && perm.ToPort != nil {
		if *perm.FromPort == *perm.ToPort {
			portRange = fmt.Sprintf("%d", *perm.FromPort)
		} else {
			portRange = fmt.Sprintf("%d-%d", *perm.FromPort, *perm.ToPort)
		}
		if *perm.FromPort == 0 && *perm.ToPort == 0 && proto != "All" {
			portRange = "All"
		}
	}

	var rules []EC2SGRule
	for _, cidr := range perm.IpRanges {
		rules = append(rules, EC2SGRule{
			Direction: direction,
			Protocol:  proto,
			PortRange: portRange,
			Source:    derefStrAws(cidr.CidrIp),
		})
	}
	for _, cidr := range perm.Ipv6Ranges {
		rules = append(rules, EC2SGRule{
			Direction: direction,
			Protocol:  proto,
			PortRange: portRange,
			Source:    derefStrAws(cidr.CidrIpv6),
		})
	}
	for _, sg := range perm.UserIdGroupPairs {
		rules = append(rules, EC2SGRule{
			Direction: direction,
			Protocol:  proto,
			PortRange: portRange,
			Source:    derefStrAws(sg.GroupId),
		})
	}
	if len(rules) == 0 {
		rules = append(rules, EC2SGRule{
			Direction: direction,
			Protocol:  proto,
			PortRange: portRange,
			Source:    "*",
		})
	}
	return rules
}
