//go:build gui

package gui

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
)

type workspaceResourceLink struct {
	label string
	ref   model.ResourceRef
}

func (w *mainWindow) clearDetailResourceLinks() {
	if w.detailLinks == nil {
		return
	}
	w.detailLinks.RemoveAll()
	w.detailLinks.SetVisible(false)
}

func (w *mainWindow) setDetailResourceLinks(links []workspaceResourceLink) {
	w.clearDetailResourceLinks()
	for _, link := range links {
		link := link
		button := gtk.NewButtonWithLabel(link.label)
		button.AddCSSClass("flat")
		button.SetTooltipText(fmt.Sprintf("Open %s %s", link.ref.Kind, link.ref.ID))
		button.ConnectClicked(func() { w.openResourceLink(link.ref) })
		w.detailLinks.Append(button)
	}
	w.detailLinks.SetVisible(len(links) > 0)
}

func (w *mainWindow) openResourceLink(ref model.ResourceRef) {
	if origin, ok := w.currentResourceRef(); ok && (origin.Kind != ref.Kind || origin.ID != ref.ID) {
		w.resourceHistory = append(w.resourceHistory, origin)
	}
	w.navigateResourceRef(ref)
}

func (w *mainWindow) navigateResourceRef(ref model.ResourceRef) {
	switch ref.Kind {
	case "ec2-instance":
		w.loadEC2InstancesAt(ref.ID)
	case "ec2-security-group":
		w.loadEC2SecurityGroups(ref.ID)
	case "ec2-vpc":
		w.loadEC2VPCs(ref.ID)
	case "ec2-subnet":
		w.loadEC2Subnets(ref.ID, "")
	case "ec2-subnets":
		w.loadEC2Subnets("", ref.ID)
	case "ec2-volume":
		w.loadEC2Volumes(ref.ID)
	default:
		w.setStatus("Navigation is not implemented for "+ref.Kind, true)
	}
}

func (w *mainWindow) currentResourceRef() (model.ResourceRef, bool) {
	switch w.currentPage {
	case pageEC2Instances:
		if w.selectedEC2Instance != "" {
			return model.ResourceRef{Kind: "ec2-instance", ID: w.selectedEC2Instance}, true
		}
	case pageEC2SecurityGroups:
		if w.selectedEC2SecurityGroup != "" {
			return model.ResourceRef{Kind: "ec2-security-group", ID: w.selectedEC2SecurityGroup}, true
		}
	case pageEC2VPCs:
		if w.selectedEC2VPC != "" {
			return model.ResourceRef{Kind: "ec2-vpc", ID: w.selectedEC2VPC}, true
		}
	case pageEC2Subnets:
		if w.selectedEC2Subnet != "" {
			return model.ResourceRef{Kind: "ec2-subnet", ID: w.selectedEC2Subnet}, true
		}
	case pageEC2Volumes:
		if w.selectedEC2Volume != "" {
			return model.ResourceRef{Kind: "ec2-volume", ID: w.selectedEC2Volume}, true
		}
	}
	return model.ResourceRef{}, false
}

func (w *mainWindow) navigateResourceHistoryBack() bool {
	if len(w.resourceHistory) == 0 {
		return false
	}
	last := len(w.resourceHistory) - 1
	ref := w.resourceHistory[last]
	w.resourceHistory = w.resourceHistory[:last]
	w.navigateResourceRef(ref)
	return true
}

func (w *mainWindow) setEC2InstanceResourceLinks(detail model.EC2InstanceDetail) {
	links := make([]workspaceResourceLink, 0, len(detail.SecurityGroups)+2)
	if detail.VpcID != "" {
		links = append(links, workspaceResourceLink{label: "VPC: " + detail.VpcID,
			ref: model.ResourceRef{Kind: "ec2-vpc", ID: detail.VpcID}})
	}
	if detail.SubnetID != "" {
		links = append(links, workspaceResourceLink{label: "Subnet: " + detail.SubnetID,
			ref: model.ResourceRef{Kind: "ec2-subnet", ID: detail.SubnetID}})
	}
	for _, group := range detail.SecurityGroups {
		label := group.Name
		if label == "" {
			label = group.ID
		} else {
			label += " (" + group.ID + ")"
		}
		links = append(links, workspaceResourceLink{
			label: "Security group: " + label,
			ref:   model.ResourceRef{Kind: "ec2-security-group", ID: group.ID, Name: group.Name},
		})
	}
	for _, volume := range detail.Volumes {
		if volume.VolumeID != "" {
			links = append(links, workspaceResourceLink{label: "Volume: " + volume.VolumeID, ref: model.ResourceRef{Kind: "ec2-volume", ID: volume.VolumeID}})
		}
	}
	w.setDetailResourceLinks(links)
}
