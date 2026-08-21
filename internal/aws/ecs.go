package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	aastypes "github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/dostrow/e9s/internal/model"
)

func (c *Client) ListClusters(ctx context.Context) ([]model.Cluster, error) {
	var clusterARNs []string
	paginator := ecs.NewListClustersPaginator(c.ECS, &ecs.ListClustersInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		clusterARNs = append(clusterARNs, page.ClusterArns...)
	}

	if len(clusterARNs) == 0 {
		return nil, nil
	}

	desc, err := c.ECS.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: clusterARNs,
	})
	if err != nil {
		return nil, err
	}

	clusters := make([]model.Cluster, 0, len(desc.Clusters))
	for _, cl := range desc.Clusters {
		clusters = append(clusters, model.TransformCluster(cl))
	}
	return clusters, nil
}

func (c *Client) ListServices(ctx context.Context, clusterARN string) ([]model.Service, error) {
	var serviceARNs []string
	paginator := ecs.NewListServicesPaginator(c.ECS, &ecs.ListServicesInput{
		Cluster: &clusterARN,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		serviceARNs = append(serviceARNs, page.ServiceArns...)
	}

	if len(serviceARNs) == 0 {
		return nil, nil
	}

	// DescribeServices accepts max 10 at a time
	var services []model.Service
	for i := 0; i < len(serviceARNs); i += 10 {
		end := min(i+10, len(serviceARNs))
		desc, err := c.ECS.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  &clusterARN,
			Services: serviceARNs[i:end],
		})
		if err != nil {
			return nil, err
		}
		for _, s := range desc.Services {
			services = append(services, model.TransformService(s))
		}
	}
	return services, nil
}

func (c *Client) ListTasks(ctx context.Context, clusterARN, serviceName string) ([]model.Task, error) {
	input := &ecs.ListTasksInput{
		Cluster: &clusterARN,
	}
	if serviceName != "" {
		input.ServiceName = &serviceName
	}

	var taskARNs []string
	paginator := ecs.NewListTasksPaginator(c.ECS, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		taskARNs = append(taskARNs, page.TaskArns...)
	}

	if len(taskARNs) == 0 {
		return nil, nil
	}
	return c.describeTasks(ctx, clusterARN, taskARNs)
}

// ListStoppedTasksPage returns one ECS page of stopped tasks. Callers retain
// the opaque continuation token to fetch another page on demand.
func (c *Client) ListStoppedTasksPage(ctx context.Context, clusterARN, serviceName, nextToken string, maxResults int) (model.TaskPage, error) {
	input := stoppedTasksInput(clusterARN, serviceName, nextToken, maxResults)
	page, err := c.ECS.ListTasks(ctx, input)
	if err != nil {
		return model.TaskPage{}, err
	}
	tasks, err := c.describeTasks(ctx, clusterARN, page.TaskArns)
	if err != nil {
		return model.TaskPage{}, err
	}
	return model.TaskPage{Tasks: tasks, NextToken: derefStrAws(page.NextToken)}, nil
}

func stoppedTasksInput(clusterARN, serviceName, nextToken string, maxResults int) *ecs.ListTasksInput {
	maxResults = min(max(maxResults, 1), 100)
	limit := int32(maxResults)
	input := &ecs.ListTasksInput{
		Cluster:       &clusterARN,
		DesiredStatus: ecstypes.DesiredStatusStopped,
		MaxResults:    &limit,
	}
	if serviceName != "" {
		input.ServiceName = &serviceName
	}
	if nextToken != "" {
		input.NextToken = &nextToken
	}
	return input
}

func (c *Client) describeTasks(ctx context.Context, clusterARN string, taskARNs []string) ([]model.Task, error) {
	if len(taskARNs) == 0 {
		return nil, nil
	}
	var tasks []model.Task
	for i := 0; i < len(taskARNs); i += 100 {
		end := min(i+100, len(taskARNs))
		desc, err := c.ECS.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: &clusterARN,
			Tasks:   taskARNs[i:end],
		})
		if err != nil {
			return nil, err
		}
		for _, task := range desc.Tasks {
			tasks = append(tasks, model.TransformTask(task))
		}
	}
	c.enrichTaskEC2Resources(ctx, clusterARN, tasks)
	return tasks, nil
}

func (c *Client) DescribeTask(ctx context.Context, cluster, taskARN string) (*model.Task, error) {
	tasks, err := c.describeTasks(ctx, cluster, []string{taskARN})
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, nil
	}
	return &tasks[0], nil
}

