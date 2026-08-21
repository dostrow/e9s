package service

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/model"
)

// EC2API is the low-level EC2 and Session Manager behavior used by shared
// workflows. It deliberately exposes domain values instead of SDK types.
type EC2API interface {
	ListEC2Instances(context.Context, string) ([]model.EC2Instance, error)
	DescribeEC2Instance(context.Context, string) (*model.EC2InstanceDetail, error)
	GetConsoleOutput(context.Context, string) (string, error)
	StartEC2Instance(context.Context, string) error
	StopEC2Instance(context.Context, string) error
	RebootEC2Instance(context.Context, string) error
	TerminateEC2Instance(context.Context, string) error
	StartSSMSession(context.Context, string) (*model.ExecSession, error)
	GetEC2Metrics(context.Context, string, time.Duration) (*model.MetricSnapshot, error)
}

func (s *EC2) Metrics(ctx context.Context, instanceID string, window time.Duration) (*model.MetricSnapshot, error) {
	instanceID, err := requireEC2InstanceID("read metrics for", instanceID)
	if err != nil {
		return nil, err
	}
	if window <= 0 {
		return nil, fmt.Errorf("read metrics for EC2 instance %q: time range must be positive", instanceID)
	}
	metrics, err := s.api.GetEC2Metrics(ctx, instanceID, window)
	if err != nil {
		return nil, fmt.Errorf("read metrics for EC2 instance %q: %w", instanceID, err)
	}
	return metrics, nil
}

// EC2 exposes instance workflows without frontend dependencies.
type EC2 struct {
	api        EC2API
	pluginPath func() (string, error)
}

func NewEC2(api EC2API) *EC2 {
	return &EC2{api: api, pluginPath: aws.SessionManagerPluginPath}
}

func (s *EC2) List(ctx context.Context, filter string) ([]model.EC2Instance, error) {
	instances, err := s.api.ListEC2Instances(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("list EC2 instances: %w", err)
	}
	instances = FilterEC2Instances(instances, filter)
	SortEC2Instances(instances)
	return instances, nil
}

// FilterEC2Instances applies the common case-insensitive instance filter to an
// already loaded result set. Frontends use it without issuing another AWS call.
func FilterEC2Instances(instances []model.EC2Instance, filter string) []model.EC2Instance {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return append([]model.EC2Instance(nil), instances...)
	}
	filtered := make([]model.EC2Instance, 0, len(instances))
	for _, instance := range instances {
		if ec2InstanceMatches(instance, filter) {
			filtered = append(filtered, instance)
		}
	}
	return filtered
}

// SortEC2Instances orders active states first, then names and instance IDs.
func SortEC2Instances(instances []model.EC2Instance) {
	sort.SliceStable(instances, func(i, j int) bool {
		left, right := instances[i], instances[j]
		if ec2StateOrder(left.State) != ec2StateOrder(right.State) {
			return ec2StateOrder(left.State) < ec2StateOrder(right.State)
		}
		if !strings.EqualFold(left.Name, right.Name) {
			return strings.ToLower(left.Name) < strings.ToLower(right.Name)
		}
		return left.InstanceID < right.InstanceID
	})
}

func (s *EC2) Detail(ctx context.Context, instanceID string) (*model.EC2InstanceDetail, error) {
	instanceID, err := requireEC2InstanceID("read", instanceID)
	if err != nil {
		return nil, err
	}
	detail, err := s.api.DescribeEC2Instance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("read EC2 instance %q: %w", instanceID, err)
	}
	if detail == nil {
		return nil, fmt.Errorf("read EC2 instance %q: instance was not found", instanceID)
	}
	sort.SliceStable(detail.SecurityGroups, func(i, j int) bool {
		return strings.ToLower(detail.SecurityGroups[i].Name) < strings.ToLower(detail.SecurityGroups[j].Name)
	})
	sort.SliceStable(detail.Volumes, func(i, j int) bool {
		return detail.Volumes[i].DeviceName < detail.Volumes[j].DeviceName
	})
	sort.SliceStable(detail.SecurityGroupRules, func(i, j int) bool {
		left, right := detail.SecurityGroupRules[i], detail.SecurityGroupRules[j]
		return strings.Join([]string{left.Direction, left.Protocol, left.PortRange, left.Source}, "\x00") <
			strings.Join([]string{right.Direction, right.Protocol, right.PortRange, right.Source}, "\x00")
	})
	return detail, nil
}

