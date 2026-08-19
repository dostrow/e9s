package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/ui/views"
)

func (a App) openRDSInstances() (App, tea.Cmd) {
	a.mode = modeRDS
	a.state = viewRDSInstances
	a.rdsInstancesView = views.NewRDSInstances()
	a.rdsInstancesView = a.rdsInstancesView.SetSize(a.width-3, a.height-6)
	a.loading = true
	rdsService := a.rds
	ctx := a.ctx
	return a, func() tea.Msg {
		instances, err := rdsService.List(ctx, "")
		if err != nil {
			return errMsg{err}
		}
		return rdsInstancesLoadedMsg{instances}
	}
}

func (a App) openRDSDetail(identifier string) (App, tea.Cmd) {
	a.loading = true
	rdsService := a.rds
	ctx := a.ctx
	return a, func() tea.Msg {
		detail, err := rdsService.Detail(ctx, identifier)
		if err != nil {
			return errMsg{err}
		}
		return rdsDetailLoadedMsg{detail}
	}
}

func (a App) refreshRDSInstances() tea.Cmd {
	rdsService := a.rds
	ctx := a.ctx
	return func() tea.Msg {
		instances, err := rdsService.List(ctx, "")
		if err != nil {
			return errMsg{err}
		}
		return rdsInstancesLoadedMsg{instances}
	}
}

func (a App) refreshRDSDetail() tea.Cmd {
	identifier := a.rdsDetailView.InstanceID()
	if identifier == "" {
		return nil
	}
	rdsService := a.rds
	ctx := a.ctx
	return func() tea.Msg {
		detail, err := rdsService.Detail(ctx, identifier)
		if err != nil {
			return errMsg{err}
		}
		return rdsDetailLoadedMsg{detail}
	}
}
