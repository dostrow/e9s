//go:build gui

package gui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
)

func (w *mainWindow) openEC2Module() {
	if w.currentPage == pageEC2Instances {
		return
	}
	if w.guardEditorNavigation(w.openEC2Module) {
		return
	}
	w.loadEC2Instances()
}

func (w *mainWindow) loadEC2Instances() {
	w.resetWorkspaceForBrowserChange()
	w.clearEC2Browser()
	w.currentPage = pageEC2Instances
	w.selectedCluster = ""
	w.selectedService = ""
	w.selectedTask = ""
	w.selectedTaskDefinition = nil
	w.updateActionSensitivity()
	w.setBreadcrumb("EC2 / Instances")
	w.backButton.SetSensitive(false)
	w.search.SetPlaceholderText("Filter instances…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageEC2Instances)
	w.setDetail("Loading EC2 instances…", detailIntro)

	if w.options.EC2 == nil {
		w.setDetail("EC2 is unavailable because no EC2 service was configured.", detailError)
		w.setStatus("EC2 service unavailable", true)
		return
	}

	ctx, generation := w.startRequest("Loading EC2 instances…")
	go func() {
		instances, err := w.options.EC2.List(ctx, "")
		w.finishRequest(ctx, generation, err, func() {
			w.allEC2Instances = instances
			w.applyEC2Filter()
			w.setDetail(ec2ListSummary(len(instances)), detailIntro)
		})
	}()
}

func (w *mainWindow) refreshEC2(foreground bool) {
	if w.options.EC2 == nil {
		return
	}
	selected := w.selectedEC2Instance
	ctx, generation := w.startRefreshRequest("Refreshing EC2 instances…", foreground)
	go func() {
		instances, err := w.options.EC2.List(ctx, "")
		var detail *model.EC2InstanceDetail
		if err == nil && selected != "" {
			if _, found := findEC2Instance(instances, selected); found {
				detail, err = w.options.EC2.Detail(ctx, selected)
			}
		}
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allEC2Instances = instances
			w.applyEC2Filter()
			if selected == "" {
				if w.detailContent == detailIntro {
					w.setDetail(ec2ListSummary(len(instances)), detailIntro)
				}
				return
			}
			instance, found := findEC2Instance(instances, selected)
			if !found {
				w.selectedEC2Instance = ""
				w.ec2Detail = nil
				w.setBreadcrumb("EC2 / Instances")
				w.setDetail("The selected EC2 instance is no longer available.\n\n"+ec2ListSummary(len(instances)), detailIntro)
				w.updateActionSensitivity()
				return
			}
			if detail != nil {
				w.ec2Detail = detail
				w.setDetail(formatEC2Detail(*detail), detailEC2)
			} else {
				w.setDetail(formatEC2InstanceSummary(instance), detailEC2)
			}
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) clearEC2Browser() {
	w.allEC2Instances = nil
	w.filteredEC2Instances = nil
	w.selectedEC2Instance = ""
	w.ec2Detail = nil
	if w.ec2Table != nil {
		w.ec2Table.clear()
	}
}

func (w *mainWindow) applyEC2Filter() {
	w.filteredEC2Instances = service.FilterEC2Instances(w.allEC2Instances, w.search.Text())
	rows := make([]string, len(w.filteredEC2Instances))
	for i, instance := range w.filteredEC2Instances {
		rows[i] = fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s",
			valueOrDash(instance.Name), instance.InstanceID, valueOrDash(instance.State),
			valueOrDash(instance.Type), valueOrDash(instance.AZ), valueOrDash(instance.PrivateIP),
			valueOrDash(instance.PublicIP), formatEC2Age(instance.LaunchTime, time.Now()))
	}
	w.ec2Table.replace(rows)
}

func (w *mainWindow) selectEC2InstanceRow() {
	if w.currentPage != pageEC2Instances {
		return
	}
	position := w.ec2Table.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredEC2Instances) {
		if w.selectedEC2Instance != "" {
			w.resetWorkspaceForBrowserChange()
		}
		w.selectedEC2Instance = ""
		w.ec2Detail = nil
		w.setBreadcrumb("EC2 / Instances")
		w.setDetail(ec2ListSummary(len(w.allEC2Instances)), detailIntro)
		w.updateActionSensitivity()
		return
	}
	instance := w.filteredEC2Instances[position]
	if w.selectedEC2Instance != instance.InstanceID {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedEC2Instance = instance.InstanceID
	w.ec2Detail = nil
	w.setBreadcrumb("EC2 / Instances / " + ec2DisplayName(instance))
	w.setDetail("Loading instance details…\n\n"+formatEC2InstanceSummary(instance), detailEC2)
	w.updateActionSensitivity()
	w.loadEC2Detail(instance.InstanceID)
}

func (w *mainWindow) openEC2InstanceAt(position uint) {
	if int(position) >= len(w.filteredEC2Instances) {
		return
	}
	if w.ec2Table.selection.Selected() == position {
		w.loadEC2Detail(w.filteredEC2Instances[position].InstanceID)
	} else {
		w.ec2Table.selection.SetSelected(position)
	}
}

func (w *mainWindow) loadEC2Detail(instanceID string) {
	if instanceID == "" || w.options.EC2 == nil {
		return
	}
	ctx, generation := w.startRequest("Loading EC2 instance " + instanceID + "…")
	go func() {
		detail, err := w.options.EC2.Detail(ctx, instanceID)
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageEC2Instances || w.selectedEC2Instance != instanceID {
				return
			}
			w.ec2Detail = detail
			w.setDetail(formatEC2Detail(*detail), detailEC2)
			w.updateActionSensitivity()
		})
	}()
}

