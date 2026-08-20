package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigatewaytypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigatewayv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/dostrow/e9s/internal/model"
)

func (c *Client) ListAPIGateways(ctx context.Context, kind model.APIGatewayKind) ([]model.APIGatewayAPI, error) {
	switch kind {
	case model.APIGatewayREST:
		return c.listRESTAPIs(ctx)
	case model.APIGatewayHTTP, model.APIGatewayWebSocket:
		return c.listV2APIs(ctx, kind)
	case model.APIGatewayDomain:
		return c.listAPIGatewayDomains(ctx)
	default:
		return nil, fmt.Errorf("unsupported API Gateway resource kind %q", kind)
	}
}

func (c *Client) DescribeAPIGateway(ctx context.Context, kind model.APIGatewayKind, identifier string) (*model.APIGatewayAPI, error) {
	if identifier = strings.TrimSpace(identifier); identifier == "" {
		return nil, fmt.Errorf("API Gateway identifier is required")
	}
	switch kind {
	case model.APIGatewayREST:
		return c.describeRESTAPI(ctx, identifier)
	case model.APIGatewayHTTP, model.APIGatewayWebSocket:
		return c.describeV2API(ctx, kind, identifier)
	case model.APIGatewayDomain:
		return c.describeAPIGatewayDomain(ctx, identifier)
	default:
		return nil, fmt.Errorf("unsupported API Gateway resource kind %q", kind)
	}
}

func (c *Client) GetAPIGatewayMetrics(ctx context.Context, api model.APIGatewayAPI, window time.Duration) (*model.MetricSnapshot, error) {
	if api.Kind == model.APIGatewayDomain {
		return &model.MetricSnapshot{}, nil
	}
	if window <= 0 {
		window = 15 * time.Minute
	}
	end := time.Now()
	dimension := model.MetricDimension{Name: "ApiId", Value: api.ID}
	if api.Kind == model.APIGatewayREST {
		dimension = model.MetricDimension{Name: "ApiName", Value: api.Name}
	}
	query := func(id, label, name, stat, unit string) model.MetricQuery {
		return model.MetricQuery{ID: id, Label: label, Namespace: "AWS/ApiGateway", MetricName: name,
			Dimensions: []model.MetricDimension{dimension}, Statistic: stat, Unit: unit, Scale: 1}
	}
	return c.GetMetricSeries(ctx, model.MetricRequest{StartTime: end.Add(-window), EndTime: end, MaxPoints: defaultMetricMaxPoints,
		Queries: []model.MetricQuery{
			query("count", "Requests", "Count", "Sum", "count"), query("latency", "Latency", "Latency", "Average", "ms"),
			query("integration_latency", "Integration latency", "IntegrationLatency", "Average", "ms"),
			query("4xx", "4XX errors", "4XXError", "Sum", "count"), query("5xx", "5XX errors", "5XXError", "Sum", "count"),
		}})
}

func (c *Client) listRESTAPIs(ctx context.Context) ([]model.APIGatewayAPI, error) {
	if c.APIGateway == nil {
		return nil, fmt.Errorf("API Gateway client is unavailable")
	}
	paginator := apigateway.NewGetRestApisPaginator(c.APIGateway, &apigateway.GetRestApisInput{})
	var result []model.APIGatewayAPI
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, api := range page.Items {
			result = append(result, c.mapRESTAPI(api))
		}
	}
	return result, nil
}

