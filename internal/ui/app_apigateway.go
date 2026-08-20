package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/views"
)

func (a App) openAPIGateway(kind model.APIGatewayKind) (App, tea.Cmd) {
	if kind == "" {
		kind = model.APIGatewayREST
	}
	a.mode = modeAPIGateway
	a.state = viewAPIGateway
	a.apiGatewayKind = kind
	a.selectedAPIGateway = nil
	a.apiGatewayView = views.NewEC2ResourceList(apiGatewayTUITitle(kind), []string{"NAME", "ID", "PROTOCOL", "STATUS", "ENDPOINT", "CREATED"}).SetSize(a.width-3, a.height-6)
	a.loading = true
	return a, a.loadAPIGateway(kind)
}

func (a App) loadAPIGateway(kind model.APIGatewayKind) tea.Cmd {
	apiService, ctx := a.apiGateway, a.ctx
	return func() tea.Msg {
		resources, err := apiService.List(ctx, kind, "")
		if err != nil {
			return errMsg{err}
		}
		return apiGatewayLoadedMsg{kind: kind, resources: resources}
	}
}

func (a App) openAPIGatewayDetail() (App, tea.Cmd) {
	id := a.apiGatewayView.SelectedID()
	if id == "" {
		return a, nil
	}
	kind, apiService, ctx := a.apiGatewayKind, a.apiGateway, a.ctx
	a.loading = true
	return a, func() tea.Msg {
		resource, err := apiService.Detail(ctx, kind, id)
		if err != nil {
			return errMsg{err}
		}
		var metrics *model.MetricSnapshot
		if kind != model.APIGatewayDomain {
			metrics, err = apiService.Metrics(ctx, *resource, 15*time.Minute)
			if err != nil {
				return errMsg{err}
			}
		}
		return apiGatewayDetailLoadedMsg{resource: resource, metrics: metrics}
	}
}

func apiGatewayTUITitle(kind model.APIGatewayKind) string {
	switch kind {
	case model.APIGatewayHTTP:
		return "API Gateway HTTP APIs"
	case model.APIGatewayWebSocket:
		return "API Gateway WebSocket APIs"
	case model.APIGatewayDomain:
		return "API Gateway Custom Domains"
	default:
		return "API Gateway REST APIs"
	}
}

func apiGatewayTUIRows(resources []model.APIGatewayAPI) []views.EC2ResourceRow {
	rows := make([]views.EC2ResourceRow, 0, len(resources))
	for _, resource := range resources {
		created := ""
		if !resource.Created.IsZero() {
			created = resource.Created.Format("2006-01-02 15:04")
		}
		rows = append(rows, views.EC2ResourceRow{ID: resource.ID, Search: resource.Name + " " + resource.Description, Cells: []string{
			resource.Name, resource.ID, resource.Protocol, resource.Status, resource.Endpoint, created,
		}})
	}
	return rows
}

func formatTUIAPIGateway(resource model.APIGatewayAPI, snapshot *model.MetricSnapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "API GATEWAY %s\n\nName                %s\nIdentifier          %s\nARN                 %s\nProtocol            %s\nStatus              %s\nEndpoint            %s\nEndpoint types      %s\nDefault disabled    %t\nDescription         %s\n",
		strings.ToUpper(strings.ReplaceAll(string(resource.Kind), "-", " ")), resource.Name, resource.ID, resource.ARN,
		resource.Protocol, resource.Status, resource.Endpoint, strings.Join(resource.EndpointTypes, ", "), resource.DisableExecuteEndpoint, resource.Description)
	if len(resource.Stages) > 0 {
		b.WriteString("\nSTAGES\n\n")
		for _, stage := range resource.Stages {
			fmt.Fprintf(&b, "%-20s deployment=%s auto=%t\n  %s\n", stage.Name, stage.DeploymentID, stage.AutoDeploy, stage.InvokeURL)
			if stage.LogGroupARN != "" {
				fmt.Fprintf(&b, "  logs: %s\n", stage.LogGroupARN)
			}
		}
	}
	if len(resource.Routes) > 0 {
		b.WriteString("\nROUTES AND RESOURCES\n\n")
		for _, route := range resource.Routes {
			key := route.Key
			if key == "" {
				key = strings.TrimSpace(strings.Join(route.Methods, ",") + " " + route.Path)
			}
			fmt.Fprintf(&b, "%-36s auth=%s target=%s\n", key, route.Authorization, route.Target)
		}
	}
	if len(resource.Integrations) > 0 {
		b.WriteString("\nINTEGRATIONS\n\n")
		for _, integration := range resource.Integrations {
			fmt.Fprintf(&b, "%-18s %-10s %-8s %s\n", integration.ID, integration.Type, integration.Method, integration.URI)
		}
	}
	if len(resource.Mappings) > 0 {
		b.WriteString("\nAPI MAPPINGS\n\n")
		for _, mapping := range resource.Mappings {
			fmt.Fprintf(&b, "%-24s API=%s stage=%s\n", mapping.MappingKey, mapping.APIID, mapping.Stage)
		}
	}
	if snapshot != nil && len(snapshot.Series) > 0 {
		b.WriteString("\nLATEST CLOUDWATCH METRICS (15 MINUTES)\n\n")
		for _, series := range snapshot.Series {
			if len(series.Points) == 0 {
				continue
			}
			point := series.Points[len(series.Points)-1]
			fmt.Fprintf(&b, "%-24s %.2f %s\n", series.Label, point.Value, series.Unit)
		}
	}
	if len(resource.Tags) > 0 {
		b.WriteString("\nTAGS\n\n")
		keys := make([]string, 0, len(resource.Tags))
		for key := range resource.Tags {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "%s  %s\n", key, resource.Tags[key])
		}
	}
	return strings.TrimSpace(b.String())
}
