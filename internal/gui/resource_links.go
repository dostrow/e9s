//go:build gui

package gui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/dostrow/e9s/internal/model"
)

type workspaceResourceLink struct {
	label string
	ref   model.ResourceRef
}

type detailResourceTag struct {
	tag *gtk.TextTag
	ref model.ResourceRef
}

func (w *mainWindow) clearDetailResourceLinks() {
	if w.detailLinks == nil {
		return
	}
	for _, resourceTag := range w.detailResourceTags {
		w.detailBuffer.TagTable().Remove(resourceTag.tag)
	}
	w.detailResourceTags = nil
	w.detailLinks.SetPopover(nil)
	w.detailLinks.SetVisible(false)
}

func (w *mainWindow) setDetailResourceLinks(links []workspaceResourceLink) {
	w.clearDetailResourceLinks()
	links = uniqueWorkspaceResourceLinks(links)
	menu := gtk.NewBox(gtk.OrientationVertical, 2)
	menu.SetMarginTop(6)
	menu.SetMarginBottom(6)
	menu.SetMarginStart(6)
	menu.SetMarginEnd(6)
	for index, link := range links {
		link := link
		button := gtk.NewButtonWithLabel(link.label)
		button.AddCSSClass("flat")
		button.SetHAlign(gtk.AlignFill)
		button.SetTooltipText(fmt.Sprintf("Open %s %s", link.ref.Kind, link.ref.ID))
		button.ConnectClicked(func() { w.openResourceLink(link.ref) })
		menu.Append(button)
		w.applyInlineResourceLink(link, index)
	}
	popover := gtk.NewPopover()
	popover.SetChild(menu)
	w.detailLinks.SetPopover(popover)
	w.detailLinks.SetLabel(fmt.Sprintf("Linked resources (%d)", len(links)))
	w.detailLinks.SetVisible(len(links) > 0)
}

func (w *mainWindow) applyInlineResourceLink(link workspaceResourceLink, index int) {
	needle := link.ref.ID
	if needle == "" || !strings.Contains(w.detailText, needle) {
		needle = link.ref.Name
	}
	if needle == "" {
		return
	}
	tag := gtk.NewTextTag(fmt.Sprintf("resource-link-%d", index))
	tag.SetObjectProperty("underline", int(pango.UnderlineSingle))
	tag.SetObjectProperty("weight", int(pango.WeightSemibold))
	color := w.detailView.StyleContext().Color()
	for _, name := range []string{"accent_color", "theme_selected_bg_color"} {
		if candidate, ok := w.detailView.StyleContext().LookupColor(name); ok {
			color = candidate
			break
		}
	}
	tag.SetObjectProperty("foreground", color.String())
	w.detailBuffer.TagTable().Add(tag)
	w.detailResourceTags = append(w.detailResourceTags, detailResourceTag{tag: tag, ref: link.ref})
	searchFrom := 0
	for {
		position := strings.Index(w.detailText[searchFrom:], needle)
		if position < 0 {
			break
		}
		byteStart := searchFrom + position
		byteEnd := byteStart + len(needle)
		start := utf8.RuneCountInString(w.detailText[:byteStart])
		end := start + utf8.RuneCountInString(needle)
		w.detailBuffer.ApplyTag(tag, w.detailBuffer.IterAtOffset(start), w.detailBuffer.IterAtOffset(end))
		searchFrom = byteEnd
	}
}

func (w *mainWindow) installDetailLinkControllers() {
	click := gtk.NewGestureClick()
	click.SetButton(1)
	click.ConnectPressed(func(_ int, x, y float64) {
		if ref, ok := w.detailResourceAt(x, y); ok {
			w.openResourceLink(ref)
		}
	})
	w.detailView.AddController(click)
	motion := gtk.NewEventControllerMotion()
	motion.ConnectMotion(func(x, y float64) {
		if _, ok := w.detailResourceAt(x, y); ok {
			w.detailView.SetCursorFromName("pointer")
		} else {
			w.detailView.SetCursorFromName("default")
		}
	})
	motion.ConnectLeave(func() { w.detailView.SetCursorFromName("default") })
	w.detailView.AddController(motion)
}

func (w *mainWindow) detailResourceAt(x, y float64) (model.ResourceRef, bool) {
	bufferX, bufferY := w.detailView.WindowToBufferCoords(gtk.TextWindowWidget, int(x), int(y))
	iter, ok := w.detailView.IterAtLocation(bufferX, bufferY)
	if !ok {
		return model.ResourceRef{}, false
	}
	for _, resourceTag := range w.detailResourceTags {
		if iter.HasTag(resourceTag.tag) {
			return resourceTag.ref, true
		}
	}
	return model.ResourceRef{}, false
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
	case "ec2-load-balancer":
		w.loadEC2LoadBalancers(ref.ID)
	case "ec2-target-group":
		w.loadEC2TargetGroups(ref.ID)
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
	case pageEC2LoadBalancers:
		if w.selectedEC2LoadBalancer != "" {
			return model.ResourceRef{Kind: "ec2-load-balancer", ID: w.selectedEC2LoadBalancer}, true
		}
	case pageEC2TargetGroups:
		if w.selectedEC2TargetGroup != "" {
			return model.ResourceRef{Kind: "ec2-target-group", ID: w.selectedEC2TargetGroup}, true
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
