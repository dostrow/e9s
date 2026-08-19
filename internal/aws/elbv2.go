package aws

import (
	"context"
	"fmt"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/dostrow/e9s/internal/model"
)

// ListEC2LoadBalancers returns ALBs and NLBs. Gateway and Classic load
// balancers are intentionally outside the EC2 module's current scope.
func (c *Client) ListEC2LoadBalancers(ctx context.Context) ([]model.EC2LoadBalancer, error) {
	var loadBalancers []model.EC2LoadBalancer
	paginator := elasticloadbalancingv2.NewDescribeLoadBalancersPaginator(c.ELBV2,
		&elasticloadbalancingv2.DescribeLoadBalancersInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, loadBalancer := range page.LoadBalancers {
			if loadBalancer.Type != elbtypes.LoadBalancerTypeEnumApplication &&
				loadBalancer.Type != elbtypes.LoadBalancerTypeEnumNetwork {
				continue
			}
			loadBalancers = append(loadBalancers, loadBalancerFromSDK(loadBalancer))
		}
	}
	return loadBalancers, nil
}

// DescribeEC2LoadBalancer returns one ALB/NLB and its listeners, target groups,
// and current target health.
func (c *Client) DescribeEC2LoadBalancer(ctx context.Context, arn string) (*model.EC2LoadBalancer, error) {
	out, err := c.ELBV2.DescribeLoadBalancers(ctx, &elasticloadbalancingv2.DescribeLoadBalancersInput{
		LoadBalancerArns: []string{arn},
	})
	if err != nil {
		return nil, err
	}
	if len(out.LoadBalancers) == 0 {
		return nil, fmt.Errorf("load balancer %s not found", arn)
	}
	loadBalancer := loadBalancerFromSDK(out.LoadBalancers[0])
	if loadBalancer.Type != string(elbtypes.LoadBalancerTypeEnumApplication) &&
		loadBalancer.Type != string(elbtypes.LoadBalancerTypeEnumNetwork) {
		return nil, fmt.Errorf("load balancer %s has unsupported type %s", arn, loadBalancer.Type)
	}

	listeners, err := c.listEC2Listeners(ctx, arn)
	if err != nil {
		return nil, err
	}
	loadBalancer.Listeners = listeners

	targetGroups, err := c.listEC2TargetGroups(ctx, arn)
	if err != nil {
		return nil, err
	}
	for i := range targetGroups {
		targets, targetErr := c.listEC2TargetHealth(ctx, targetGroups[i].ARN)
		if targetErr != nil {
			return nil, targetErr
		}
		targetGroups[i].Targets = targets
	}
	loadBalancer.TargetGroups = targetGroups
	return &loadBalancer, nil
}

// ListEC2TargetGroups returns all ALB/NLB target groups in the active region.
func (c *Client) ListEC2TargetGroups(ctx context.Context) ([]model.EC2TargetGroup, error) {
	return c.listEC2TargetGroups(ctx, "")
}

// DescribeEC2TargetGroup composes one target group with current target health.
func (c *Client) DescribeEC2TargetGroup(ctx context.Context, arn string) (*model.EC2TargetGroup, error) {
	out, err := c.ELBV2.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{
		TargetGroupArns: []string{arn},
	})
	if err != nil {
		return nil, err
	}
	if len(out.TargetGroups) == 0 {
		return nil, fmt.Errorf("target group %s not found", arn)
	}
	targetGroup := targetGroupFromSDK(out.TargetGroups[0])
	targetGroup.Targets, err = c.listEC2TargetHealth(ctx, arn)
	if err != nil {
		return nil, err
	}
	return &targetGroup, nil
}

func (c *Client) listEC2Listeners(ctx context.Context, loadBalancerARN string) ([]model.EC2Listener, error) {
	input := &elasticloadbalancingv2.DescribeListenersInput{LoadBalancerArn: awssdk.String(loadBalancerARN)}
	paginator := elasticloadbalancingv2.NewDescribeListenersPaginator(c.ELBV2, input)
	var listeners []model.EC2Listener
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, listener := range page.Listeners {
			listeners = append(listeners, listenerFromSDK(listener))
		}
	}
	return listeners, nil
}

func (c *Client) listEC2TargetGroups(ctx context.Context, loadBalancerARN string) ([]model.EC2TargetGroup, error) {
	input := &elasticloadbalancingv2.DescribeTargetGroupsInput{}
	if loadBalancerARN != "" {
		input.LoadBalancerArn = awssdk.String(loadBalancerARN)
	}
	paginator := elasticloadbalancingv2.NewDescribeTargetGroupsPaginator(c.ELBV2, input)
	var targetGroups []model.EC2TargetGroup
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, targetGroup := range page.TargetGroups {
			targetGroups = append(targetGroups, targetGroupFromSDK(targetGroup))
		}
	}
	return targetGroups, nil
}

func (c *Client) listEC2TargetHealth(ctx context.Context, targetGroupARN string) ([]model.EC2TargetHealth, error) {
	out, err := c.ELBV2.DescribeTargetHealth(ctx, &elasticloadbalancingv2.DescribeTargetHealthInput{
		TargetGroupArn: awssdk.String(targetGroupARN),
	})
	if err != nil {
		return nil, err
	}
	targets := make([]model.EC2TargetHealth, 0, len(out.TargetHealthDescriptions))
	for _, target := range out.TargetHealthDescriptions {
		targets = append(targets, targetHealthFromSDK(target))
	}
	return targets, nil
}

