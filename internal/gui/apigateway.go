//go:build gui

package gui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
)

func (w *mainWindow) openAPIGatewayModule(kind model.APIGatewayKind) {
	if kind == "" {
		kind = model.APIGatewayREST
	}
	w.resetWorkspaceForBrowserChange()
	w.resourceHistory = nil
	w.clearAPIGatewayBrowser()
	w.apiGatewayKind = kind
	w.currentPage = pageAPIGateway
	w.setBreadcrumb("API Gateway / " + apiGatewayKindTitle(kind))
	w.backButton.SetSensitive(false)
	w.search.SetText("")
	w.search.SetPlaceholderText("Filter " + strings.ToLower(apiGatewayKindTitle(kind)) + "…")
	w.resourceStack.SetVisibleChildName(pageAPIGateway)
	w.setDetail("Loading API Gateway "+strings.ToLower(apiGatewayKindTitle(kind))+"…", detailIntro)
	w.updateActionSensitivity()
	if w.options.APIGateway == nil {
		w.setDetail("API Gateway is unavailable because no service was configured.", detailError)
		w.setStatus("API Gateway service unavailable", true)
		return
	}
	ctx, generation := w.startRequest("Loading API Gateway " + strings.ToLower(apiGatewayKindTitle(kind)) + "…")
	go func() {
		resources, err := w.options.APIGateway.List(ctx, kind, "")
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageAPIGateway || w.apiGatewayKind != kind {
				return
			}
			w.allAPIGateways = resources
			w.applyAPIGatewayFilter()
			w.setDetail(apiGatewayListSummary(kind, len(resources)), detailIntro)
			w.setStatus(fmt.Sprintf("Loaded %d API Gateway %s", len(resources), strings.ToLower(apiGatewayKindTitle(kind))), false)
		})
	}()
}

func (w *mainWindow) refreshAPIGateway(foreground bool) {
	if w.options.APIGateway == nil {
		return
	}
	kind, selected := w.apiGatewayKind, w.selectedAPIGateway
	ctx, generation := w.startRefreshRequest("Refreshing API Gateway…", foreground)
	go func() {
		resources, err := w.options.APIGateway.List(ctx, kind, "")
		var detail *model.APIGatewayAPI
		if err == nil && selected != "" {
			detail, err = w.options.APIGateway.Detail(ctx, kind, selected)
		}
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			if w.currentPage != pageAPIGateway || w.apiGatewayKind != kind {
				return
			}
			w.allAPIGateways = resources
			w.applyAPIGatewayFilter()
			if selected == "" {
				w.setDetail(apiGatewayListSummary(kind, len(resources)), detailIntro)
				return
			}
			if detail == nil {
				w.selectedAPIGateway = ""
				w.apiGatewayDetail = nil
				w.setBreadcrumb("API Gateway / " + apiGatewayKindTitle(kind))
				w.setDetail("The selected API Gateway resource is no longer available.\n\n"+apiGatewayListSummary(kind, len(resources)), detailIntro)
				w.updateActionSensitivity()
				return
			}
			w.apiGatewayDetail = detail
			w.renderAPIGatewayDetail(*detail)
		})
	}()
}

func (w *mainWindow) clearAPIGatewayBrowser() {
	w.allAPIGateways = nil
	w.filteredAPIGateways = nil
	w.selectedAPIGateway = ""
	w.apiGatewayDetail = nil
	if w.apiGatewayTable != nil {
		w.apiGatewayTable.clear()
	}
}

func (w *mainWindow) applyAPIGatewayFilter() {
	w.filteredAPIGateways = service.FilterAPIGateways(w.allAPIGateways, w.search.Text())
	rows := make([]string, len(w.filteredAPIGateways))
	for index, resource := range w.filteredAPIGateways {
		rows[index] = fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s", resource.Name, resource.ID, valueOrDash(resource.Protocol),
			valueOrDash(resource.Status), valueOrDash(resource.Endpoint), formatTime(resource.Created))
	}
	w.apiGatewayTable.replace(rows)
}

