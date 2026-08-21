//go:build gui

package gui

import (
	"testing"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func TestZoomAcceleratorsParse(t *testing.T) {
	accelerators := append([]string{}, zoomInAccelerators...)
	accelerators = append(accelerators, zoomOutAccelerators...)
	accelerators = append(accelerators, zoomResetAccelerators...)
	for _, accelerator := range accelerators {
		_, _, ok := gtk.AcceleratorParse(accelerator)
		if !ok {
			t.Errorf("GTK could not parse zoom accelerator %q", accelerator)
		}
	}
}
