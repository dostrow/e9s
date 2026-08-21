//go:build gui

package gui

import (
	"fmt"

	"github.com/dostrow/e9s/internal/model"
)

func findCluster(clusters []model.Cluster, name string) (model.Cluster, bool) {
	for _, cluster := range clusters {
		if cluster.Name == name {
			return cluster, true
		}
	}
	return model.Cluster{}, false
}

func clusterListSummary(clusterCount int) string {
	return fmt.Sprintf("ECS CLUSTERS\n\n%d clusters loaded\n\nSelect a cluster to inspect it. Double-click or press Enter to browse its services.", clusterCount)
}
