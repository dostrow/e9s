//go:build gui

package gui

import (
	"fmt"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
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
	if int(position) < len(w.filteredRoute53Zones) {
		w.route53ZoneTable.selection.SetSelected(position)
	}
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