func (c *Client) mapRESTAPI(api apigatewaytypes.RestApi) model.APIGatewayAPI {
	id := awssdk.ToString(api.Id)
	resource := model.APIGatewayAPI{Kind: model.APIGatewayREST, ID: id, Name: awssdk.ToString(api.Name), Description: awssdk.ToString(api.Description),
		Protocol: "REST", Version: awssdk.ToString(api.Version), Created: awssdk.ToTime(api.CreatedDate), Tags: cloneStringMap(api.Tags),
		DisableExecuteEndpoint: api.DisableExecuteApiEndpoint, Status: string(api.ApiStatus),
		Endpoint: fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com", id, c.region)}
	if api.EndpointConfiguration != nil {
		for _, endpointType := range api.EndpointConfiguration.Types {
			resource.EndpointTypes = append(resource.EndpointTypes, string(endpointType))
		}
	}
	return resource
}

func (c *Client) describeRESTAPI(ctx context.Context, id string) (*model.APIGatewayAPI, error) {
	out, err := c.APIGateway.GetRestApi(ctx, &apigateway.GetRestApiInput{RestApiId: awssdk.String(id)})
	if err != nil {
		return nil, err
	}
	resource := c.mapRESTAPI(apigatewaytypes.RestApi{Id: out.Id, Name: out.Name, Description: out.Description, Version: out.Version,
		CreatedDate: out.CreatedDate, DisableExecuteApiEndpoint: out.DisableExecuteApiEndpoint, EndpointConfiguration: out.EndpointConfiguration,
		Tags: out.Tags, ApiStatus: out.ApiStatus})
	stages, err := c.APIGateway.GetStages(ctx, &apigateway.GetStagesInput{RestApiId: awssdk.String(id)})
	if err != nil {
		return nil, err
	}
	for _, stage := range stages.Item {
		name := awssdk.ToString(stage.StageName)
		logARN := ""
		if stage.AccessLogSettings != nil {
			logARN = awssdk.ToString(stage.AccessLogSettings.DestinationArn)
		}
		resource.Stages = append(resource.Stages, model.APIGatewayStage{Name: name, DeploymentID: awssdk.ToString(stage.DeploymentId),
			Description: awssdk.ToString(stage.Description), InvokeURL: strings.TrimSuffix(resource.Endpoint, "/") + "/" + name,
			LogGroupARN: logARN, LastUpdatedAt: awssdk.ToTime(stage.LastUpdatedDate)})
	}
	paginator := apigateway.NewGetResourcesPaginator(c.APIGateway, &apigateway.GetResourcesInput{RestApiId: awssdk.String(id), Embed: []string{"methods"}})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, apiResource := range page.Items {
			route := model.APIGatewayRoute{ID: awssdk.ToString(apiResource.Id), Path: awssdk.ToString(apiResource.Path)}
			for methodName, method := range apiResource.ResourceMethods {
				route.Methods = append(route.Methods, methodName)
				if route.Authorization == "" {
					route.Authorization = awssdk.ToString(method.AuthorizationType)
				}
				if method.MethodIntegration != nil {
					integration := restIntegration(route.ID+":"+methodName, methodName, *method.MethodIntegration)
					resource.Integrations = append(resource.Integrations, integration)
				}
			}
			sort.Strings(route.Methods)
			resource.Routes = append(resource.Routes, route)
		}
	}
	return &resource, nil
}

func restIntegration(id, method string, integration apigatewaytypes.Integration) model.APIGatewayIntegration {
	uri := awssdk.ToString(integration.Uri)
	return model.APIGatewayIntegration{ID: id, Type: string(integration.Type), Method: method, URI: uri,
		ConnectionType: string(integration.ConnectionType), TimeoutMS: integration.TimeoutInMillis,
		CredentialsARN: awssdk.ToString(integration.Credentials), LambdaFunctionARN: lambdaARNFromIntegrationURI(uri)}
}