func findEC2Instance(instances []model.EC2Instance, instanceID string) (model.EC2Instance, bool) {
	for _, instance := range instances {
		if instance.InstanceID == instanceID {
			return instance, true
		}
	}
	return model.EC2Instance{}, false
}

func ec2DisplayName(instance model.EC2Instance) string {
	if instance.Name != "" {
		return instance.Name
	}
	return instance.InstanceID
}

func ec2ListSummary(count int) string {
	return fmt.Sprintf("EC2 instances: %d\n\nSelect an instance to inspect its configuration, networking, security groups, volumes, and tags.", count)
}

func formatEC2Age(launched, now time.Time) string {
	if launched.IsZero() {
		return "—"
	}
	age := now.Sub(launched)
	if age < 0 {
		age = 0
	}
	switch {
	case age < time.Minute:
		return fmt.Sprintf("%ds", int(age.Seconds()))
	case age < time.Hour:
		return fmt.Sprintf("%dm", int(age.Minutes()))
	case age < 24*time.Hour:
		return fmt.Sprintf("%dh", int(age.Hours()))
	default:
		return fmt.Sprintf("%dd", int(age.Hours()/24))
	}
}

func formatEC2InstanceSummary(instance model.EC2Instance) string {
	return fmt.Sprintf("EC2 INSTANCE\n\nName           %s\nInstance ID    %s\nState          %s\nType           %s\nAvailability   %s\nPrivate IP     %s\nPublic IP      %s\nLaunched       %s\nAMI            %s\nIAM role       %s",
		valueOrDash(instance.Name), instance.InstanceID, valueOrDash(instance.State),
		valueOrDash(instance.Type), valueOrDash(instance.AZ), valueOrDash(instance.PrivateIP),
		valueOrDash(instance.PublicIP), formatTime(instance.LaunchTime), valueOrDash(instance.AMI),
		valueOrDash(instance.IAMRole))
}

func formatEC2Detail(detail model.EC2InstanceDetail) string {
	var out strings.Builder
	out.WriteString(formatEC2InstanceSummary(detail.EC2Instance))
	fmt.Fprintf(&out, "\nPlatform       %s\nArchitecture   %s\nKey name       %s\nMonitoring     %s\nRoot device    %s (%s)\nEBS optimized  %s",
		valueOrDash(detail.Platform), valueOrDash(detail.Architecture), valueOrDash(detail.KeyName),
		valueOrDash(detail.Monitoring), valueOrDash(detail.RootDeviceName), valueOrDash(detail.RootDeviceType),
		yesNo(detail.EBSOptimized))

	fmt.Fprintf(&out, "\n\nNETWORKING\n\nVPC            %s\nSubnet         %s\nPrivate IP     %s\nPublic IP      %s",
		valueOrDash(detail.VpcID), valueOrDash(detail.SubnetID), valueOrDash(detail.PrivateIP), valueOrDash(detail.PublicIP))

	out.WriteString("\n\nSECURITY GROUPS\n")
	if len(detail.SecurityGroups) == 0 {
		out.WriteString("\n  None\n")
	} else {
		for _, group := range detail.SecurityGroups {
			fmt.Fprintf(&out, "\n  %-28s %s", valueOrDash(group.Name), valueOrDash(group.ID))
		}
		out.WriteByte('\n')
	}
	if len(detail.SecurityGroupRules) > 0 {
		out.WriteString("\nSECURITY GROUP RULES\n\n")
		fmt.Fprintf(&out, "%-10s %-10s %-12s %s\n", "DIRECTION", "PROTOCOL", "PORTS", "SOURCE / DESTINATION")
		for _, rule := range detail.SecurityGroupRules {
			fmt.Fprintf(&out, "%-10s %-10s %-12s %s\n", valueOrDash(rule.Direction), valueOrDash(rule.Protocol),
				valueOrDash(rule.PortRange), valueOrDash(rule.Source))
		}
	}

	out.WriteString("\nVOLUMES\n")
	if len(detail.Volumes) == 0 {
		out.WriteString("\n  None\n")
	} else {
		fmt.Fprintf(&out, "\n%-14s %-22s %-10s %-10s %s\n", "DEVICE", "VOLUME", "TYPE", "SIZE", "STATE")
		for _, volume := range detail.Volumes {
			size := "—"
			if volume.Size > 0 {
				size = fmt.Sprintf("%d GiB", volume.Size)
			}
			fmt.Fprintf(&out, "%-14s %-22s %-10s %-10s %s\n", valueOrDash(volume.DeviceName),
				valueOrDash(volume.VolumeID), valueOrDash(volume.VolumeType), size, valueOrDash(volume.State))
		}
	}

	out.WriteString("\nTAGS\n")
	if len(detail.Tags) == 0 {
		out.WriteString("\n  None")
	} else {
		keys := make([]string, 0, len(detail.Tags))
		for key := range detail.Tags {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&out, "\n%-30s %s", key, detail.Tags[key])
		}
	}
	return strings.TrimRight(out.String(), "\n")
}
