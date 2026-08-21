//go:build gui

package gui

import (
	"fmt"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/refreshpolicy"
)

func (w *mainWindow) noteAWSActivity() {
	if w.settingsOpen || w.showingTerminal || w.terminalDockHasFocus() {
		return
	}
	w.lastAWSActivity.Store(time.Now().UnixNano())
}

func (w *mainWindow) terminalDockHasFocus() bool {
	for _, session := range w.terminalDockSessions {
		if session.terminal.HasFocus() {
			return true
		}
	}
	return false
}

func (w *mainWindow) backgroundPollingAllowed(now time.Time) bool {
	if w.manualRefreshPaused.Load() || !w.windowActive.Load() {
		return false
	}
	if timeout := w.idleTimeoutSeconds.Load(); timeout > 0 {
		last := time.Unix(0, w.lastAWSActivity.Load())
		return now.Sub(last) < time.Duration(timeout)*time.Second
	}
	return true
}

func (w *mainWindow) installAWSActivityControllers(widgets ...*gtk.Widget) {
	for _, widget := range widgets {
		click := gtk.NewGestureClick()
		click.ConnectPressed(func(_ int, _, _ float64) { w.noteAWSActivity() })
		widget.AddController(click)

		scroll := gtk.NewEventControllerScroll(gtk.EventControllerScrollBothAxes)
		scroll.SetPropagationPhase(gtk.PhaseCapture)
		scroll.ConnectScroll(func(_, _ float64) bool {
			w.noteAWSActivity()
			return false
		})
		widget.AddController(scroll)
	}
}

func (w *mainWindow) autoRefreshClass() refreshpolicy.Class {
	if w.showingLogs || w.showingEditor || w.showingTerminal || w.settingsOpen {
		return refreshpolicy.Manual
	}
	if w.showingMetrics {
		return refreshpolicy.Metrics
	}
	switch w.currentPage {
	case pageServices, pageTasks, pageStandaloneTasks, pageStoppedTasks, pageCodeBuildBuilds:
		return refreshpolicy.Operational
	case pageS3Buckets, pageS3Objects, pageSQSQueues, pageSQSMessages, pageSecrets, pageSSM:
		return refreshpolicy.Metered
	case pageDynamoItems, pageTofuWorkspaces, pageTofuResources, pageTofuPlan, pageSavedLogSearch,
		pageCostOverview, pageCostBreakdown, pageCostAnomalies, pageCostResources, pageCostSavedView, pageSQLObjects:
		return refreshpolicy.Manual
	default:
		return refreshpolicy.Inventory
	}
}

func (w *mainWindow) automaticRefreshBlockReason(now time.Time) string {
	if w.manualRefreshPaused.Load() {
		return "manually paused"
	}
	if !w.windowActive.Load() {
		return "window is not active"
	}
	if w.settingsOpen {
		return "Settings is open"
	}
	if w.options.Config != nil {
		if limit := w.options.Config.Defaults.CostGuardUSD; limit > 0 && w.options.RequestSnapshot != nil {
			snapshot := w.options.RequestSnapshot()
			if snapshot.EstimatedCostUSD >= limit {
				return fmt.Sprintf("known session request charges reached $%.2f (guard $%.2f)", snapshot.EstimatedCostUSD, limit)
			}
		}
		if timeout := w.options.Config.Defaults.IdleTimeout; timeout > 0 {
			last := time.Unix(0, w.lastAWSActivity.Load())
			if now.Sub(last) >= time.Duration(timeout)*time.Second {
				return fmt.Sprintf("inactive for %s", time.Duration(timeout)*time.Second)
			}
		}
	}
	return ""
}

func (w *mainWindow) updateAutoRefreshBlock(reason string) {
	if reason == w.autoRefreshBlock {
		return
	}
	previous := w.autoRefreshBlock
	w.autoRefreshBlock = reason
	if reason != "" {
		w.setStatus("Automatic AWS refresh paused: "+reason, false)
		return
	}
	if previous != "" {
		w.setStatus("Automatic AWS refresh resumed", false)
	}
}

func (w *mainWindow) scheduleRefresh() {
	glib.IdleAdd(func() {
		if w.ctx.Err() != nil {
			return
		}
		now := time.Now()
		reason := w.automaticRefreshBlockReason(now)
		w.updateAutoRefreshBlock(reason)
		if reason != "" || w.workspaceBusy || w.requestPending {
			return
		}
		base := time.Duration(w.options.RefreshInterval) * time.Second
		class := w.autoRefreshClass()
		if !refreshpolicy.Due(w.lastAutomaticRefresh, now, base, class) {
			return
		}
		w.lastAutomaticRefresh = now
		w.refreshCurrent(false)
	})
}

func (w *mainWindow) autoRefresh() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.scheduleRefresh()
		}
	}
}