func (c *Client) listV2APIs(ctx context.Context, kind model.APIGatewayKind) ([]model.APIGatewayAPI, error) {
	if c.APIGatewayV2 == nil {
		return nil, fmt.Errorf("API Gateway v2 client is unavailable")
	}
	var result []model.APIGatewayAPI
	var nextToken *string
	for {
		page, err := c.APIGatewayV2.GetApis(ctx, &apigatewayv2.GetApisInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		for _, api := range page.Items {
			mapped := mapV2API(api)
			if mapped.Kind == kind {
				result = append(result, mapped)
			}
		}
		if page.NextToken == nil || awssdk.ToString(page.NextToken) == "" {
			break
		}
		nextToken = page.NextToken
	}
	return result, nil
}

func mapV2API(api apigatewayv2types.Api) model.APIGatewayAPI {
	kind := model.APIGatewayHTTP
	if api.ProtocolType == apigatewayv2types.ProtocolTypeWebsocket {
		kind = model.APIGatewayWebSocket
	}
	return model.APIGatewayAPI{Kind: kind, ID: awssdk.ToString(api.ApiId), Name: awssdk.ToString(api.Name),
		Description: awssdk.ToString(api.Description), Protocol: string(api.ProtocolType), Version: awssdk.ToString(api.Version),
		Endpoint: awssdk.ToString(api.ApiEndpoint), DisableExecuteEndpoint: awssdk.ToBool(api.DisableExecuteApiEndpoint),
		Created: awssdk.ToTime(api.CreatedDate), Tags: cloneStringMap(api.Tags)}
}

func (c *Client) describeV2API(ctx context.Context, kind model.APIGatewayKind, id string) (*model.APIGatewayAPI, error) {
	out, err := c.APIGatewayV2.GetApi(ctx, &apigatewayv2.GetApiInput{ApiId: awssdk.String(id)})
	if err != nil {
		return nil, err
	}
	resource := mapV2API(apigatewayv2types.Api{ApiId: out.ApiId, Name: out.Name, Description: out.Description, ProtocolType: out.ProtocolType,
		Version: out.Version, ApiEndpoint: out.ApiEndpoint, DisableExecuteApiEndpoint: out.DisableExecuteApiEndpoint, CreatedDate: out.CreatedDate, Tags: out.Tags})
	if resource.Kind != kind {
		return nil, fmt.Errorf("API %q has protocol %s", id, resource.Protocol)
	}
	var nextToken *string
	for {
		page, err := c.APIGatewayV2.GetStages(ctx, &apigatewayv2.GetStagesInput{ApiId: awssdk.String(id), NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		for _, stage := range page.Items {
			name := awssdk.ToString(stage.StageName)
			logARN := ""
			if stage.AccessLogSettings != nil {
				logARN = awssdk.ToString(stage.AccessLogSettings.DestinationArn)
			}
			resource.Stages = append(resource.Stages, model.APIGatewayStage{Name: name, DeploymentID: awssdk.ToString(stage.DeploymentId),
				AutoDeploy: awssdk.ToBool(stage.AutoDeploy), Description: awssdk.ToString(stage.Description), LogGroupARN: logARN,
				InvokeURL: strings.TrimSuffix(resource.Endpoint, "/") + stagePath(name), LastUpdatedAt: awssdk.ToTime(stage.LastUpdatedDate)})
		}
		if page.NextToken == nil || awssdk.ToString(page.NextToken) == "" {
			break
		}
		nextToken = page.NextToken
	}
	nextToken = nil
	for {
		page, err := c.APIGatewayV2.GetRoutes(ctx, &apigatewayv2.GetRoutesInput{ApiId: awssdk.String(id), NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		for _, route := range page.Items {
			resource.Routes = append(resource.Routes, model.APIGatewayRoute{ID: awssdk.ToString(route.RouteId), Key: awssdk.ToString(route.RouteKey),
				Authorization: string(route.AuthorizationType), Target: awssdk.ToString(route.Target)})
		}
		if page.NextToken == nil || awssdk.ToString(page.NextToken) == "" {
			break
		}
		nextToken = page.NextToken
	}
	nextToken = nil
	for {
		page, err := c.APIGatewayV2.GetIntegrations(ctx, &apigatewayv2.GetIntegrationsInput{ApiId: awssdk.String(id), NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		for _, integration := range page.Items {
			uri := awssdk.ToString(integration.IntegrationUri)
			resource.Integrations = append(resource.Integrations, model.APIGatewayIntegration{ID: awssdk.ToString(integration.IntegrationId),
				Type: string(integration.IntegrationType), Method: awssdk.ToString(integration.IntegrationMethod), URI: uri,
				ConnectionType: string(integration.ConnectionType), PayloadVersion: awssdk.ToString(integration.PayloadFormatVersion),
				TimeoutMS: awssdk.ToInt32(integration.TimeoutInMillis), CredentialsARN: awssdk.ToString(integration.CredentialsArn),
				LambdaFunctionARN: lambdaARNFromIntegrationURI(uri)})
		}
		if page.NextToken == nil || awssdk.ToString(page.NextToken) == "" {
			break
		}
		nextToken = page.NextToken
	}
	return &resource, nil
}

func (c *Client) listAPIGatewayDomains(ctx context.Context) ([]model.APIGatewayAPI, error) {
	seen := map[string]struct{}{}
	var result []model.APIGatewayAPI
	var nextToken *string
	for {
		page, err := c.APIGatewayV2.GetDomainNames(ctx, &apigatewayv2.GetDomainNamesInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		for _, domain := range page.Items {
			mapped := mapV2Domain(domain)
			seen[mapped.ID] = struct{}{}
			result = append(result, mapped)
		}
		if page.NextToken == nil || awssdk.ToString(page.NextToken) == "" {
			break
		}
		nextToken = page.NextToken
	}
	restPaginator := apigateway.NewGetDomainNamesPaginator(c.APIGateway, &apigateway.GetDomainNamesInput{})
	for restPaginator.HasMorePages() {
		page, err := restPaginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, domain := range page.Items {
			mapped := mapRESTDomain(domain)
			if _, exists := seen[mapped.ID]; !exists {
				result = append(result, mapped)
			}
		}
	}
	return result, nil
}

func mapV2Domain(domain apigatewayv2types.DomainName) model.APIGatewayAPI {
	resource := model.APIGatewayAPI{Kind: model.APIGatewayDomain, ID: awssdk.ToString(domain.DomainName), Name: awssdk.ToString(domain.DomainName),
		ARN: awssdk.ToString(domain.DomainNameArn), Tags: cloneStringMap(domain.Tags)}
	if len(domain.DomainNameConfigurations) > 0 {
		configuration := domain.DomainNameConfigurations[0]
		resource.Endpoint = awssdk.ToString(configuration.ApiGatewayDomainName)
		resource.EndpointTypes = []string{string(configuration.EndpointType)}
		resource.CertificateARN = awssdk.ToString(configuration.CertificateArn)
		resource.HostedZoneID = awssdk.ToString(configuration.HostedZoneId)
		resource.SecurityPolicy = string(configuration.SecurityPolicy)
		resource.Status = string(configuration.DomainNameStatus)
	}
	return resource
}

func mapRESTDomain(domain apigatewaytypes.DomainName) model.APIGatewayAPI {
	resource := model.APIGatewayAPI{Kind: model.APIGatewayDomain, ID: awssdk.ToString(domain.DomainName), Name: awssdk.ToString(domain.DomainName),
		ARN: awssdk.ToString(domain.DomainNameArn), Endpoint: awssdk.ToString(domain.RegionalDomainName),
		CertificateARN: awssdk.ToString(domain.RegionalCertificateArn), HostedZoneID: awssdk.ToString(domain.RegionalHostedZoneId),
		SecurityPolicy: string(domain.SecurityPolicy), Status: string(domain.DomainNameStatus), Tags: cloneStringMap(domain.Tags)}
	if domain.EndpointConfiguration != nil {
		for _, endpointType := range domain.EndpointConfiguration.Types {
			resource.EndpointTypes = append(resource.EndpointTypes, string(endpointType))
		}
	}
	if resource.Endpoint == "" {
		resource.Endpoint = awssdk.ToString(domain.DistributionDomainName)
		resource.CertificateARN = awssdk.ToString(domain.CertificateArn)
		resource.HostedZoneID = awssdk.ToString(domain.DistributionHostedZoneId)
	}
	return resource
}

func (c *Client) describeAPIGatewayDomain(ctx context.Context, name string) (*model.APIGatewayAPI, error) {
	v2, v2Err := c.APIGatewayV2.GetDomainName(ctx, &apigatewayv2.GetDomainNameInput{DomainName: awssdk.String(name)})
	if v2Err == nil {
		resource := mapV2Domain(apigatewayv2types.DomainName{DomainName: v2.DomainName, DomainNameArn: v2.DomainNameArn,
			DomainNameConfigurations: v2.DomainNameConfigurations, Tags: v2.Tags})
		var nextToken *string
		for {
			page, err := c.APIGatewayV2.GetApiMappings(ctx, &apigatewayv2.GetApiMappingsInput{DomainName: awssdk.String(name), NextToken: nextToken})
			if err != nil {
				return nil, err
			}
			for _, mapping := range page.Items {
				resource.Mappings = append(resource.Mappings, model.APIGatewayMapping{APIID: awssdk.ToString(mapping.ApiId), Stage: awssdk.ToString(mapping.Stage), MappingKey: awssdk.ToString(mapping.ApiMappingKey)})
			}
			if page.NextToken == nil || awssdk.ToString(page.NextToken) == "" {
				break
			}
			nextToken = page.NextToken
		}
		return &resource, nil
	}
	rest, err := c.APIGateway.GetDomainName(ctx, &apigateway.GetDomainNameInput{DomainName: awssdk.String(name)})
	if err != nil {
		return nil, fmt.Errorf("API Gateway v2: %v; REST API Gateway: %w", v2Err, err)
	}
	resource := mapRESTDomain(apigatewaytypes.DomainName{DomainName: rest.DomainName, DomainNameArn: rest.DomainNameArn,
		RegionalDomainName: rest.RegionalDomainName, RegionalCertificateArn: rest.RegionalCertificateArn,
		RegionalHostedZoneId: rest.RegionalHostedZoneId, DistributionDomainName: rest.DistributionDomainName,
		DistributionHostedZoneId: rest.DistributionHostedZoneId, CertificateArn: rest.CertificateArn,
		EndpointConfiguration: rest.EndpointConfiguration, DomainNameStatus: rest.DomainNameStatus, SecurityPolicy: rest.SecurityPolicy, Tags: rest.Tags})
	mappingPaginator := apigateway.NewGetBasePathMappingsPaginator(c.APIGateway, &apigateway.GetBasePathMappingsInput{DomainName: awssdk.String(name)})
	for mappingPaginator.HasMorePages() {
		page, err := mappingPaginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, mapping := range page.Items {
			resource.Mappings = append(resource.Mappings, model.APIGatewayMapping{APIID: awssdk.ToString(mapping.RestApiId), Stage: awssdk.ToString(mapping.Stage), MappingKey: awssdk.ToString(mapping.BasePath)})
		}
	}
	return &resource, nil
}

func lambdaARNFromIntegrationURI(uri string) string {
	marker := "functions/"
	start := strings.Index(uri, marker)
	if start < 0 {
		return ""
	}
	value := uri[start+len(marker):]
	if end := strings.Index(value, "/invocations"); end >= 0 {
		value = value[:end]
	}
	if strings.HasPrefix(value, "arn:") {
		return value
	}
	return ""
}

func stagePath(name string) string {
	if name == "" || name == "$default" {
		return ""
	}
	return "/" + name
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	copy := make(map[string]string, len(values))
	for key, value := range values {
		copy[key] = value
	}
	return copy
}
