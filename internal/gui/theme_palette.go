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
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/dostrow/e9s/internal/config"
)

// semanticPalette maps application meaning onto colors exported by the active
// GTK theme. Themes are not required to expose every named color, so each role
// has a conservative fallback derived from colors that GTK always provides.
type semanticPalette struct {
	foreground gdk.RGBA
	background gdk.RGBA
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
		background: background,
		surface:    surface,
		accent:     accent,
		success:    success,
		warning:    warning,
		error:      errorColor,
		info:       info,
		muted:      muted,
	}
}

type appearancePreset struct {
	name    string
	palette semanticPalette
}

const appearanceSystem = "system"

// e9s-owned geometry and a selected preset must win over both the desktop
// theme and ~/.config/gtk-4.0/gtk.css. The structural stylesheet contains no
// fixed palette, so System mode still derives all colors from GTK.
const e9sStylePriority = gtk.STYLE_PROVIDER_PRIORITY_USER + 1

var appearancePresets = []appearancePreset{
	{name: appearanceSystem},
	{name: "e9s-dark", palette: paletteFromHex("#E6E6E6", "#181A1B", "#242728", "#6EA8FE", "#7EC699", "#E5C07B", "#E06C75", "#61AFEF", "#8B9195")},
	{name: "e9s-light", palette: paletteFromHex("#25282B", "#F2F3F5", "#FFFFFF", "#315CBE", "#247A45", "#9A6700", "#C42B38", "#2563A8", "#73777C")},
	{name: "gruvbox-material-dark", palette: paletteFromHex("#D4BE98", "#282828", "#32302F", "#7DAEA3", "#A9B665", "#D8A657", "#EA6962", "#7DAEA3", "#928374")},
	{name: "catppuccin-mocha", palette: paletteFromHex("#CDD6F4", "#1E1E2E", "#313244", "#CBA6F7", "#A6E3A1", "#F9E2AF", "#F38BA8", "#89B4FA", "#6C7086")},
	{name: "high-contrast", palette: paletteFromHex("#FFFFFF", "#000000", "#101010", "#00B7FF", "#53FF79", "#FFE65A", "#FF5A67", "#00B7FF", "#BFBFBF")},
}

func paletteFromHex(foreground, background, surface, accent, success, warning, errorColor, info, muted string) semanticPalette {
	parse := func(value string) gdk.RGBA {
		color := gdk.NewRGBA(0, 0, 0, 1)
		color.Parse(value)
		return color
	}
	return semanticPalette{
		foreground: parse(foreground), background: parse(background), surface: parse(surface),
		accent: parse(accent), success: parse(success), warning: parse(warning),
		error: parse(errorColor), info: parse(info), muted: parse(muted),
	}
}

func configuredPreset(name string) (semanticPalette, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = appearanceSystem
	}
	for _, preset := range appearancePresets {
		if preset.name == name {
			return preset.palette, preset.name != appearanceSystem
		}
	}
	return semanticPalette{}, false
}

func appearancePresetLabels() []string {
	return []string{"System GTK theme", "e9s Dark", "e9s Light", "Gruvbox Material Dark", "Catppuccin Mocha", "High Contrast"}
}

func appearancePresetNames() []string {
	names := make([]string, len(appearancePresets))
	for index, preset := range appearancePresets {
		names[index] = preset.name
	}
	return names
}

func appearancePresetIndex(name string) int {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = appearanceSystem
	}
	for index, preset := range appearancePresets {
		if preset.name == name {
			return index
		}
	}
	return 0
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
.e9s-root, .e9s-root .inspector, .e9s-root .log-view { color: %s; }
.e9s-root .e9s-table row:not(:selected) .table-cell { color: %s; }
.e9s-root .e9s-table > header > button,
.e9s-root columnview > header > button { color: %s; font-weight: 600; }
.saved-log-modified, .semantic-warning { color: %s; }
.error, .semantic-error { color: %s; }
.semantic-success { color: %s; }
.semantic-info { color: %s; }
.semantic-muted { color: %s; }
.semantic-success, .semantic-warning, .semantic-error, .semantic-info { font-weight: 600; }
`, palette.foreground.String(), palette.foreground.String(), palette.accent.String(), palette.warning.String(), palette.error.String(), palette.success.String(),
		palette.info.String(), palette.muted.String())
}

func configuredAppearanceCSS(cfg *config.Config, palette semanticPalette, customPalette bool) string {
	var css strings.Builder
	if customPalette {
		buttonBackground := blendRGBA(palette.surface, palette.foreground, 0.08)
		buttonHoverBackground := blendRGBA(palette.surface, palette.foreground, 0.14)
		trackBackground := blendRGBA(palette.background, palette.foreground, 0.08)
		fmt.Fprintf(&css, `
