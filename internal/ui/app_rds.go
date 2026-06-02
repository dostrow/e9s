package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/ui/views"
)

func (a App) openRDSInstances() (App, tea.Cmd) {
	a.mode = modeRDS
	a.state = viewRDSInstances
	a.rdsInstancesView = views.NewRDSInstances()
	a.rdsInstancesView = a.rdsInstancesView.SetSize(a.width-3, a.height-6)
	a.loading = true
	client := a.client
	return a, func() tea.Msg {
		instances, err := client.ListRDSInstances(context.Background(), "")
		if err != nil {
			return errMsg{err}
		}
		return rdsInstancesLoadedMsg{instances}
	}
}

func (a App) openRDSDetail(identifier string) (App, tea.Cmd) {
	a.loading = true
	client := a.client
	return a, func() tea.Msg {
		detail, err := client.DescribeRDSInstance(context.Background(), identifier)
		if err != nil {
			return errMsg{err}
		}
		return rdsDetailLoadedMsg{detail}
	}
}

func (a App) refreshRDSInstances() tea.Cmd {
	client := a.client
	return func() tea.Msg {
		instances, err := client.ListRDSInstances(context.Background(), "")
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
	client := a.client
	return func() tea.Msg {
		detail, err := client.DescribeRDSInstance(context.Background(), identifier)
		if err != nil {
			return errMsg{err}
		}
		return rdsDetailLoadedMsg{detail}
	}
}
