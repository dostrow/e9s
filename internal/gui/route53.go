//go:build gui

package gui

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
)

func (w *mainWindow) openRoute53Module() {
	if w.currentPage != pageRoute53Zones {
		w.loadRoute53Zones()
	}
}

func (w *mainWindow) loadRoute53Zones() {
	w.resetWorkspaceForBrowserChange()
	w.clearRoute53Zones()
	w.clearRoute53Records()
	w.currentPage = pageRoute53Zones
	w.search.SetPlaceholderText("Filter hosted zones…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageRoute53Zones)
	w.backButton.SetSensitive(false)
	w.setBreadcrumb("Route53 / Hosted zones")
	w.setDetail("Loading Route53 hosted zones…", detailIntro)
	w.updateActionSensitivity()
	if w.options.Route53 == nil {
		w.setDetail("Route53 is unavailable because no Route53 service was configured.", detailError)
		w.setStatus("Route53 service unavailable", true)
		return
	}
	ctx, generation := w.startRequest("Loading Route53 hosted zones…")
	go func() {
		zones, err := w.options.Route53.Zones(ctx, "")
		w.finishRequest(ctx, generation, err, func() {
			w.allRoute53Zones = zones
			w.applyRoute53ZoneFilter()
			w.setDetail(route53ZoneListSummary(len(zones)), detailIntro)
		})
	}()
}

func (w *mainWindow) refreshRoute53Zones(foreground bool) {
	if w.options.Route53 == nil || w.route53ActionPending {
		return
	}
	selected := w.selectedRoute53Zone
	ctx, generation := w.startRefreshRequest("Refreshing Route53 hosted zones…", foreground)
	go func() {
		zones, err := w.options.Route53.Zones(ctx, "")
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allRoute53Zones = zones
			w.applyRoute53ZoneFilter()
			if selected == "" {
				w.setDetail(route53ZoneListSummary(len(zones)), detailIntro)
				return
			}
			if index := findRoute53ZoneIndex(w.filteredRoute53Zones, selected); index >= 0 {
				w.route53ZoneTable.selection.SetSelected(uint(index))
				return
			}
			w.selectedRoute53Zone = ""
			w.setBreadcrumb("Route53 / Hosted zones")
			w.setDetail("The selected hosted zone is no longer available.", detailIntro)
		})
	}()
}

func (w *mainWindow) clearRoute53Zones() {
	w.allRoute53Zones = nil
	w.filteredRoute53Zones = nil
	w.selectedRoute53Zone = ""
	if w.route53ZoneTable != nil {
		w.route53ZoneTable.clear()
	}
}

func (w *mainWindow) clearRoute53Records() {
	w.allRoute53Records = nil
	w.filteredRoute53Records = nil
	w.selectedRoute53Record = ""
	w.route53ZoneContext = nil
	w.route53DNSAnswer = nil
	if w.route53RecordTable != nil {
		w.route53RecordTable.clear()
	}
}

func (w *mainWindow) applyRoute53ZoneFilter() {
	w.filteredRoute53Zones = filterRoute53Zones(w.allRoute53Zones, w.search.Text())
	rows := make([]string, 0, len(w.filteredRoute53Zones))
	for _, zone := range w.filteredRoute53Zones {
		zoneType := "Public"
		if zone.Private {
			zoneType = "Private"
		}
		rows = append(rows, fmt.Sprintf("%s\t%s\t%d\t%s", zone.Name, zoneType, zone.RecordCount, zone.Comment))
	}
	w.route53ZoneTable.replace(rows)
}

func filterRoute53Zones(zones []model.Route53Zone, query string) []model.Route53Zone {
	query = strings.ToLower(strings.TrimSpace(query))
	filtered := make([]model.Route53Zone, 0, len(zones))
	for _, zone := range zones {
		if query == "" || strings.Contains(strings.ToLower(zone.Name), query) || strings.Contains(strings.ToLower(zone.Comment), query) || strings.Contains(strings.ToLower(zone.ID), query) {
			filtered = append(filtered, zone)
		}
	}
	return filtered
}

func (w *mainWindow) selectRoute53ZoneRow() {
	if w.currentPage != pageRoute53Zones {
		return
	}
	position := w.route53ZoneTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredRoute53Zones) {
		w.selectedRoute53Zone = ""
		w.setBreadcrumb("Route53 / Hosted zones")
		w.setDetail(route53ZoneListSummary(len(w.allRoute53Zones)), detailIntro)
		w.updateActionSensitivity()
		return
	}
	zone := w.filteredRoute53Zones[position]
	w.selectedRoute53Zone = zone.ID
	w.setBreadcrumb("Route53 / Hosted zones / " + zone.Name)
	w.setDetail(formatRoute53Zone(zone), detailRoute53Zone)
	w.updateActionSensitivity()
}