window { color: %s; background-color: %s; }
window label, window image, window checkbutton, window switch,
window notebook, window notebook tab, window popover,
window entry, window searchentry, window text,
window button, window dropdown { color: %s; }
popover, tooltip { color: %s; background-color: %s; }
popover label, tooltip label { color: %s; }
.e9s-root { color: %s; background-color: %s; }
.e9s-root .toolbar, .e9s-root .status-bar { color: %s; background-color: %s; }
.e9s-root .pane-card, .e9s-root .pane-card > viewport,
.e9s-root .pane-card textview, .e9s-root .pane-card listview,
.e9s-root .pane-card columnview { color: %s; background-color: %s; }
.e9s-root row:selected { background-color: alpha(%s, 0.38); }
window entry, window searchentry, window searchentry > text,
window textview, window listview, window columnview,
window dropdown, window popover { color: %s; background-color: %s; }
window button:not(.flat) { color: %s; background-color: %s; border-color: %s; }
window button:not(.flat):hover { background-color: %s; }
window button:not(.flat):checked, window button:not(.flat):active { background-color: alpha(%s, 0.38); }
window dropdown > button,
window spinbutton,
window spinbutton > text {
  color: %s; background-color: %s; border-color: %s;
  background-image: none; box-shadow: none;
}
window dropdown > button:hover,
window spinbutton > button:hover { background-color: %s; }
window dropdown image,
window spinbutton > button,
window spinbutton > button image {
  color: %s;
}
window spinbutton > button {
  background-color: %s;
  border-color: %s;
  background-image: none;
}
window notebook > header { color: %s; background-color: %s; border-color: %s; }
window notebook > header tab { color: %s; background-color: %s; }
window notebook > header tab:checked { color: %s; border-color: %s; }
window checkbutton > check,
.e9s-settings checkbutton > check {
  min-width: 14px; min-height: 14px;
  margin: 0; padding: 0;
  color: %s; background-color: %s;
  border: 1px solid %s; border-radius: 3px;
  background-image: none; box-shadow: none;
  -gtk-icon-source: none;
}
window checkbutton:checked > check,
window checkbutton > check:checked,
.e9s-settings checkbutton:checked > check,
.e9s-settings checkbutton > check:checked {
  color: %s; background-color: %s; border-color: %s;
  -gtk-icon-source: -gtk-icontheme("object-select-symbolic");
}
window checkbutton:disabled > check,
window checkbutton > check:disabled,
.e9s-settings checkbutton:disabled > check { opacity: 0.55; }
window switch, .e9s-settings switch {
  min-width: 38px; min-height: 20px;
  color: %s; background-color: %s;
  border: 1px solid %s; border-radius: 999px;
}
window switch:checked, .e9s-settings switch:checked { background-color: %s; border-color: %s; }
window switch > slider, .e9s-settings switch > slider {
  min-width: 16px; min-height: 16px;
  background-color: %s; border-radius: 999px;
}
window columnview > header > button,
window .e9s-table > header > button { color: %s; background-color: %s; }
`, palette.foreground.String(), palette.background.String(), palette.foreground.String(),
			palette.foreground.String(), palette.surface.String(), palette.foreground.String(),
			palette.foreground.String(), palette.background.String(), palette.foreground.String(), palette.background.String(),
			palette.foreground.String(), palette.surface.String(), palette.accent.String(),
			palette.foreground.String(), palette.surface.String(),
			palette.foreground.String(), buttonBackground.String(), palette.muted.String(),
			buttonHoverBackground.String(), palette.accent.String(),
			palette.foreground.String(), palette.surface.String(), palette.muted.String(),
			buttonHoverBackground.String(), palette.foreground.String(), buttonBackground.String(), palette.muted.String(),
			palette.foreground.String(), palette.background.String(), palette.muted.String(),
			palette.muted.String(), palette.background.String(), palette.accent.String(), palette.accent.String(),
			palette.foreground.String(), palette.surface.String(), palette.muted.String(),
			palette.background.String(), palette.accent.String(), palette.accent.String(),
			palette.foreground.String(), palette.surface.String(), palette.muted.String(), palette.accent.String(), palette.accent.String(),
			palette.foreground.String(), palette.accent.String(), buttonBackground.String())
		fmt.Fprintf(&css, `
