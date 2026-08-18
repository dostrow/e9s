package gui

import (
	"fmt"
	"strings"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

func filterClusters(clusters []model.Cluster, query string) []model.Cluster {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return append([]model.Cluster(nil), clusters...)
	}
	filtered := make([]model.Cluster, 0, len(clusters))
	for _, cluster := range clusters {
		if strings.Contains(strings.ToLower(cluster.Name), query) ||
			strings.Contains(strings.ToLower(cluster.Status), query) {
			filtered = append(filtered, cluster)
		}
	}
	return filtered
}

func filterServices(services []model.Service, query string) []model.Service {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return append([]model.Service(nil), services...)
	}
	filtered := make([]model.Service, 0, len(services))
	for _, svc := range services {
		haystack := strings.ToLower(strings.Join([]string{
			svc.Name,
			svc.Status,
			svc.HealthStatus,
			svc.TaskDefinition,
			svc.LaunchType,
		}, " "))
		if strings.Contains(haystack, query) {
			filtered = append(filtered, svc)
		}
	}
	return filtered
}

func findService(services []model.Service, name string) (model.Service, bool) {
	for _, service := range services {
		if service.Name == name {
			return service, true
		}
	}
	return model.Service{}, false
}

func filterTasks(tasks []model.Task, query string) []model.Task {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return append([]model.Task(nil), tasks...)
	}
	filtered := make([]model.Task, 0, len(tasks))
	for _, task := range tasks {
		haystack := strings.ToLower(strings.Join([]string{
			task.TaskID,
			task.TaskARN,
			task.Status,
			task.HealthStatus,
			task.TaskDefinition,
			task.AvailabilityZone,
			task.PrivateIP,
			task.StoppedReason,
		}, " "))
		if strings.Contains(haystack, query) {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func findTask(tasks []model.Task, arn string) (model.Task, bool) {
	for _, task := range tasks {
		if task.TaskARN == arn {
			return task, true
		}
	}
	return model.Task{}, false
}

func clusterSummary(cluster string, serviceCount int) string {
	if serviceCount == 0 {
		return "No ECS services found in " + cluster + "."
	}
	return fmt.Sprintf("%s\n\n%d services\n\nSelect a service and press Enter for deployments, tasks, and recent events.", cluster, serviceCount)
}

func formatServiceDetail(cluster string, svc model.Service, tasks []model.Task) string {
	var out strings.Builder
	fmt.Fprintf(&out, "%s / %s\n\n", cluster, svc.Name)
	fmt.Fprintf(&out, "Status            %s\n", valueOrDash(svc.Status))
	fmt.Fprintf(&out, "Health            %s\n", valueOrDash(svc.HealthStatus))
	fmt.Fprintf(&out, "Desired           %d\n", svc.DesiredCount)
	fmt.Fprintf(&out, "Running           %d\n", svc.RunningCount)
	fmt.Fprintf(&out, "Pending           %d\n", svc.PendingCount)
	fmt.Fprintf(&out, "Task definition   %s\n", valueOrDash(svc.TaskDefinition))
	fmt.Fprintf(&out, "Launch type       %s\n", valueOrDash(svc.LaunchType))
	fmt.Fprintf(&out, "Created           %s\n", formatTime(svc.CreatedAt))
	fmt.Fprintf(&out, "ECS Exec          %s\n", yesNo(svc.EnableExecuteCommand))

	out.WriteString("\nDEPLOYMENTS\n")
	if len(svc.Deployments) == 0 {
		out.WriteString("  No deployments\n")
	}
	for _, deployment := range svc.Deployments {
		fmt.Fprintf(&out, "  %-10s %-12s %d/%d running  %s\n",
			valueOrDash(deployment.Status), valueOrDash(deployment.RolloutState),
			deployment.RunningCount, deployment.DesiredCount,
			valueOrDash(deployment.TaskDefinition))
	}

	out.WriteString("\nTASKS\n")
	if len(tasks) == 0 {
		out.WriteString("  No tasks\n")
	}
	for _, task := range tasks {
		fmt.Fprintf(&out, "  %-12s %-12s %-10s %-16s %s\n",
			shortID(task.TaskID), valueOrDash(task.Status), valueOrDash(task.HealthStatus),
			valueOrDash(task.AvailabilityZone), valueOrDash(task.PrivateIP))
	}

	out.WriteString("\nRECENT EVENTS\n")
	if len(svc.Events) == 0 {
		out.WriteString("  No recent events\n")
	}
	for _, event := range svc.Events {
		fmt.Fprintf(&out, "  %s  %s\n", formatTime(event.CreatedAt), event.Message)
	}
	return out.String()
}

func formatTaskDetail(task model.Task) string {
	var out strings.Builder
	fmt.Fprintf(&out, "TASK\n\n%s\n\n", task.TaskID)
	fmt.Fprintf(&out, "Status            %s\n", valueOrDash(task.Status))
	fmt.Fprintf(&out, "Desired status    %s\n", valueOrDash(task.DesiredStatus))
	fmt.Fprintf(&out, "Health            %s\n", valueOrDash(task.HealthStatus))
	fmt.Fprintf(&out, "Task definition   %s\n", valueOrDash(task.TaskDefinition))
	fmt.Fprintf(&out, "Launch type       %s\n", valueOrDash(task.LaunchType))
	fmt.Fprintf(&out, "Availability zone %s\n", valueOrDash(task.AvailabilityZone))
	fmt.Fprintf(&out, "Private IP        %s\n", valueOrDash(task.PrivateIP))
	fmt.Fprintf(&out, "Group             %s\n", valueOrDash(task.Group))
	fmt.Fprintf(&out, "ECS Exec agent    %s\n", yesNo(task.ExecAgentRunning))
	fmt.Fprintf(&out, "Started           %s\n", formatTime(task.StartedAt))
	if !task.StoppedAt.IsZero() {
		fmt.Fprintf(&out, "Stopped           %s\n", formatTime(task.StoppedAt))
	}
	if task.StoppedReason != "" {
		fmt.Fprintf(&out, "Stop reason       %s\n", task.StoppedReason)
	}
	fmt.Fprintf(&out, "Task ARN          %s\n", valueOrDash(task.TaskARN))

	out.WriteString("\nCONTAINERS\n")
	if len(task.Containers) == 0 {
		out.WriteString("  No containers\n")
	}
	for _, container := range task.Containers {
		fmt.Fprintf(&out, "\n  %s\n", valueOrDash(container.Name))
		fmt.Fprintf(&out, "    Status       %s\n", valueOrDash(container.Status))
		fmt.Fprintf(&out, "    Health       %s\n", valueOrDash(container.HealthStatus))
		fmt.Fprintf(&out, "    Image        %s\n", valueOrDash(container.Image))
		if container.ExitCode != nil {
			fmt.Fprintf(&out, "    Exit code    %d\n", *container.ExitCode)
		}
		if container.Reason != "" {
			fmt.Fprintf(&out, "    Reason       %s\n", container.Reason)
		}
		if container.LogGroup != "" {
			fmt.Fprintf(&out, "    Log group    %s\n", container.LogGroup)
		}
		if container.LogStream != "" {
			fmt.Fprintf(&out, "    Log stream   %s\n", container.LogStream)
		}
	}
	return strings.TrimRight(out.String(), "\n")
}

func valueOrDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

func yesNo(value bool) string {
	if value {
		return "enabled"
	}
	return "disabled"
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "—"
	}
	return value.Local().Format("2006-01-02 15:04:05")
}

func shortID(value string) string {
	if len(value) <= 12 {
		return valueOrDash(value)
	}
	return value[:12]
}