func (w *mainWindow) openRoute53ZoneAt(position uint) {
	if int(position) >= len(w.filteredRoute53Zones) {
		return
	}
	zone := w.filteredRoute53Zones[position]
	w.route53ZoneTable.selection.SetSelected(position)
	w.loadRoute53Records(zone)
}

func (w *mainWindow) loadRoute53Records(zone model.Route53Zone) {
	if w.options.Route53 == nil || zone.ID == "" {
		return
	}
	w.resetWorkspaceForBrowserChange()
	w.clearRoute53Records()
	w.currentPage = pageRoute53Records
	w.route53ZoneContext = &zone
	w.selectedRoute53Zone = zone.ID
	w.search.SetPlaceholderText("Filter record sets…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageRoute53Records)
	w.backButton.SetSensitive(true)
	w.setBreadcrumb(route53RecordBreadcrumb(zone, ""))
	w.setDetail("Loading Route53 record sets…", detailIntro)
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Loading Route53 records for " + zone.Name + "…")
	go func() {
		records, err := w.options.Route53.Records(ctx, zone.ID)
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageRoute53Records || w.route53ZoneContext == nil || w.route53ZoneContext.ID != zone.ID {
				return
			}
			w.allRoute53Records = records
			w.applyRoute53RecordFilter()
			w.setDetail(route53RecordListSummary(zone, len(records)), detailRoute53Zone)
		})
	}()
}