func loadBalancerFromSDK(loadBalancer elbtypes.LoadBalancer) model.EC2LoadBalancer {
	result := model.EC2LoadBalancer{
		ARN:            derefStrAws(loadBalancer.LoadBalancerArn),
		Name:           derefStrAws(loadBalancer.LoadBalancerName),
		Type:           string(loadBalancer.Type),
		Scheme:         string(loadBalancer.Scheme),
		DNSName:        derefStrAws(loadBalancer.DNSName),
		HostedZoneID:   derefStrAws(loadBalancer.CanonicalHostedZoneId),
		VpcID:          derefStrAws(loadBalancer.VpcId),
		IPAddressType:  string(loadBalancer.IpAddressType),
		SecurityGroups: append([]string(nil), loadBalancer.SecurityGroups...),
	}
	if loadBalancer.CreatedTime != nil {
		result.CreatedAt = *loadBalancer.CreatedTime
	}
	if loadBalancer.State != nil {
		result.State = string(loadBalancer.State.Code)
		result.StateReason = derefStrAws(loadBalancer.State.Reason)
	}
	for _, zone := range loadBalancer.AvailabilityZones {
		item := model.EC2LoadBalancerZone{
			AZ:       derefStrAws(zone.ZoneName),
			SubnetID: derefStrAws(zone.SubnetId),
		}
		if len(zone.LoadBalancerAddresses) > 0 {
			item.IPv4 = firstNonEmpty(
				derefStrAws(zone.LoadBalancerAddresses[0].IpAddress),
				derefStrAws(zone.LoadBalancerAddresses[0].PrivateIPv4Address),
			)
			item.IPv6 = derefStrAws(zone.LoadBalancerAddresses[0].IPv6Address)
		}
		result.AvailabilityZones = append(result.AvailabilityZones, item)
	}
	return result
}

func listenerFromSDK(listener elbtypes.Listener) model.EC2Listener {
	result := model.EC2Listener{
		ARN:       derefStrAws(listener.ListenerArn),
		Protocol:  string(listener.Protocol),
		SSLPolicy: derefStrAws(listener.SslPolicy),
	}
	if listener.Port != nil {
		result.Port = *listener.Port
	}
	for _, certificate := range listener.Certificates {
		if arn := derefStrAws(certificate.CertificateArn); arn != "" {
			result.Certificates = append(result.Certificates, arn)
		}
	}
	for _, action := range listener.DefaultActions {
		result.Actions = append(result.Actions, loadBalancerActionSummary(action))
	}
	return result
}

func loadBalancerActionSummary(action elbtypes.Action) string {
	summary := string(action.Type)
	if targetGroup := derefStrAws(action.TargetGroupArn); targetGroup != "" {
		return summary + " → " + targetGroup
	}
	if action.ForwardConfig != nil && len(action.ForwardConfig.TargetGroups) > 0 {
		targets := make([]string, 0, len(action.ForwardConfig.TargetGroups))
		for _, target := range action.ForwardConfig.TargetGroups {
			if arn := derefStrAws(target.TargetGroupArn); arn != "" {
				targets = append(targets, arn)
			}
		}
		if len(targets) > 0 {
			return summary + " → " + strings.Join(targets, ", ")
		}
	}
	return summary
}

func targetGroupFromSDK(targetGroup elbtypes.TargetGroup) model.EC2TargetGroup {
	result := model.EC2TargetGroup{
		ARN:                 derefStrAws(targetGroup.TargetGroupArn),
		Name:                derefStrAws(targetGroup.TargetGroupName),
		Protocol:            string(targetGroup.Protocol),
		ProtocolVersion:     derefStrAws(targetGroup.ProtocolVersion),
		TargetType:          string(targetGroup.TargetType),
		LoadBalancerARNs:    append([]string(nil), targetGroup.LoadBalancerArns...),
		VpcID:               derefStrAws(targetGroup.VpcId),
		HealthCheckProtocol: string(targetGroup.HealthCheckProtocol),
		HealthCheckPort:     derefStrAws(targetGroup.HealthCheckPort),
		HealthCheckPath:     derefStrAws(targetGroup.HealthCheckPath),
	}
	if targetGroup.Port != nil {
		result.Port = *targetGroup.Port
	}
	return result
}

func targetHealthFromSDK(description elbtypes.TargetHealthDescription) model.EC2TargetHealth {
	result := model.EC2TargetHealth{}
	if description.Target != nil {
		result.ID = derefStrAws(description.Target.Id)
		result.AZ = derefStrAws(description.Target.AvailabilityZone)
		if description.Target.Port != nil {
			result.Port = *description.Target.Port
		}
	}
	if description.TargetHealth != nil {
		result.State = string(description.TargetHealth.State)
		result.Reason = string(description.TargetHealth.Reason)
		result.Description = derefStrAws(description.TargetHealth.Description)
	}
	return result
}
