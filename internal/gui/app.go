//go:build gui

// Package gui implements the experimental GTK 4 frontend for e9s.
package gui

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

//go:embed style.css
var styleCSS string

const applicationID = "com.github.dostrow.e9s.gui"

// Run starts the experimental GTK application and returns its exit status.
func Run(args []string) int {
	app := gtk.NewApplication(applicationID, gio.ApplicationFlagsNone)
	var cancel context.CancelFunc

	app.ConnectActivate(func() {
		if cancel != nil {
			return
		}
		var ctx context.Context
		ctx, cancel = context.WithCancel(context.Background())
		activateSpike(ctx, app)
	})
	app.ConnectShutdown(func() {
		if cancel != nil {
			cancel()
		}
	})

	return app.Run(args)
}

func activateSpike(ctx context.Context, app *gtk.Application) {
	installStyles()

	window := gtk.NewApplicationWindow(app)
	window.SetTitle("e9s GTK 4 spike")
	window.SetDefaultSize(1100, 720)

	header := gtk.NewBox(gtk.OrientationHorizontal, 12)
	header.AddCSSClass("toolbar")
	title := gtk.NewLabel("e9s")
	title.AddCSSClass("app-title")
	status := gtk.NewLabel("GTK 4 / native Wayland spike")
	status.SetHExpand(true)
	status.SetXAlign(0)
	header.Append(title)
	header.Append(status)

	rows := syntheticServices(1200)
	list := gtk.NewStringList(rows)
	selection := gtk.NewSingleSelection(list)
	columns := gtk.NewColumnView(selection)
	columns.SetShowColumnSeparators(true)
	columns.SetShowRowSeparators(true)
	columns.AppendColumn(textColumn("SERVICE", 0, true))
	columns.AppendColumn(textColumn("STATUS", 1, false))
	columns.AppendColumn(textColumn("RUNNING", 2, false))
	columns.AppendColumn(textColumn("UPDATED", 3, false))

	listScroll := gtk.NewScrolledWindow()
	listScroll.SetHExpand(true)
	listScroll.SetVExpand(true)
	listScroll.SetChild(columns)

	logBuffer := gtk.NewTextBuffer(nil)
	logBuffer.SetText(syntheticLogs(5000))
	logView := gtk.NewTextViewWithBuffer(logBuffer)
	logView.SetEditable(false)
	logView.SetCursorVisible(false)
	logView.SetMonospace(true)
	logView.SetWrapMode(gtk.WrapNone)
	logView.AddCSSClass("log-view")

	logScroll := gtk.NewScrolledWindow()
	logScroll.SetHExpand(true)
	logScroll.SetVExpand(true)
	logScroll.SetChild(logView)

	content := gtk.NewPaned(gtk.OrientationHorizontal)
	content.SetStartChild(listScroll)
	content.SetEndChild(logScroll)
	content.SetPosition(580)
	content.SetResizeStartChild(true)
	content.SetResizeEndChild(true)

	footer := gtk.NewLabel("Ctrl+R appends a refresh marker • table: 1,200 rows • log: 5,000 bounded spike lines")
	footer.SetXAlign(0)
	footer.AddCSSClass("status-bar")

	root := gtk.NewBox(gtk.OrientationVertical, 0)
	root.Append(header)
	root.Append(content)
	root.Append(footer)
	window.SetChild(root)

	refresh := gio.NewSimpleAction("refresh", nil)
	refresh.ConnectActivate(func(_ *glib.Variant) {
		appendLog(logView, logBuffer, fmt.Sprintf("%s manual refresh requested\n", time.Now().Format(time.RFC3339)))
	})
	app.AddAction(refresh)
	app.SetAccelsForAction("app.refresh", []string{"<Control>r"})

	go appendAsyncMarkers(ctx, logView, logBuffer, status)
	window.Present()
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

func textColumn(title string, field int, expand bool) *gtk.ColumnViewColumn {
	factory := gtk.NewSignalListItemFactory()
	factory.ConnectSetup(func(object *coreglib.Object) {
		item := object.Cast().(*gtk.ColumnViewCell)
		label := gtk.NewLabel("")
		label.SetXAlign(0)
		label.SetEllipsize(3)
		item.SetChild(label)
	})
	factory.ConnectBind(func(object *coreglib.Object) {
		item := object.Cast().(*gtk.ColumnViewCell)
		value := item.Item().Cast().(*gtk.StringObject).String()
		fields := strings.Split(value, "\t")
		text := ""
		if field < len(fields) {
			text = fields[field]
		}
		item.Child().(*gtk.Label).SetLabel(text)
	})

	column := gtk.NewColumnViewColumn(title, &factory.ListItemFactory)
	column.SetExpand(expand)
	column.SetResizable(true)
	return column
}

func syntheticServices(count int) []string {
	rows := make([]string, count)
	statuses := []string{"HEALTHY", "DEPLOYING", "DEGRADED"}
	for i := range rows {
		rows[i] = fmt.Sprintf(
			"service-%04d\t%s\t%d/%d\t%ds ago",
			i+1,
			statuses[i%len(statuses)],
			(i%8)+1,
			(i%8)+1,
			i%60,
		)
	}
	return rows
}

func syntheticLogs(count int) string {
	var logs strings.Builder
	for i := 0; i < count; i++ {
		fmt.Fprintf(&logs, "2026-08-17T22:%02d:%02dZ  service-%04d  synthetic log event %05d\n", (i/60)%60, i%60, (i%1200)+1, i+1)
	}
	return logs.String()
}

func appendAsyncMarkers(ctx context.Context, view *gtk.TextView, buffer *gtk.TextBuffer, status *gtk.Label) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			marker := fmt.Sprintf("%s async main-loop handoff\n", now.Format(time.RFC3339))
			glib.IdleAdd(func() {
				select {
				case <-ctx.Done():
					return
				default:
					appendLog(view, buffer, marker)
					status.SetLabel("Background updates active • " + now.Format("15:04:05"))
				}
			})
		}
	}
}

func appendLog(view *gtk.TextView, buffer *gtk.TextBuffer, line string) {
	end := buffer.EndIter()
	buffer.Insert(end, line)
	view.ScrollToIter(buffer.EndIter(), 0, false, 0, 1)
}