func (w *mainWindow) restoreRoute53ZoneBrowser() {
	zoneID := w.selectedRoute53Zone
	w.resetWorkspaceForBrowserChange()
	w.clearRoute53Records()
	w.currentPage = pageRoute53Zones
	w.search.SetPlaceholderText("Filter hosted zones…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageRoute53Zones)
	w.backButton.SetSensitive(false)
	w.applyRoute53ZoneFilter()
	if index := findRoute53ZoneIndex(w.filteredRoute53Zones, zoneID); index >= 0 {
		zone := w.filteredRoute53Zones[index]
		w.selectedRoute53Zone = zone.ID
		w.route53ZoneTable.selection.SetSelected(uint(index))
		w.setBreadcrumb("Route53 / Hosted zones / " + zone.Name)
		w.setDetail(formatRoute53Zone(zone), detailRoute53Zone)
	} else {
		w.selectedRoute53Zone = ""
		w.setBreadcrumb("Route53 / Hosted zones")
		w.setDetail(route53ZoneListSummary(len(w.allRoute53Zones)), detailIntro)
	}
	w.updateActionSensitivity()
	w.setStatus("Ready", false)
}

func (w *mainWindow) applyRoute53RecordFilter() {
	w.filteredRoute53Records = filterRoute53Records(w.allRoute53Records, w.search.Text())
	rows := make([]string, 0, len(w.filteredRoute53Records))
	for _, record := range w.filteredRoute53Records {
		ttl := "—"
		if record.TTL > 0 {
			ttl = fmt.Sprintf("%d", record.TTL)
		}
		rows = append(rows, strings.Join([]string{record.Name, record.Type, ttl, route53RecordSummaryValue(record), route53RoutingLabel(record)}, "\t"))
	}
	w.route53RecordTable.replace(rows)
}

func filterRoute53Records(records []model.Route53Record, query string) []model.Route53Record {
	query = strings.ToLower(strings.TrimSpace(query))
	filtered := make([]model.Route53Record, 0, len(records))
	for _, record := range records {
		search := strings.Join([]string{record.Name, record.Type, strings.Join(record.Values, " "), record.AliasTarget, record.RoutingPolicy, record.SetIdentifier}, "\n")
		if query == "" || strings.Contains(strings.ToLower(search), query) {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func (w *mainWindow) selectRoute53RecordRow() {
	if w.currentPage != pageRoute53Records || w.route53ZoneContext == nil {
		return
	}
	position := w.route53RecordTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredRoute53Records) {
		w.selectedRoute53Record = ""
		w.route53DNSAnswer = nil
		w.setBreadcrumb(route53RecordBreadcrumb(*w.route53ZoneContext, ""))
		w.setDetail(route53RecordListSummary(*w.route53ZoneContext, len(w.allRoute53Records)), detailRoute53Zone)
		w.updateActionSensitivity()
		return
	}
	record := w.filteredRoute53Records[position]
	w.selectedRoute53Record = route53RecordIdentity(record)
	w.route53DNSAnswer = nil
	w.setBreadcrumb(route53RecordBreadcrumb(*w.route53ZoneContext, record.Name+" "+record.Type))
	w.setDetail(formatRoute53Record(*w.route53ZoneContext, record, nil), detailRoute53Record)
	w.updateActionSensitivity()
}

func (w *mainWindow) openRoute53RecordAt(position uint) {
	if int(position) < len(w.filteredRoute53Records) {
		w.route53RecordTable.selection.SetSelected(position)
	}
}

func (w *mainWindow) refreshRoute53Records(foreground bool) {
	if w.options.Route53 == nil || w.route53ZoneContext == nil || w.route53ActionPending {
		return
	}
	zone, selected := *w.route53ZoneContext, w.selectedRoute53Record
	ctx, generation := w.startRefreshRequest("Refreshing Route53 records…", foreground)
	go func() {
		records, err := w.options.Route53.Records(ctx, zone.ID)
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allRoute53Records = records
			w.applyRoute53RecordFilter()
			if selected != "" {
				if index := findRoute53RecordIndex(w.filteredRoute53Records, selected); index >= 0 {
					w.route53RecordTable.selection.SetSelected(uint(index))
					return
				}
			}
			w.selectedRoute53Record = ""
			w.route53DNSAnswer = nil
			w.setBreadcrumb(route53RecordBreadcrumb(zone, ""))
			w.setDetail(route53RecordListSummary(zone, len(records)), detailRoute53Zone)
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) testSelectedRoute53DNS() {
	record, found := w.selectedRoute53RecordValue()
	if !found || w.route53ZoneContext == nil || w.options.Route53 == nil || w.route53ActionPending {
		return
	}
	w.route53ActionPending = true
	w.updateActionSensitivity()
	zone := *w.route53ZoneContext
	ctx, generation := w.startRequest("Testing DNS answer for " + record.Name + "…")
	go func() {
		answer, err := w.options.Route53.TestDNS(ctx, zone.ID, record)
		w.finishRoute53Action(ctx, generation, err, "DNS test completed for "+record.Name, func() {
			w.route53DNSAnswer = answer
			w.setDetail(formatRoute53Record(zone, record, answer), detailRoute53Record)
		})
	}()
}

func (w *mainWindow) finishRoute53Action(ctx context.Context, generation uint64, err error, success string, apply func()) {
	glib.IdleAdd(func() {
		if ctx.Err() != nil || generation != w.generation {
			return
		}
		w.requestCancel = nil
		w.spinner.Stop()
		w.setWorkspaceBusy("", false)
		w.route53ActionPending = false
		if err != nil {
			w.updateActionSensitivity()
			w.setStatus(err.Error(), true)
			return
		}
		if apply != nil {
			apply()
		}
		w.updateActionSensitivity()
		w.setStatus(success, false)
	})
}

func (w *mainWindow) selectedRoute53RecordValue() (model.Route53Record, bool) {
	if w.selectedRoute53Record == "" {
		return model.Route53Record{}, false
	}
	for _, record := range w.allRoute53Records {
		if route53RecordIdentity(record) == w.selectedRoute53Record {
			return record, true
		}
	}
	return model.Route53Record{}, false
}

func route53RecordIdentity(record model.Route53Record) string {
	return record.Name + "\x00" + record.Type + "\x00" + record.SetIdentifier
}

func findRoute53RecordIndex(records []model.Route53Record, identity string) int {
	for index, record := range records {
		if route53RecordIdentity(record) == identity {
			return index
		}
	}
	return -1
}

func route53RecordSummaryValue(record model.Route53Record) string {
	if record.AliasTarget != "" {
		return "ALIAS → " + record.AliasTarget
	}
	if len(record.Values) == 0 {
		return "—"
	}
	if len(record.Values) == 1 {
		return record.Values[0]
	}
	return fmt.Sprintf("%s (+%d more)", record.Values[0], len(record.Values)-1)
}

func route53RoutingLabel(record model.Route53Record) string {
	if record.RoutingPolicy == "" || record.RoutingPolicy == "Simple" {
		return "—"
	}
	return record.RoutingPolicy
}

func route53RecordBreadcrumb(zone model.Route53Zone, record string) string {
	breadcrumb := "Route53 / Hosted zones / " + zone.Name + " / Records"
	if record != "" {
		breadcrumb += " / " + record
	}
	return breadcrumb
}

func route53RecordListSummary(zone model.Route53Zone, count int) string {
	return fmt.Sprintf("ROUTE53 RECORD SETS\n\nHosted zone  %s\nZone ID      %s\nRecord sets  %d\n\nSelect a record set for its complete values and routing configuration.", zone.Name, zone.ID, count)
}

func formatRoute53Record(zone model.Route53Zone, record model.Route53Record, answer *model.Route53DNSAnswer) string {
	lines := []string{
		"ROUTE53 RECORD SET", "", "Hosted zone    " + zone.Name, "Name           " + record.Name,
		"Type           " + record.Type, "Routing        " + route53RoutingLabel(record),
	}
	if record.TTL > 0 {
		lines = append(lines, fmt.Sprintf("TTL            %d seconds", record.TTL))
	}
	if record.SetIdentifier != "" {
		lines = append(lines, "Set identifier "+record.SetIdentifier)
	}
	if record.Weight > 0 {
		lines = append(lines, fmt.Sprintf("Weight         %d", record.Weight))
	}
	if record.Region != "" {
		lines = append(lines, "Region         "+record.Region)
	}
	if record.Failover != "" {
		lines = append(lines, "Failover       "+record.Failover)
	}
	if record.HealthCheckID != "" {
		lines = append(lines, "Health check   "+record.HealthCheckID)
	}
	if record.AliasTarget != "" {
		lines = append(lines, "", "ALIAS TARGET", "DNS name       "+record.AliasTarget, "Hosted zone ID "+record.AliasZoneID, fmt.Sprintf("Evaluate health %t", record.EvaluateTargetHealth))
	} else {
		lines = append(lines, "", fmt.Sprintf("VALUES (%d)", len(record.Values)))
		lines = append(lines, record.Values...)
	}
	if answer != nil {
		lines = append(lines, "", "DNS TEST RESULT", "Response       "+answer.ResponseCode, "Nameserver     "+answer.Nameserver, "Protocol       "+answer.Protocol)
		if len(answer.RecordData) > 0 {
			lines = append(lines, "Resolved values")
			lines = append(lines, answer.RecordData...)
		}
	}
	return strings.Join(lines, "\n")
}

func (w *mainWindow) promptCreateRoute53Record() {
	if w.currentPage != pageRoute53Records || w.route53ZoneContext == nil || w.options.Route53 == nil || w.route53ActionPending {
		return
	}
	w.promptRoute53RecordEditor("Create Route53 record", service.BuildRoute53RecordTemplate(nil), nil)
}

func (w *mainWindow) promptEditRoute53Record() {
	record, found := w.selectedRoute53RecordValue()
	if !found || w.route53ZoneContext == nil || w.options.Route53 == nil || w.route53ActionPending {
		return
	}
	w.promptRoute53RecordEditor("Edit Route53 record", service.BuildRoute53RecordTemplate(&record), &record)
}

func (w *mainWindow) promptRoute53RecordEditor(title, initial string, original *model.Route53Record) {
	dialog := gtk.NewDialogWithFlags(title, &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	dialog.SetDefaultSize(760, 620)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	guidance := "Edit the portable JSON record document. Use either values with a positive TTL or an alias target."
	if original != nil {
		guidance += " Name, type, and setIdentifier are immutable in this edit."
	}
	label := gtk.NewLabel(guidance)
	label.SetXAlign(0)
	label.SetWrap(true)
	content.Append(label)
	editor := newSourceEditor(sourceDocument{Path: "route53-record.json", Language: "json"})
	editor.ApplyPalette(semanticPaletteFromStyle(w.window.StyleContext()))
	editor.SetText(initial)
	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetHExpand(true)
	scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
	scroll.SetChild(editor.Widget())
	content.Append(scroll)
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.SetWrap(true)
	errorLabel.AddCSSClass("error")
	content.Append(errorLabel)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Review change…", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		record, err := service.ParseRoute53RecordTemplate(editor.Text())
		if err == nil && original != nil && route53RecordIdentity(*record) != route53RecordIdentity(*original) {
			err = fmt.Errorf("name, type, and setIdentifier cannot be changed by an edit; create a new record instead")
		}
		if err != nil {
			errorLabel.SetLabel(err.Error())
			return
		}
		dialog.Destroy()
		w.confirmRoute53RecordChange(*record, original != nil)
	})
	dialog.Present()
}

func (w *mainWindow) confirmRoute53RecordChange(record model.Route53Record, update bool) {
	if w.route53ZoneContext == nil {
		return
	}
	action := "Create"
	if update {
		action = "Update"
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsNone)
	dialog.SetTitle(action + " Route53 record")
	dialog.SetMarkup(action + " <b>" + html.EscapeString(record.Type) + " " + html.EscapeString(record.Name) + "</b> in <b>" + html.EscapeString(w.route53ZoneContext.Name) + "</b>?")
	dialog.SetObjectProperty("secondary-text", route53RecordChangeSummary(record))
	dialog.SetDestroyWithParent(true)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton(action, int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseOK) {
			w.runRoute53RecordChange(record, update)
		}
	})
	dialog.Present()
}

func route53RecordChangeSummary(record model.Route53Record) string {
	if record.AliasTarget != "" {
		return fmt.Sprintf("Alias: %s • Target zone: %s • Routing: %s", record.AliasTarget, record.AliasZoneID, route53RoutingLabel(record))
	}
	return fmt.Sprintf("TTL: %ds • Values: %d • Routing: %s", record.TTL, len(record.Values), route53RoutingLabel(record))
}

func (w *mainWindow) runRoute53RecordChange(record model.Route53Record, update bool) {
	if w.route53ZoneContext == nil || w.options.Route53 == nil || w.route53ActionPending {
		return
	}
	w.route53ActionPending = true
	w.updateActionSensitivity()
	zone := *w.route53ZoneContext
	action := "Creating"
	success := "Created"
	if update {
		action, success = "Updating", "Updated"
	}
	ctx, generation := w.startRequest(action + " Route53 record " + record.Name + "…")
	go func() {
		var err error
		if update {
			err = w.options.Route53.Update(ctx, zone.ID, record)
		} else {
			err = w.options.Route53.Create(ctx, zone.ID, record)
		}
		w.finishRoute53Action(ctx, generation, err, success+" Route53 record "+record.Name, func() {
			w.loadRoute53Records(zone)
		})
	}()
}

func (w *mainWindow) confirmDeleteRoute53Record() {
	record, found := w.selectedRoute53RecordValue()
	if !found || w.route53ZoneContext == nil || w.options.Route53 == nil || w.route53ActionPending {
		return
	}
	if record.Type == "NS" || record.Type == "SOA" {
		w.setStatus("Apex "+record.Type+" records are protected", true)
		return
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsNone)
	dialog.SetTitle("Delete Route53 record")
	dialog.SetMarkup("Permanently delete <b>" + html.EscapeString(record.Type) + " " + html.EscapeString(record.Name) + "</b>?")
	dialog.SetObjectProperty("secondary-text", "The complete loaded record set will be submitted for deletion. This cannot be undone.")
	dialog.SetDestroyWithParent(true)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Delete", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseOK) {
			w.runDeleteRoute53Record(record)
		}
	})
	dialog.Present()
}