func (c *Client) enrichTaskEC2Resources(ctx context.Context, clusterARN string, tasks []model.Task) {
	networkInterfaceIDs := uniqueTaskValues(tasks, func(task model.Task) string { return task.NetworkInterfaceID })
	if len(networkInterfaceIDs) > 0 {
		interfaces, err := c.describeTaskNetworkInterfaces(ctx, networkInterfaceIDs)
		if err != nil {
			appendTaskResourceWarning(tasks, func(task model.Task) bool { return task.NetworkInterfaceID != "" }, "EC2 network details unavailable: "+err.Error())
		} else {
			applyTaskNetworkInterfaces(tasks, interfaces)
		}
	}
	subnetIDs := uniqueTaskValues(tasks, func(task model.Task) string {
		if task.VpcID == "" {
			return task.SubnetID
		}
		return ""
	})
	if len(subnetIDs) > 0 {
		out, err := c.EC2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{SubnetIds: subnetIDs})
		if err != nil {
			appendTaskResourceWarning(tasks, func(task model.Task) bool {
				return task.VpcID == "" && task.SubnetID != ""
			}, "EC2 subnet details unavailable: "+err.Error())
		} else {
			vpcBySubnet := make(map[string]string, len(out.Subnets))
			for _, subnet := range out.Subnets {
				vpcBySubnet[derefStrAws(subnet.SubnetId)] = derefStrAws(subnet.VpcId)
			}
			for i := range tasks {
				if tasks[i].VpcID == "" {
					tasks[i].VpcID = vpcBySubnet[tasks[i].SubnetID]
				}
			}
		}
	}
	containerInstanceARNs := uniqueTaskValues(tasks, func(task model.Task) string { return task.ContainerInstanceARN })
	if len(containerInstanceARNs) > 0 {
		instances, err := c.ECS.DescribeContainerInstances(ctx, &ecs.DescribeContainerInstancesInput{
			Cluster: &clusterARN, ContainerInstances: containerInstanceARNs,
		})
		if err != nil {
			appendTaskResourceWarning(tasks, func(task model.Task) bool { return task.ContainerInstanceARN != "" }, "ECS container instance details unavailable: "+err.Error())
		} else {
			instanceIDs := make(map[string]string, len(instances.ContainerInstances))
			for _, instance := range instances.ContainerInstances {
				instanceIDs[derefStrAws(instance.ContainerInstanceArn)] = derefStrAws(instance.Ec2InstanceId)
			}
			for i := range tasks {
				tasks[i].EC2InstanceID = instanceIDs[tasks[i].ContainerInstanceARN]
			}
		}
	}
}

func (c *Client) describeTaskNetworkInterfaces(ctx context.Context, ids []string) ([]ec2types.NetworkInterface, error) {
	var interfaces []ec2types.NetworkInterface
	for start := 0; start < len(ids); start += 100 {
		end := min(start+100, len(ids))
		out, err := c.EC2.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{NetworkInterfaceIds: ids[start:end]})
		if err != nil {
			return nil, err
		}
		interfaces = append(interfaces, out.NetworkInterfaces...)
	}
	return interfaces, nil
}

func applyTaskNetworkInterfaces(tasks []model.Task, interfaces []ec2types.NetworkInterface) {
	byID := make(map[string]ec2types.NetworkInterface, len(interfaces))
	for _, networkInterface := range interfaces {
		byID[derefStrAws(networkInterface.NetworkInterfaceId)] = networkInterface
	}
	for i := range tasks {
		networkInterface, found := byID[tasks[i].NetworkInterfaceID]
		if !found {
			continue
		}
		tasks[i].VpcID = derefStrAws(networkInterface.VpcId)
		if subnetID := derefStrAws(networkInterface.SubnetId); subnetID != "" {
			tasks[i].SubnetID = subnetID
		}
		if privateIP := derefStrAws(networkInterface.PrivateIpAddress); privateIP != "" {
			tasks[i].PrivateIP = privateIP
		}
		tasks[i].SecurityGroups = make([]model.EC2SecurityGroupRef, 0, len(networkInterface.Groups))
		for _, group := range networkInterface.Groups {
			tasks[i].SecurityGroups = append(tasks[i].SecurityGroups, model.EC2SecurityGroupRef{ID: derefStrAws(group.GroupId), Name: derefStrAws(group.GroupName)})
		}
	}
}

func uniqueTaskValues(tasks []model.Task, value func(model.Task) string) []string {
	seen := make(map[string]bool, len(tasks))
	values := make([]string, 0, len(tasks))
	for _, task := range tasks {
		item := value(task)
		if item != "" && !seen[item] {
			seen[item] = true
			values = append(values, item)
		}
	}
	return values
}

func appendTaskResourceWarning(tasks []model.Task, applies func(model.Task) bool, warning string) {
	for i := range tasks {
		if applies(tasks[i]) {
			tasks[i].ResourceWarnings = append(tasks[i].ResourceWarnings, warning)
		}
	}
}

func (c *Client) ForceNewDeployment(ctx context.Context, cluster, service string) error {
	_, err := c.ECS.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:            &cluster,
		Service:            &service,
		ForceNewDeployment: true,
	})
	return err
}

