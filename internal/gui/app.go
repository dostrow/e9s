//go:build gui

// Package gui implements the experimental GTK 4 frontend for e9s.
package gui

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
)

const applicationID = "com.github.dostrow.e9s.gui"

type ECSService interface {
	ListClusters(context.Context) ([]model.Cluster, error)
	ListServices(context.Context, string) ([]model.Service, error)
	ListTasks(context.Context, string, string) ([]model.Task, error)
	ForceDeployment(context.Context, string, string) error
	ScaleService(context.Context, string, string, int) error
	StopTask(context.Context, string, string, string) error
	ContainerLogSource(context.Context, model.Task, string) (model.LogSource, error)
	ServiceLogSource(context.Context, string, string) (model.LogSource, error)
}

type LogService interface {
	Fetch(context.Context, string, model.LogQuery) (model.LogPage, error)
}

type Options struct {
	ECS             ECSService
	Logs            LogService
	DefaultCluster  string
	Profile         string
	Region          string
	RefreshInterval int
}

// Run starts the experimental GTK application.
func Run(options Options) error {
	app := gtk.NewApplication(applicationID, gio.ApplicationFlagsNone)
	ctx, cancel := context.WithCancel(context.Background())
	var window *mainWindow

	app.ConnectActivate(func() {
		if window != nil {
			window.window.Present()
			return
		}
		installStyles()
		window = newMainWindow(ctx, app, options)
		window.window.Present()
		window.loadClusters()
	})
	app.ConnectShutdown(cancel)

	if code := app.Run(nil); code != 0 {
		return &ExitError{Code: code}
	}
	return nil
}

type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return "GTK application exited unsuccessfully"
}

func installStyles() {
	provider := gtk.NewCSSProvider()
	provider.LoadFromString(styleCSS)
	gtk.StyleContextAddProviderForDisplay(
		gdk.DisplayGetDefault(),
		provider,
		gtk.STYLE_PROVIDER_PRIORITY_APPLICATION,
	)
}