func (w *mainWindow) runDeleteRoute53Record(record model.Route53Record) {
	if w.route53ZoneContext == nil || w.options.Route53 == nil || w.route53ActionPending {
		return
	}
	w.route53ActionPending = true
	w.updateActionSensitivity()
	zone := *w.route53ZoneContext
	ctx, generation := w.startRequest("Deleting Route53 record " + record.Name + "…")
	go func() {
		err := w.options.Route53.Delete(ctx, zone.ID, record)
		w.finishRoute53Action(ctx, generation, err, "Deleted Route53 record "+record.Name, func() {
			w.selectedRoute53Record = ""
			w.loadRoute53Records(zone)
		})
	}()
}

func route53ZoneListSummary(count int) string {
	return fmt.Sprintf("ROUTE53 HOSTED ZONES\n\nHosted zones  %d\n\nSelect a hosted zone for configuration; double-click it to browse its record sets.", count)
}

func formatRoute53Zone(zone model.Route53Zone) string {
	zoneType := "Public"
	if zone.Private {
		zoneType = "Private"
	}
	return fmt.Sprintf("ROUTE53 HOSTED ZONE\n\nName          %s\nZone ID       %s\nVisibility    %s\nRecord sets   %d\nComment       %s", zone.Name, zone.ID, zoneType, zone.RecordCount, valueOrDash(zone.Comment))
}

func findRoute53ZoneIndex(zones []model.Route53Zone, zoneID string) int {
	for index, zone := range zones {
		if zone.ID == zoneID {
			return index
		}
	}
	return -1
}
