package gui

import (
	"fmt"
	"sort"
	"strconv"
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

func findCluster(clusters []model.Cluster, name string) (model.Cluster, bool) {
	for _, cluster := range clusters {
		if cluster.Name == name {
			return cluster, true
		}
	}
	return model.Cluster{}, false
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
			task.StopCode,
			task.StoppedReason,
		}, " "))
		for _, container := range task.Containers {
			haystack += " " + strings.ToLower(container.Name+" "+container.Reason)
			if container.ExitCode != nil {
				haystack += " " + strconv.Itoa(*container.ExitCode)
			}
		}
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

func clusterListSummary(clusterCount int) string {
	return fmt.Sprintf("ECS CLUSTERS\n\n%d clusters loaded\n\nSelect a cluster to inspect it. Double-click or press Enter to browse its services.", clusterCount)
}

func formatClusterDetail(cluster model.Cluster) string {
	return fmt.Sprintf("CLUSTER\n\nName              %s\nStatus            %s\nActive services   %d\nRunning tasks     %d\nPending tasks     %d\nARN               %s\n\nDouble-click or press Enter to browse this cluster's services.",
		cluster.Name, valueOrDash(cluster.Status), cluster.ActiveServices, cluster.RunningTasks,
		cluster.PendingTasks, valueOrDash(cluster.ARN))
}

func standaloneTaskSummary(cluster string, tasks []model.Task) string {
	if len(tasks) == 0 {
		return "No active standalone ECS tasks found in " + cluster + "."
	}
	return fmt.Sprintf("%s\n\n%d active standalone tasks\n\nSelect a task and press Enter for details, logs, and task actions.", cluster, len(tasks))
}

func stoppedStandaloneTaskSummary(cluster string, tasks []model.Task, hasMore bool) string {
	if len(tasks) == 0 {
		return "No recently stopped standalone ECS tasks are available in " + cluster + ".\n\nECS retains stopped task descriptions for at least one hour."
	}
	message := fmt.Sprintf("%s\n\n%d recently stopped standalone tasks loaded", cluster, len(tasks))
	if hasMore {
		message += " • more available"
	}
	return message + "\n\nSelect a task for exit status, stop details, containers, and logs."
}

func sortStoppedTasks(tasks []model.Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].StoppedAt.After(tasks[j].StoppedAt)
	})
}

func appendUniqueTasks(existing, additions []model.Task) []model.Task {
	seen := make(map[string]struct{}, len(existing)+len(additions))
	for _, task := range existing {
		seen[task.TaskARN] = struct{}{}
	}
	for _, task := range additions {
		if _, found := seen[task.TaskARN]; found {
			continue
		}
		existing = append(existing, task)
		seen[task.TaskARN] = struct{}{}
	}
	return existing
}

func taskExitSummary(task model.Task) string {
	var exits []string
	for _, container := range task.Containers {
		if container.ExitCode == nil {
			continue
		}
		value := strconv.Itoa(*container.ExitCode)
		if len(task.Containers) > 1 {
			value = valueOrDash(container.Name) + "=" + value
		}
		exits = append(exits, value)
	}
	return valueOrDash(strings.Join(exits, ", "))
}

func formatServiceDetail(cluster string, svc model.Service, tasks []model.Task) string {
	return formatServiceDetailWithTasks(cluster, svc, tasks, "TASKS", false)
}

func formatServiceOverview(cluster string, svc model.Service) string {
	return formatServiceDetailWithTasks(cluster, svc, nil, "", false)
}

func formatServiceTaskContext(cluster string, svc model.Service, tasks []model.Task, stopped, hasMore bool) string {
	scope := "Active tasks"
	if stopped {
		scope = "Recently stopped tasks"
	}
	more := ""
	if hasMore {
		more = " • more available"
	}
	return fmt.Sprintf("SERVICE TASKS\n\n%s / %s\n\nStatus            %s\nHealth            %s\nDesired           %d\nRunning           %d\nPending           %d\nTask definition   %s\nScope             %s\nLoaded            %d%s\n\nSelect a task to inspect it.",
		cluster, svc.Name, valueOrDash(svc.Status), valueOrDash(svc.HealthStatus), svc.DesiredCount,
		svc.RunningCount, svc.PendingCount, valueOrDash(svc.TaskDefinition), scope, len(tasks), more)
}

