package gui

import (
	"strings"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

func TestFilterClusters(t *testing.T) {
	clusters := []model.Cluster{
		{Name: "production", Status: "ACTIVE"},
		{Name: "sandbox", Status: "DRAINING"},
	}

	got := filterClusters(clusters, "PROD")
	if len(got) != 1 || got[0].Name != "production" {
		t.Fatalf("filterClusters() = %#v", got)
	}
	got = filterClusters(clusters, "draining")
	if len(got) != 1 || got[0].Name != "sandbox" {
		t.Fatalf("filterClusters() status match = %#v", got)
	}
}

func TestFilterServicesAcrossVisibleFields(t *testing.T) {
	services := []model.Service{
		{Name: "api", Status: "ACTIVE", HealthStatus: "healthy", TaskDefinition: "api:42", LaunchType: "FARGATE"},
		{Name: "worker", Status: "DRAINING", HealthStatus: "degraded", TaskDefinition: "worker:8", LaunchType: "EC2"},
	}

	for _, query := range []string{"worker", "degraded", "worker:8", "ec2"} {
		got := filterServices(services, query)
		if len(got) != 1 || got[0].Name != "worker" {
			t.Fatalf("filterServices(%q) = %#v", query, got)
		}
	}
}

func TestFormatServiceDetail(t *testing.T) {
	created := time.Date(2026, 8, 17, 20, 0, 0, 0, time.Local)
	svc := model.Service{
		Name:                 "api",
		Status:               "ACTIVE",
		HealthStatus:         "healthy",
		DesiredCount:         2,
		RunningCount:         2,
		TaskDefinition:       "api:42",
		EnableExecuteCommand: true,
		CreatedAt:            created,
		Deployments: []model.Deployment{{
			Status: "PRIMARY", RolloutState: "COMPLETED", RunningCount: 2, DesiredCount: 2,
		}},
		Events: []model.ServiceEvent{{Message: "deployment completed", CreatedAt: created}},
	}
	tasks := []model.Task{{TaskID: "1234567890abcdef", Status: "RUNNING", HealthStatus: "HEALTHY"}}

	got := formatServiceDetail("prod", svc, tasks)
	for _, want := range []string{"prod / api", "api:42", "DEPLOYMENTS", "PRIMARY", "TASKS", "1234567890ab", "RECENT EVENTS", "deployment completed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatServiceDetail() missing %q:\n%s", want, got)
		}
	}
}
