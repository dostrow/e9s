//go:build gui

package gui

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// semanticPalette maps application meaning onto colors exported by the active
// GTK theme. Themes are not required to expose every named color, so each role
// has a conservative fallback derived from colors that GTK always provides.
type semanticPalette struct {
	foreground gdk.RGBA
	surface    gdk.RGBA
	accent     gdk.RGBA
	success    gdk.RGBA
	warning    gdk.RGBA
	error      gdk.RGBA
	info       gdk.RGBA
	muted      gdk.RGBA
}

func semanticPaletteFromStyle(style *gtk.StyleContext) semanticPalette {
	foreground := themeColorOr(style, *style.Color(),
		"theme_text_color", "theme_fg_color", "window_fg_color", "view_fg_color")
	background := themeColorOr(style, foreground, "theme_bg_color", "window_bg_color")
	surface := themeColorOr(style, background,
		"content_view_bg", "text_view_bg", "view_bg_color", "theme_base_color")
	success := themeColorOr(style, foreground, "success_color", "success_bg_color")
	accent := themeColorOr(style, success,
		"accent_color", "link_color")
	warning := themeColorOr(style, accent, "warning_color", "warning_bg_color")
	errorColor := themeColorOr(style, accent, "error_color", "error_bg_color")
	info := themeColorOr(style, accent, "link_color", "accent_color")
	muted := themeColorOr(style, blendRGBA(foreground, background, 0.38),
		"dim_label_fg_color", "insensitive_fg_color")
	return semanticPalette{
		foreground: foreground,
		surface:    surface,
		accent:     accent,
		success:    success,
		warning:    warning,
		error:      errorColor,
		info:       info,
		muted:      muted,
	}
}

// clearDrawingSurface removes the previous Cairo frame without imposing an
// opaque application color. GTK can then composite the widget over whatever
// surface, gradient, or texture the active theme gives its ancestors.
func clearDrawingSurface(cr *cairo.Context) {
	cr.Save()
	cr.SetOperator(cairo.OperatorClear)
	cr.Paint()
	cr.Restore()
}

func themeColorOr(style *gtk.StyleContext, fallback gdk.RGBA, names ...string) gdk.RGBA {
	if color := lookupThemeColor(style, names...); color.Alpha() > 0 {
		return color
	}
	return fallback
}

func blendRGBA(from, to gdk.RGBA, amount float64) gdk.RGBA {
	amount = min(1, max(0, amount))
	channel := func(a, b float32) float32 {
		return float32(float64(a) + (float64(b)-float64(a))*amount)
	}
	return gdk.NewRGBA(
		channel(from.Red(), to.Red()),
		channel(from.Green(), to.Green()),
		channel(from.Blue(), to.Blue()),
		channel(from.Alpha(), to.Alpha()),
	)
}

func rgbaHex(color gdk.RGBA) string {
	channel := func(value float32) uint8 {
		return uint8(min(255, max(0, int(float64(value)*255+0.5))))
	}
	return fmt.Sprintf("#%02X%02X%02X", channel(color.Red()), channel(color.Green()), channel(color.Blue()))
}

func semanticStyleCSS(palette semanticPalette) string {
	return fmt.Sprintf(`
.e9s-table row:not(:selected) .table-cell { color: %s; }
.e9s-table > header > button { color: %s; font-weight: 600; }
.saved-log-modified, .semantic-warning { color: %s; }
.error, .semantic-error { color: %s; }
.semantic-success { color: %s; }
.semantic-info { color: %s; }
.semantic-muted { color: %s; }
.semantic-success, .semantic-warning, .semantic-error, .semantic-info { font-weight: 600; }
`, palette.foreground.String(), palette.accent.String(), palette.warning.String(), palette.error.String(), palette.success.String(),
		palette.info.String(), palette.muted.String())
}

func installSemanticStyles(window *mainWindow) {
	provider := gtk.NewCSSProvider()
	apply := func() {
		palette := semanticPaletteFromStyle(window.window.StyleContext())
		provider.LoadFromString(semanticStyleCSS(palette))
		window.applySemanticPalette(palette)
		window.window.QueueDraw()
	}
	gtk.StyleContextAddProviderForDisplay(
		gdk.DisplayGetDefault(),
		provider,
		gtk.STYLE_PROVIDER_PRIORITY_APPLICATION+1,
	)
	window.window.ConnectMap(apply)
	if settings := gtk.SettingsGetDefault(); settings != nil {
		settings.NotifyProperty("gtk-theme-name", apply)
		settings.NotifyProperty("gtk-application-prefer-dark-theme", apply)
	}
	apply()
}

func (w *mainWindow) applySemanticPalette(palette semanticPalette) {
	if w.detailHeadingTag != nil {
		w.detailHeadingTag.SetObjectProperty("foreground", palette.accent.String())
	}
	if w.detailErrorTag != nil {
		w.detailErrorTag.SetObjectProperty("foreground", palette.error.String())
	}
	for _, resourceTag := range w.detailResourceTags {
		resourceTag.tag.SetObjectProperty("foreground", palette.info.String())
	}
	w.taskDefinitionSourceEditor.ApplyPalette(palette)
	w.lambdaSourceEditor.ApplyPalette(palette)
	if w.terminal != nil {
		w.terminal.SetPalette(palette)
	}
}

func (w *mainWindow) applyDetailHeadingStyles() {
	if w.detailHeadingTag == nil || w.detailText == "" {
		return
	}
	offset := 0
	for _, line := range strings.Split(w.detailText, "\n") {
		length := utf8.RuneCountInString(line)
		if isDetailHeading(line) {
			w.detailBuffer.ApplyTag(
				w.detailHeadingTag,
				w.detailBuffer.IterAtOffset(offset),
				w.detailBuffer.IterAtOffset(offset+length),
			)
		}
		offset += length + 1
	}
}

func isDetailHeading(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" || utf8.RuneCountInString(line) > 64 {
		return false
	}
	hasLetter := false
	for _, r := range line {
		if unicode.IsLower(r) {
			return false
		}
		if unicode.IsUpper(r) {
			hasLetter = true
		}
	}
	return hasLetter
}