func formatServiceStoppedTasksDetail(cluster string, svc model.Service, tasks []model.Task, hasMore bool) string {
	heading := fmt.Sprintf("RECENTLY STOPPED TASKS — %d LOADED", len(tasks))
	if hasMore {
		heading += " — MORE AVAILABLE"
	}
	return formatServiceDetailWithTasks(cluster, svc, tasks, heading, true)
}

func formatServiceDetailWithTasks(cluster string, svc model.Service, tasks []model.Task, taskHeading string, stopped bool) string {
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

	out.WriteString("\nCONFIGURED RESOURCES\n")
	if len(svc.TargetGroups) == 0 && len(svc.SecurityGroups) == 0 {
		out.WriteString("  No linked infrastructure\n")
	}
	for _, targetGroup := range svc.TargetGroups {
		fmt.Fprintf(&out, "  %-18s %s\n", "Target group", targetGroup.ID)
	}
	for _, group := range svc.SecurityGroups {
		fmt.Fprintf(&out, "  %-18s %s\n", "Security group", group.ID)
	}

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

	if taskHeading != "" {
		fmt.Fprintf(&out, "\n%s\n", taskHeading)
		if len(tasks) == 0 {
			out.WriteString("  No tasks\n")
		}
		for _, task := range tasks {
			if stopped {
				fmt.Fprintf(&out, "  %-12s %s  exit %-16s %s\n",
					shortID(task.TaskID), formatTime(task.StoppedAt), taskExitSummary(task), valueOrDash(task.StopCode))
			} else {
				fmt.Fprintf(&out, "  %-12s %-12s %-10s %-16s %s\n",
					shortID(task.TaskID), valueOrDash(task.Status), valueOrDash(task.HealthStatus),
					valueOrDash(task.AvailabilityZone), valueOrDash(task.PrivateIP))
			}
		}
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
	fmt.Fprintf(&out, "Network interface %s\n", valueOrDash(task.NetworkInterfaceID))
	fmt.Fprintf(&out, "Subnet            %s\n", valueOrDash(task.SubnetID))
	fmt.Fprintf(&out, "VPC               %s\n", valueOrDash(task.VpcID))
	fmt.Fprintf(&out, "EC2 instance      %s\n", valueOrDash(task.EC2InstanceID))
	if len(task.SecurityGroups) > 0 {
		groups := make([]string, 0, len(task.SecurityGroups))
		for _, group := range task.SecurityGroups {
			groups = append(groups, group.ID)
		}
		fmt.Fprintf(&out, "Security groups   %s\n", strings.Join(groups, ", "))
	}
	if len(task.VolumeIDs) > 0 {
		fmt.Fprintf(&out, "EBS volumes       %s\n", strings.Join(task.VolumeIDs, ", "))
	}
	fmt.Fprintf(&out, "Group             %s\n", valueOrDash(task.Group))
	fmt.Fprintf(&out, "ECS Exec agent    %s\n", yesNo(task.ExecAgentRunning))
	fmt.Fprintf(&out, "Started           %s\n", formatTime(task.StartedAt))
	if !task.StoppedAt.IsZero() {
		fmt.Fprintf(&out, "Stopped           %s\n", formatTime(task.StoppedAt))
	}
	if task.StopCode != "" {
		fmt.Fprintf(&out, "Stop code         %s\n", task.StopCode)
	}
	if task.StoppedReason != "" {
		fmt.Fprintf(&out, "Stop reason       %s\n", task.StoppedReason)
	}
	fmt.Fprintf(&out, "Task ARN          %s\n", valueOrDash(task.TaskARN))
	for _, warning := range task.ResourceWarnings {
		fmt.Fprintf(&out, "Resource warning  %s\n", warning)
	}

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

func splitCommaSeparated(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}
