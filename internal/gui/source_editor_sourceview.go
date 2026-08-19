//go:build gui && sourceview

package gui

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	gtksource "libdb.so/gotk4-sourceview/pkg/gtksource/v4"
)

func newSourceEditor(document sourceDocument) *sourceEditor {
	buffer := gtksource.NewBuffer(nil)
	buffer.SetHighlightSyntax(true)
	buffer.SetHighlightMatchingBrackets(true)
	view := gtksource.NewViewWithBuffer(buffer)
	configurePlainSourceView(&view.TextView)
	view.SetShowLineNumbers(true)
	view.SetHighlightCurrentLine(true)
	view.SetAutoIndent(true)
	view.SetInsertSpacesInsteadOfTabs(true)
	view.SetTabWidth(4)

	setDocument := func(document sourceDocument) {
		manager := gtksource.LanguageManagerGetDefault()
		language := manager.Language(document.Language)
		if language == nil && document.Path != "" {
			language = manager.GuessLanguage(document.Path, "")
		}
		buffer.SetLanguage(language)
	}
	setDocument(document)

	applyTheme := func() {
		schemeID := "Adwaita"
		if darkEditorBackground(&view.TextView.Widget) {
			schemeID = "Adwaita-dark"
		}
		if scheme := gtksource.StyleSchemeManagerGetDefault().Scheme(schemeID); scheme != nil {
			buffer.SetStyleScheme(scheme)
		}
	}
	view.ConnectMap(applyTheme)
	if settings := gtk.SettingsGetDefault(); settings != nil {
		settings.NotifyProperty("gtk-theme-name", applyTheme)
		settings.NotifyProperty("gtk-application-prefer-dark-theme", applyTheme)
	}

	return &sourceEditor{
		buffer:      &buffer.TextBuffer,
		view:        view,
		setDocument: setDocument,
	}
}
