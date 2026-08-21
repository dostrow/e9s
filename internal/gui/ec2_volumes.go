//go:build gui

package gui

import (
	"fmt"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
)

func (w *mainWindow) openEC2VolumesModule() { w.resourceHistory = nil; w.loadEC2Volumes("") }
func (w *mainWindow) loadEC2Volumes(volumeID string) {
	w.resetWorkspaceForBrowserChange()
	w.allEC2Volumes = nil
	w.filteredEC2Volumes = nil
	w.selectedEC2Volume = ""
	w.ec2VolumeDetail = nil
	w.ec2VolumeTable.clear()
	w.currentPage = pageEC2Volumes
	w.setBreadcrumb("EC2 / Volumes")
	w.backButton.SetSensitive(len(w.resourceHistory) > 0)
	w.search.SetPlaceholderText("Filter volumes…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageEC2Volumes)
	w.setDetail("Loading EBS volumes…", detailIntro)
	ctx, generation := w.startRequest("Loading EBS volumes…")
	go func() {
		volumes, err := w.options.EBS.List(ctx, "")
		w.finishRequest(ctx, generation, err, func() {
			w.allEC2Volumes = volumes
			w.applyEC2VolumeFilter()
			w.setDetail(fmt.Sprintf("Loaded %d EBS volumes. Unattached volumes are listed first.", len(volumes)), detailIntro)
			if volumeID != "" {
				for i, v := range w.filteredEC2Volumes {
					if v.VolumeID == volumeID {
						w.ec2VolumeTable.selection.SetSelected(uint(i))
						return
					}
				}
			}
		})
	}()
}
func (w *mainWindow) applyEC2VolumeFilter() {
	w.filteredEC2Volumes = service.FilterEBSVolumes(w.allEC2Volumes, w.search.Text())
	rows := make([]string, len(w.filteredEC2Volumes))
	for i, v := range w.filteredEC2Volumes {
		attached := make([]string, 0, len(v.Attachments))
		for _, a := range v.Attachments {
			attached = append(attached, a.InstanceID+" "+a.DeviceName)
		}
		rows[i] = fmt.Sprintf("%s\t%s\t%s\t%s\t%d GiB\t%s\t%s", valueOrDash(v.Name), v.VolumeID, v.State, v.VolumeType, v.Size, v.AZ, strings.Join(attached, ", "))
	}
	w.ec2VolumeTable.replace(rows)
}
func (w *mainWindow) selectEC2VolumeRow() {
	if w.currentPage != pageEC2Volumes {
		return
	}
	p := w.ec2VolumeTable.selection.Selected()
	if p == gtk.InvalidListPosition || int(p) >= len(w.filteredEC2Volumes) {
		return
	}
	v := w.filteredEC2Volumes[p]
	w.resetWorkspaceForBrowserChange()
	w.selectedEC2Volume = v.VolumeID
	w.setBreadcrumb("EC2 / Volumes / " + firstValue(v.Name, v.VolumeID))
	w.setDetail("Loading volume details…", detailEC2Volume)
	ctx, g := w.startRequest("Loading EBS volume " + v.VolumeID + "…")
	go func() {
		d, err := w.options.EBS.Detail(ctx, v.VolumeID)
		w.finishRequest(ctx, g, err, func() { w.ec2VolumeDetail = d; w.renderEC2Volume(*d) })
	}()
}
func (w *mainWindow) openEC2VolumeAt(p uint) { w.ec2VolumeTable.selection.SetSelected(p) }
func (w *mainWindow) renderEC2Volume(v model.EC2Volume) {
	var out strings.Builder
	fmt.Fprintf(&out, "EBS VOLUME\n\nName          %s\nVolume ID     %s\nState         %s\nType          %s\nSize          %d GiB\nAZ            %s\nIOPS          %d\nThroughput    %d MiB/s\nEncrypted     %s\nKMS key       %s\nSnapshot      %s\nMulti-attach  %s\nCreated       %s", valueOrDash(v.Name), v.VolumeID, v.State, v.VolumeType, v.Size, v.AZ, v.IOPS, v.Throughput, yesNo(v.Encrypted), valueOrDash(v.KMSKeyID), valueOrDash(v.SnapshotID), yesNo(v.MultiAttach), formatTime(v.CreatedAt))
	out.WriteString("\n\nATTACHMENTS\n")
	links := []workspaceResourceLink{}
	if len(v.Attachments) == 0 {
		out.WriteString("\n  None (unattached)")
	} else {
		for _, a := range v.Attachments {
			fmt.Fprintf(&out, "\n  %-20s %-14s %-12s delete on termination: %s", a.InstanceID, a.DeviceName, a.State, yesNo(a.DeleteOnTermination))
			links = append(links, workspaceResourceLink{label: "Instance: " + a.InstanceID, ref: model.ResourceRef{Kind: "ec2-instance", ID: a.InstanceID}})
		}
	}
	appendSortedTags(&out, v.Tags)
	w.setDetail(out.String(), detailEC2Volume)
	w.setDetailResourceLinks(links)
}
func (w *mainWindow) refreshEC2Volumes(foreground bool) {
	selected := w.selectedEC2Volume
	ctx, g := w.startRefreshRequest("Refreshing EBS volumes…", foreground)
	go func() {
		vs, err := w.options.EBS.List(ctx, "")
		var d *model.EC2Volume
		if err == nil && selected != "" {
			d, err = w.options.EBS.Detail(ctx, selected)
		}
		w.finishRefreshRequest(ctx, g, err, foreground, func() {
			w.allEC2Volumes = vs
			w.applyEC2VolumeFilter()
			if d != nil {
				w.ec2VolumeDetail = d
				w.renderEC2Volume(*d)
			}
		})
	}()
}
