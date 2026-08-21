package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/views"
)

func (a App) openRDSInstances() (App, tea.Cmd) {
	a.rdsClusterContext = ""
	return a.openRDSInstancesForCluster("")
}

func (a App) openRDSInstancesForCluster(clusterID string) (App, tea.Cmd) {
	a.mode = modeRDS
	a.state = viewRDSInstances
	a.rdsClusterContext = clusterID
	a.rdsInstancesView = views.NewRDSInstances()
	a.rdsInstancesView = a.rdsInstancesView.SetCluster(clusterID)
	a.rdsInstancesView = a.rdsInstancesView.SetSize(a.width-3, a.height-6)
	a.loading = true
	rdsService := a.rds
	ctx := a.ctx
	return a, func() tea.Msg {
		var instances []model.RDSInstance
		var err error
		if clusterID != "" {
			instances, err = rdsService.ClusterInstances(ctx, clusterID)
		} else {
			instances, err = rdsService.List(ctx, "")
		}
		if err != nil {
			return errMsg{err}
		}
		return rdsInstancesLoadedMsg{instances}
	}
}

func (a App) openRDSClusters() (App, tea.Cmd) {
	a.mode = modeRDS
	a.state = viewRDSClusters
	a.rdsClusterContext = ""
	a.rdsClustersView = views.NewRDSClusters().SetSize(a.width-3, a.height-6)
	a.loading = true
	rdsService, ctx := a.rds, a.ctx
	return a, func() tea.Msg {
		clusters, err := rdsService.Clusters(ctx, "")
		if err != nil {
			return errMsg{err}
		}
		return rdsClustersLoadedMsg{clusters}
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
		var instances []model.RDSInstance
		var err error
		if a.rdsClusterContext != "" {
			instances, err = rdsService.ClusterInstances(ctx, a.rdsClusterContext)
		} else {
			instances, err = rdsService.List(ctx, "")
		}
		if err != nil {
			return errMsg{err}
		}
		return rdsInstancesLoadedMsg{instances}
	}
}

func (a App) refreshRDSClusters() tea.Cmd {
	rdsService, ctx := a.rds, a.ctx
	return func() tea.Msg {
		clusters, err := rdsService.Clusters(ctx, "")
		if err != nil {
			return errMsg{err}
		}
		return rdsClustersLoadedMsg{clusters}
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