func (w *mainWindow) selectAPIGatewayRow() {
	position := int(w.apiGatewayTable.selection.Selected())
	if position < 0 || position >= len(w.filteredAPIGateways) {
		return
	}
	resource := w.filteredAPIGateways[position]
	w.selectedAPIGateway = resource.ID
	w.apiGatewayDetail = nil
	w.setBreadcrumb("API Gateway / " + apiGatewayKindTitle(resource.Kind) + " / " + resource.Name)
	w.setDetail(formatAPIGatewaySummary(resource)+"\n\nLoading stages, routes, integrations, and mappings…", detailAPIGateway)
	w.updateActionSensitivity()
	if w.options.APIGateway == nil {
		return
	}
	kind, identifier := resource.Kind, resource.ID
	ctx, generation := w.startRequest("Loading API Gateway " + resource.Name + "…")
	go func() {
		detail, err := w.options.APIGateway.Detail(ctx, kind, identifier)
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageAPIGateway || w.apiGatewayKind != kind || w.selectedAPIGateway != identifier {
				return
			}
			w.apiGatewayDetail = detail
			w.renderAPIGatewayDetail(*detail)
			w.setStatus("Loaded API Gateway "+detail.Name, false)
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) openAPIGatewayAt(position uint) {
	if int(position) >= len(w.filteredAPIGateways) {
		return
	}
	w.apiGatewayTable.selection.SetSelected(position)
}

func (w *mainWindow) renderAPIGatewayDetail(resource model.APIGatewayAPI) {
	w.setDetail(formatAPIGatewayDetail(resource), detailAPIGateway)
}

func apiGatewayKindTitle(kind model.APIGatewayKind) string {
	switch kind {
	case model.APIGatewayHTTP:
		return "HTTP APIs"
	case model.APIGatewayWebSocket:
		return "WebSocket APIs"
	case model.APIGatewayDomain:
		return "Custom Domains"
	default:
		return "REST APIs"
	}
}

func apiGatewayListSummary(kind model.APIGatewayKind, count int) string {
	return fmt.Sprintf("API GATEWAY %s\n\n%d resources loaded. Select one to inspect its stages, routes, integrations, and deployment configuration.",
		strings.ToUpper(apiGatewayKindTitle(kind)), count)
}

func formatAPIGatewaySummary(resource model.APIGatewayAPI) string {
	return fmt.Sprintf("API GATEWAY %s\n\nName        %s\nIdentifier  %s\nProtocol    %s\nStatus      %s\nEndpoint    %s\nStages      %d\nRoutes      %d\nIntegrations %d",
		strings.ToUpper(strings.ReplaceAll(string(resource.Kind), "-", " ")), resource.Name, resource.ID,
		valueOrDash(resource.Protocol), valueOrDash(resource.Status), valueOrDash(resource.Endpoint), len(resource.Stages), len(resource.Routes), len(resource.Integrations))
}

func formatAPIGatewayDetail(resource model.APIGatewayAPI) string {
	var b strings.Builder
	b.WriteString(formatAPIGatewaySummary(resource))
	fmt.Fprintf(&b, "\nARN         %s\nDescription %s\nVersion     %s\nEndpoint types %s\nDefault endpoint disabled %t\n",
		valueOrDash(resource.ARN), valueOrDash(resource.Description), valueOrDash(resource.Version),
		valueOrDash(strings.Join(resource.EndpointTypes, ", ")), resource.DisableExecuteEndpoint)
	if resource.Kind == model.APIGatewayDomain {
		fmt.Fprintf(&b, "Certificate %s\nHosted zone %s\nTLS policy  %s\n", valueOrDash(resource.CertificateARN), valueOrDash(resource.HostedZoneID), valueOrDash(resource.SecurityPolicy))
	}
	if len(resource.Stages) > 0 {
		b.WriteString("\nSTAGES\n\n")
		for _, stage := range resource.Stages {
			fmt.Fprintf(&b, "%-20s deployment=%s auto=%t\n  invoke: %s\n", stage.Name, valueOrDash(stage.DeploymentID), stage.AutoDeploy, valueOrDash(stage.InvokeURL))
			if stage.LogGroupARN != "" {
				fmt.Fprintf(&b, "  access logs: %s\n", stage.LogGroupARN)
			}
		}
	}
	if len(resource.Routes) > 0 {
		b.WriteString("\nROUTES AND RESOURCES\n\n")
		for _, route := range resource.Routes {
			key := route.Key
			if key == "" {
				key = strings.TrimSpace(strings.Join(route.Methods, ", ") + " " + route.Path)
			}
			fmt.Fprintf(&b, "%-36s authorization=%s target=%s\n", key, valueOrDash(route.Authorization), valueOrDash(route.Target))
		}
	}
	if len(resource.Integrations) > 0 {
		b.WriteString("\nINTEGRATIONS\n\n")
		for _, integration := range resource.Integrations {
			fmt.Fprintf(&b, "%-18s %-10s %-8s %s\n", integration.ID, integration.Type, integration.Method, integration.URI)
			if integration.LambdaFunctionARN != "" {
				fmt.Fprintf(&b, "  Lambda: %s\n", integration.LambdaFunctionARN)
			}
		}
	}
	if len(resource.Mappings) > 0 {
		b.WriteString("\nAPI MAPPINGS\n\n")
		for _, mapping := range resource.Mappings {
			fmt.Fprintf(&b, "%-24s API=%s stage=%s\n", valueOrDash(mapping.MappingKey), mapping.APIID, mapping.Stage)
		}
	}
	if !resource.Created.IsZero() {
		fmt.Fprintf(&b, "\nCreated %s\n", formatTime(resource.Created))
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