window.dialog,
window.dialog > box.dialog-vbox,
window.dialog .dialog-action-area,
.e9s-settings,
.e9s-settings .e9s-dialog-surface,
.e9s-settings notebook,
.e9s-settings notebook > stack,
.e9s-settings .dialog-action-area {
  color: %s;
  background-color: %s;
}
window button.flat {
  color: %s;
  background-color: transparent;
  border-color: transparent;
  box-shadow: none;
}
window button.flat:hover { background-color: alpha(%s, 0.12); }
window button.flat:checked, window button.flat:active { background-color: alpha(%s, 0.28); }
.e9s-root button.module-subitem {
  color: %s;
  background-color: transparent;
  border-color: transparent;
  box-shadow: none;
}
.e9s-root button.module-subitem:checked {
  color: %s;
  background-color: alpha(%s, 0.28);
  border-color: transparent;
}
`, palette.foreground.String(), palette.background.String(),
			palette.foreground.String(), palette.foreground.String(), palette.accent.String(),
			palette.foreground.String(), palette.accent.String(), palette.accent.String())
		fmt.Fprintf(&css, `
.e9s-root .e9s-settings-page,
.e9s-root .e9s-settings-page notebook,
.e9s-root .e9s-settings-page notebook.e9s-settings-notebook > header,
.e9s-root .e9s-settings-page notebook.e9s-settings-notebook > stack,
.e9s-root .settings-actions {
  color: %s;
  background-color: %s;
}
.e9s-root .e9s-settings-page notebook.e9s-settings-notebook,
.e9s-root .e9s-settings-page notebook.e9s-settings-notebook > header,
.e9s-root .e9s-settings-page notebook.e9s-settings-notebook > header > tabs,
.e9s-root .e9s-settings-page notebook.e9s-settings-notebook > stack {
  border: 0;
  box-shadow: none;
  background-image: none;
}
.e9s-root .e9s-settings-page notebook.e9s-settings-notebook > header > tabs > tab {
  color: %s;
  background-color: transparent;
  border: 0;
  box-shadow: none;
}
.e9s-root .e9s-settings-page notebook.e9s-settings-notebook > header > tabs > tab:checked {
  color: %s;
  box-shadow: inset 0 -2px %s;
}
.e9s-root checkbutton check {
  min-width: 14px; min-height: 14px;
  margin: 0; padding: 0;
  color: %s; background-color: %s;
  border: 1px solid %s; border-radius: 3px;
  background-image: none; box-shadow: none;
  -gtk-icon-shadow: none;
  -gtk-icon-source: none;
}
.e9s-root checkbutton:checked check,
.e9s-root checkbutton check:checked {
  color: %s; background-color: %s; border-color: %s;
  -gtk-icon-source: -gtk-icontheme("object-select-symbolic");
}
.e9s-root checkbutton:disabled check,
.e9s-root checkbutton check:disabled { opacity: 0.55; }
.e9s-root entry.resource-search,
.e9s-root entry.resource-search > text,
.e9s-root entry.resource-search image,
.e9s-root .resource-search,
.e9s-root .resource-search > text,
.e9s-root .resource-search image {
  color: %s;
  background-color: %s;
  border-color: %s;
  background-image: none;
  box-shadow: none;
}
.e9s-root entry.resource-search:focus,
.e9s-root .resource-search:focus { border-color: %s; }
.e9s-root entry.resource-search placeholder,
.e9s-root .resource-search placeholder { color: %s; }
.e9s-root scrollbar,
.e9s-root scrollbar trough { background-color: %s; }
.e9s-root scrollbar slider {
  min-width: 8px; min-height: 8px;
  background-color: alpha(%s, 0.65);
  border: 0;
  border-radius: 999px;
}
.e9s-root scrollbar slider:hover,
.e9s-root scrollbar slider:active { background-color: %s; }
.e9s-root .module-heading > box > title > expander {
  min-width: 0; min-height: 0;
  margin: 0; padding: 0; border: 0;
  opacity: 0;
  -gtk-icon-source: none;
}
`, palette.foreground.String(), palette.background.String(), palette.muted.String(),
			palette.accent.String(), palette.accent.String(),
			palette.foreground.String(), palette.surface.String(), palette.muted.String(),
			palette.background.String(), palette.accent.String(), palette.accent.String(),
			palette.foreground.String(), palette.surface.String(), palette.muted.String(), palette.accent.String(), palette.muted.String(),
			trackBackground.String(), palette.muted.String(), palette.foreground.String())
	}
	if cfg == nil {
		return css.String()
	}
	if font := fontDescriptionCSS(cfg.GUI.Appearance.InterfaceFont); font != "" {
		fmt.Fprintf(&css, ".e9s-root { %s }\nwindow, popover, tooltip { %s }\n", font, font)
	}
	if font := fontDescriptionCSS(cfg.GUI.Appearance.MonospaceFont); font != "" {
		fmt.Fprintf(&css, ".e9s-root .inspector, .e9s-root .log-view, .e9s-root .code-font, window .inspector, window .log-view, window .code-font { %s }\n", font)
	}
	return css.String()
}

func fontDescriptionCSS(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	description := pango.FontDescriptionFromString(value)
	if description == nil || strings.TrimSpace(description.Family()) == "" {
		return ""
	}
	size := float64(description.Size()) / 1024
	if size <= 0 {
		size = 10
	}
	return fmt.Sprintf("font-family: %q; font-size: %.2fpt;", description.Family(), size)
}

func installSemanticStyles(window *mainWindow) {
	provider := gtk.NewCSSProvider()
	window.semanticStyleProvider = provider
	apply := func() {
		window.refreshConfiguredAppearance()
	}
	gtk.StyleContextAddProviderForDisplay(
		gdk.DisplayGetDefault(),
		provider,
		e9sStylePriority,
	)
	window.window.ConnectMap(apply)
	if settings := gtk.SettingsGetDefault(); settings != nil {
		settings.NotifyProperty("gtk-theme-name", apply)
		settings.NotifyProperty("gtk-application-prefer-dark-theme", apply)
	}
	apply()
}

func (w *mainWindow) refreshConfiguredAppearance() {
	if w.semanticStyleProvider == nil {
		return
	}
	w.applyAppearanceConfig(w.options.Config)
}

func (w *mainWindow) applyAppearanceConfig(cfg *config.Config) {
	// Remove the previous preset before asking GTK for its native colors. If the
	// user switches back to System, leaving the old provider populated here
	// causes StyleContext to report the preset we are trying to discard.
	w.semanticStyleProvider.LoadFromString("")
	palette := semanticPaletteFromStyle(w.window.StyleContext())
	custom := false
	if cfg != nil {
		if configured, found := configuredPreset(cfg.GUI.Appearance.Preset); found {
			palette = configured
			custom = true
		}
	}
	w.semanticPalette = palette
	w.semanticPaletteReady = true
	w.semanticStyleProvider.LoadFromString(configuredAppearanceCSS(cfg, palette, custom) + semanticStyleCSS(palette))
	w.applySemanticPalette(palette)
	w.applyConfiguredFonts(cfg)
	for _, area := range w.semanticDrawingAreas {
		area.QueueDraw()
	}
	w.window.QueueDraw()
}

func (w *mainWindow) currentSemanticPalette(style *gtk.StyleContext) semanticPalette {
	if w.semanticPaletteReady {
		return w.semanticPalette
	}
	return semanticPaletteFromStyle(style)
}

func (w *mainWindow) applyConfiguredFonts(cfg *config.Config) {
	font := ""
	if cfg != nil {
		font = strings.TrimSpace(cfg.GUI.Appearance.MonospaceFont)
	}
	if w.terminal != nil {
		w.terminal.SetFont(font)
	}
	for _, session := range w.terminalDockSessions {
		session.terminal.SetFont(font)
	}
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
	w.tofuSourceEditor.ApplyPalette(palette)
	for _, tab := range w.sqlTabs {
		tab.editor.ApplyPalette(palette)
	}
	if w.terminal != nil {
		w.terminal.SetPalette(palette)
	}
	for _, session := range w.terminalDockSessions {
		session.terminal.SetPalette(palette)
	}
	for _, chart := range w.metricsCharts {
		chart.SetPalette(palette)
	}
	if w.costChart != nil {
		w.costChart.SetPalette(palette)
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
