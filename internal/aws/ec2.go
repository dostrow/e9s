package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// EC2Instance represents an EC2 instance summary.
type EC2Instance struct {
	InstanceID    string
	Name          string
	State         string // running, stopped, pending, etc.
	Type          string // t3.micro, etc.
	AZ            string
	PrivateIP     string
	PublicIP      string
	VpcID         string
	SubnetID      string
	KeyName       string
	LaunchTime    time.Time
	Platform      string // linux, windows
	AMI           string
	IAMRole       string
	Tags          map[string]string
	SecurityGroups []EC2SecurityGroupRef
}

// EC2SecurityGroupRef is a minimal SG reference on an instance.
type EC2SecurityGroupRef struct {
	ID   string
	Name string
}

// EC2InstanceDetail holds extended instance information.
type EC2InstanceDetail struct {
	EC2Instance
	Architecture   string
	RootDeviceType string
	RootDeviceName string
	EBSOptimized   bool
	Monitoring     string
	Volumes        []EC2Volume
	SecurityGroupRules []EC2SGRule
	ConsoleOutput  string
}

// EC2Volume represents an attached EBS volume.
type EC2Volume struct {
	VolumeID   string
	DeviceName string
	Size       int32 // GiB
	VolumeType string
	State      string
}

// EC2SGRule represents a security group rule.
type EC2SGRule struct {
	Direction string // inbound or outbound
	Protocol  string
	PortRange string
	Source    string // CIDR or SG ID
}

// ListEC2Instances returns EC2 instances, optionally filtered by name substring.
func (c *Client) ListEC2Instances(ctx context.Context, filter string) ([]EC2Instance, error) {
	input := &ec2.DescribeInstancesInput{}

	var instances []EC2Instance
	lf := strings.ToLower(filter)

	paginator := ec2.NewDescribeInstancesPaginator(c.EC2, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, res := range page.Reservations {
			for _, inst := range res.Instances {
				i := instanceFromSDK(inst)
				if lf != "" && !strings.Contains(strings.ToLower(i.Name), lf) &&
					!strings.Contains(strings.ToLower(i.InstanceID), lf) {
					continue
				}
				instances = append(instances, i)
			}
		}
	}

	// Sort: running first, then by name
	sort.Slice(instances, func(i, j int) bool {
		if instances[i].State != instances[j].State {
			return stateOrder(instances[i].State) < stateOrder(instances[j].State)
		}
		return instances[i].Name < instances[j].Name
	})

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

// BuildSSMPluginArgs returns plugin args for an SSM session (different endpoint than ECS).
func (s *ExecSession) BuildSSMPluginArgs() ([]string, error) {
	sessionJSON, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return []string{
		string(sessionJSON),
		s.Region,
		"StartSession",
		"",
		fmt.Sprintf(`{"Target":"%s"}`, s.Target),
		fmt.Sprintf("https://ssm.%s.amazonaws.com", s.Region),
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

func stateOrder(state string) int {
	order := map[string]int{
		"running":       0,
		"pending":       1,
		"stopping":      2,
		"stopped":       3,
		"shutting-down": 4,
		"terminated":    5,
	}
	if o, ok := order[state]; ok {
		return o
	}
	return 9
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
