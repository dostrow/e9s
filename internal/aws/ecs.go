package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
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

	// DescribeTasks accepts max 100 at a time
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
		for _, t := range desc.Tasks {
			tasks = append(tasks, model.TransformTask(t))
		}
	}
	return tasks, nil
}

func (c *Client) DescribeTask(ctx context.Context, cluster, taskARN string) (*model.Task, error) {
	desc, err := c.ECS.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &cluster,
		Tasks:   []string{taskARN},
	})
	if err != nil {
		return nil, err
	}
	if len(desc.Tasks) == 0 {
		return nil, nil
	}
	t := model.TransformTask(desc.Tasks[0])
	return &t, nil
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

func (c *Client) StopTask(ctx context.Context, cluster, taskARN, reason string) error {
	_, err := c.ECS.StopTask(ctx, &ecs.StopTaskInput{
		Cluster: &cluster,
		Task:    &taskARN,
		Reason:  aws.String(reason),
	})
	return err
}