func (c *Client) ScaleService(ctx context.Context, cluster, service string, desiredCount int) error {
	count := int32(desiredCount)
	_, err := c.ECS.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      &cluster,
		Service:      &service,
		DesiredCount: &count,
	})
	return err
}

// ScaleInSuspended checks if scale-in is currently suspended for a service.
func (c *Client) ScaleInSuspended(ctx context.Context, cluster, service string) (bool, error) {
	target, err := c.scalableTarget(ctx, cluster, service)
	if err != nil {
		return false, err
	}
	return target.SuspendedState != nil && target.SuspendedState.DynamicScalingInSuspended != nil && *target.SuspendedState.DynamicScalingInSuspended, nil
}

func (c *Client) scalableTarget(ctx context.Context, cluster, service string) (*aastypes.ScalableTarget, error) {
	resourceID := fmt.Sprintf("service/%s/%s", cluster, service)
	out, err := c.AppAutoScaling.DescribeScalableTargets(ctx, &applicationautoscaling.DescribeScalableTargetsInput{
		ServiceNamespace:  aastypes.ServiceNamespaceEcs,
		ResourceIds:       []string{resourceID},
		ScalableDimension: aastypes.ScalableDimensionECSServiceDesiredCount,
	})
	if err != nil {
		return nil, err
	}
	if len(out.ScalableTargets) == 0 {
		return nil, fmt.Errorf("no scalable target configured for %s", resourceID)
	}
	return &out.ScalableTargets[0], nil
}

// SetScaleInSuspended enables or disables scale-in suspension for a service.
func (c *Client) SetScaleInSuspended(ctx context.Context, cluster, service string, suspended bool) error {
	resourceID := fmt.Sprintf("service/%s/%s", cluster, service)
	target, err := c.scalableTarget(ctx, cluster, service)
	if err != nil {
		return err
	}
	_, err = c.AppAutoScaling.RegisterScalableTarget(ctx, &applicationautoscaling.RegisterScalableTargetInput{
		ServiceNamespace:  aastypes.ServiceNamespaceEcs,
		ResourceId:        &resourceID,
		ScalableDimension: aastypes.ScalableDimensionECSServiceDesiredCount,
		SuspendedState:    scaleInSuspendedState(target.SuspendedState, suspended),
	})
	return err
}

func scaleInSuspendedState(current *aastypes.SuspendedState, suspended bool) *aastypes.SuspendedState {
	state := &aastypes.SuspendedState{}
	if current != nil {
		*state = *current
	}
	state.DynamicScalingInSuspended = &suspended
	return state
}

func (c *Client) StopTask(ctx context.Context, cluster, taskARN, reason string) error {
	_, err := c.ECS.StopTask(ctx, &ecs.StopTaskInput{
		Cluster: &cluster,
		Task:    &taskARN,
		Reason:  aws.String(reason),
	})
	return err
}

func (c *Client) RunTask(ctx context.Context, request model.RunTaskRequest) ([]model.Task, error) {
	out, err := c.ECS.RunTask(ctx, buildRunTaskInput(request))
	if err != nil {
		return nil, err
	}
	if len(out.Failures) > 0 {
		parts := make([]string, 0, len(out.Failures))
		for _, failure := range out.Failures {
			reason := derefStrAws(failure.Reason)
			if failure.Detail != nil && *failure.Detail != "" {
				reason += ": " + *failure.Detail
			}
			parts = append(parts, reason)
		}
		return nil, fmt.Errorf("ECS rejected task: %s", strings.Join(parts, "; "))
	}
	tasks := make([]model.Task, 0, len(out.Tasks))
	for _, task := range out.Tasks {
		tasks = append(tasks, model.TransformTask(task))
	}
	return tasks, nil
}

func buildRunTaskInput(request model.RunTaskRequest) *ecs.RunTaskInput {
	count := int32(request.Count)
	input := &ecs.RunTaskInput{
		Cluster:              aws.String(request.Cluster),
		TaskDefinition:       aws.String(request.TaskDefinition),
		Count:                &count,
		EnableExecuteCommand: request.EnableExecuteCommand,
		StartedBy:            aws.String("e9s"),
	}
	if request.LaunchType != "" {
		input.LaunchType = ecstypes.LaunchType(request.LaunchType)
	}
	if request.Group != "" {
		input.Group = aws.String(request.Group)
	}
	if len(request.Subnets) > 0 {
		assignPublicIP := ecstypes.AssignPublicIpDisabled
		if request.AssignPublicIP {
			assignPublicIP = ecstypes.AssignPublicIpEnabled
		}
		input.NetworkConfiguration = &ecstypes.NetworkConfiguration{
			AwsvpcConfiguration: &ecstypes.AwsVpcConfiguration{
				Subnets:        append([]string(nil), request.Subnets...),
				SecurityGroups: append([]string(nil), request.SecurityGroups...),
				AssignPublicIp: assignPublicIP,
			},
		}
	}
	return input
}