func (s *EC2) ConsoleOutput(ctx context.Context, instanceID string) (string, error) {
	instanceID, err := requireEC2InstanceID("read console output for", instanceID)
	if err != nil {
		return "", err
	}
	output, err := s.api.GetConsoleOutput(ctx, instanceID)
	if err != nil {
		return "", fmt.Errorf("read console output for EC2 instance %q: %w", instanceID, err)
	}
	return output, nil
}

func (s *EC2) PrepareSession(ctx context.Context, instanceID, state string) (model.ExecLaunch, error) {
	instanceID, err := requireEC2InstanceID("start Session Manager session for", instanceID)
	if err != nil {
		return model.ExecLaunch{}, err
	}
	if normalizedEC2State(state) != "running" {
		return model.ExecLaunch{}, fmt.Errorf("start Session Manager session for EC2 instance %q: instance is %s, not running", instanceID, valueOrUnknown(state))
	}
	plugin, err := s.pluginPath()
	if err != nil {
		return model.ExecLaunch{}, err
	}
	session, err := s.api.StartSSMSession(ctx, instanceID)
	if err != nil {
		return model.ExecLaunch{}, fmt.Errorf("start Session Manager session for EC2 instance %q: %w", instanceID, err)
	}
	if session == nil {
		return model.ExecLaunch{}, fmt.Errorf("start Session Manager session for EC2 instance %q: empty session", instanceID)
	}
	args, err := session.BuildSSMPluginArgs()
	if err != nil {
		return model.ExecLaunch{}, fmt.Errorf("build Session Manager plugin arguments: %w", err)
	}
	return model.ExecLaunch{Executable: plugin, Args: args}, nil
}

func (s *EC2) Start(ctx context.Context, instanceID, state string) error {
	return s.mutate(ctx, "start", instanceID, state, []string{"stopped"}, s.api.StartEC2Instance)
}

func (s *EC2) Stop(ctx context.Context, instanceID, state string) error {
	return s.mutate(ctx, "stop", instanceID, state, []string{"running"}, s.api.StopEC2Instance)
}

func (s *EC2) Reboot(ctx context.Context, instanceID, state string) error {
	return s.mutate(ctx, "reboot", instanceID, state, []string{"running"}, s.api.RebootEC2Instance)
}

func (s *EC2) Terminate(ctx context.Context, instanceID, state string) error {
	return s.mutate(ctx, "terminate", instanceID, state,
		[]string{"pending", "running", "stopping", "stopped"}, s.api.TerminateEC2Instance)
}

func (s *EC2) mutate(ctx context.Context, action, instanceID, state string, allowed []string, call func(context.Context, string) error) error {
	instanceID, err := requireEC2InstanceID(action, instanceID)
	if err != nil {
		return err
	}
	normalized := normalizedEC2State(state)
	if !slices.Contains(allowed, normalized) {
		return fmt.Errorf("%s EC2 instance %q: instance is %s", action, instanceID, valueOrUnknown(state))
	}
	if err := call(ctx, instanceID); err != nil {
		return fmt.Errorf("%s EC2 instance %q: %w", action, instanceID, err)
	}
	return nil
}

func ec2InstanceMatches(instance model.EC2Instance, filter string) bool {
	values := []string{instance.Name, instance.InstanceID, instance.State, instance.Type, instance.AZ,
		instance.PrivateIP, instance.PublicIP, instance.VpcID, instance.SubnetID, instance.AMI, instance.IAMRole}
	for key, value := range instance.Tags {
		values = append(values, key, value)
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), filter) {
			return true
		}
	}
	return false
}

func requireEC2InstanceID(action, instanceID string) (string, error) {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return "", fmt.Errorf("%s EC2 instance: instance ID is required", action)
	}
	return instanceID, nil
}

func normalizedEC2State(state string) string {
	return strings.ToLower(strings.TrimSpace(state))
}

func ec2StateOrder(state string) int {
	switch normalizedEC2State(state) {
	case "running":
		return 0
	case "pending":
		return 1
	case "stopping":
		return 2
	case "stopped":
		return 3
	case "shutting-down":
		return 4
	case "terminated":
		return 5
	default:
		return 9
	}
}
