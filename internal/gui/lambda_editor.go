//go:build gui

package gui

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/service"
)

const (
	editorKindTaskDefinition = "task-definition"
	editorKindLambda         = "lambda"
	editorKindTofuVariables  = "tofu-variables"
	maxLambdaEditableBytes   = 2 << 20
)

type lambdaEditableFile struct {
	Path    string
	Content string
}

func (w *mainWindow) buildLambdaCodeEditor() gtk.Widgetter {
	cancelButton := gtk.NewButtonWithLabel("Cancel")
	cancelButton.ConnectClicked(w.closeLambdaCodeEditor)
	w.lambdaEditorTitle = gtk.NewLabel("Lambda ZIP editor")
	w.lambdaEditorTitle.SetXAlign(0)
	w.lambdaEditorTitle.SetHExpand(true)
	w.lambdaEditorTitle.AddCSSClass("breadcrumb")
	w.lambdaEditorFileSelector = gtk.NewDropDown(gtk.NewStringList(nil), nil)
	w.lambdaEditorFileSelector.SetSizeRequest(280, -1)
	w.lambdaEditorFileSelector.NotifyProperty("selected", w.switchLambdaEditorFile)
	w.lambdaEditorUploadButton = gtk.NewButtonWithLabel("Upload deployment…")
	w.lambdaEditorUploadButton.AddCSSClass("suggested-action")
	w.lambdaEditorUploadButton.ConnectClicked(w.confirmLambdaCodeUpload)

	toolbar := gtk.NewBox(gtk.OrientationHorizontal, 8)
	toolbar.AddCSSClass("log-toolbar")
	toolbar.Append(cancelButton)
	toolbar.Append(w.lambdaEditorTitle)
	toolbar.Append(w.lambdaEditorFileSelector)
	toolbar.Append(w.lambdaEditorUploadButton)

	w.lambdaSourceEditor = newSourceEditor(sourceDocument{})
	w.lambdaEditorBuffer = w.lambdaSourceEditor.Buffer()
	w.lambdaSourceEditor.ConnectChanged(func() {
		if !w.lambdaEditorLoading {
			w.lambdaEditorDirty = true
		}
	})
	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetHExpand(true)
	scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
	scroll.SetChild(w.lambdaSourceEditor.Widget())

	pane := gtk.NewBox(gtk.OrientationVertical, 0)
	pane.Append(toolbar)
	pane.Append(scroll)
	return pane
}

func (w *mainWindow) openLambdaCodeEditor() {
	name := w.selectedLambdaFunction
	if name == "" || w.options.Lambda == nil || w.lambdaActionPending || w.showingEditor {
		return
	}
	w.lambdaActionPending = true
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Downloading code for " + name + "…")
	go func() {
		packageType, err := w.options.Lambda.PackageType(ctx, name)
		var directory string
		var files []lambdaEditableFile
		if err == nil && strings.EqualFold(packageType, "Image") {
			err = fmt.Errorf("cannot edit container-image function %q; only ZIP deployments are supported", name)
		}
		if err == nil {
			directory, err = w.options.Lambda.DownloadCode(ctx, name)
		}
		if err == nil {
			files, err = scanLambdaEditableFiles(directory)
		}
		if err != nil && directory != "" {
			_ = os.RemoveAll(directory)
		}
		glib.IdleAdd(func() {
			if ctx.Err() != nil || generation != w.generation {
				if directory != "" {
					_ = os.RemoveAll(directory)
				}
				return
			}
			w.spinner.Stop()
			w.setWorkspaceBusy("", false)
			w.lambdaActionPending = false
			if err != nil {
				w.updateActionSensitivity()
				w.setStatus(err.Error(), true)
				return
			}
			if w.currentPage != pageLambda || w.selectedLambdaFunction != name {
				_ = os.RemoveAll(directory)
				return
			}
			w.startLambdaEditor(name, directory, files)
			w.lastSuccessfulLoad = time.Now()
			w.setStatus("Editing downloaded code for "+name, false)
		})
	}()
}

func scanLambdaEditableFiles(directory string) ([]lambdaEditableFile, error) {
	files := make([]lambdaEditableFile, 0)
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > maxLambdaEditableBytes {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !utf8.Valid(data) || strings.IndexByte(string(data), 0) >= 0 {
			return nil
		}
		relative, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		files = append(files, lambdaEditableFile{Path: relative, Content: string(data)})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inspect Lambda deployment: %w", err)
	}
	slices.SortFunc(files, func(a, b lambdaEditableFile) int { return strings.Compare(a.Path, b.Path) })
	if len(files) == 0 {
		return nil, fmt.Errorf("the Lambda deployment contains no editable UTF-8 text files under %s", formatLambdaBytes(maxLambdaEditableBytes))
	}
	return files, nil
}

func (w *mainWindow) startLambdaEditor(name, directory string, files []lambdaEditableFile) {
	w.discardLambdaEditor()
	w.lambdaEditorFunction = name
	w.lambdaEditorDirectory = directory
	w.lambdaEditorFiles = files
	w.lambdaEditorFileIndex = -1
	w.lambdaEditorDirty = false
	w.lambdaEditorLoading = true
	paths := make([]string, len(files))
	for i, file := range files {
		paths[i] = file.Path
	}
	w.lambdaEditorFileSelector.SetModel(gtk.NewStringList(paths))
	w.lambdaEditorFileSelector.SetSelected(0)
	w.lambdaEditorFileIndex = 0
	w.lambdaSourceEditor.SetDocument(sourceDocument{Path: files[0].Path})
	w.lambdaEditorBuffer.SetText(files[0].Content)
	w.lambdaEditorTitle.SetLabel("Lambda ZIP editor • " + name)
	w.lambdaEditorLoading = false
	w.showingEditor = true
	w.editorKind = editorKindLambda
	w.lambdaTable.view.SetSensitive(false)
	w.search.SetSensitive(false)
	w.detailStack.SetVisibleChildName("lambda-editor")
	w.updateActionSensitivity()
}

func (w *mainWindow) switchLambdaEditorFile() {
	if w.lambdaEditorLoading || !w.showingEditor || w.editorKind != editorKindLambda {
		return
	}
	selected := int(w.lambdaEditorFileSelector.Selected())
	if selected < 0 || selected >= len(w.lambdaEditorFiles) || selected == w.lambdaEditorFileIndex {
		return
	}
	w.saveCurrentLambdaEditorFile()
	w.lambdaEditorLoading = true
	w.lambdaEditorFileIndex = selected
	w.lambdaSourceEditor.SetDocument(sourceDocument{Path: w.lambdaEditorFiles[selected].Path})
	w.lambdaEditorBuffer.SetText(w.lambdaEditorFiles[selected].Content)
	w.lambdaEditorLoading = false
}

func (w *mainWindow) lambdaEditorText() string {
	start, end := w.lambdaEditorBuffer.Bounds()
	return w.lambdaEditorBuffer.Text(start, end, true)
}

func (w *mainWindow) saveCurrentLambdaEditorFile() {
	if w.lambdaEditorFileIndex >= 0 && w.lambdaEditorFileIndex < len(w.lambdaEditorFiles) {
		w.lambdaEditorFiles[w.lambdaEditorFileIndex].Content = w.lambdaEditorText()
	}
}

func (w *mainWindow) confirmLambdaCodeUpload() {
	if !w.showingEditor || w.editorKind != editorKindLambda || w.lambdaActionPending {
		return
	}
	w.saveCurrentLambdaEditorFile()
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsYesNo)
	dialog.SetTitle("Upload Lambda code")
	dialog.SetMarkup("Replace the deployed ZIP code for <b>" + html.EscapeString(w.lambdaEditorFunction) + "</b>?")
	dialog.SetObjectProperty("secondary-text", "All files in the downloaded deployment package are preserved; edited text files are repackaged and uploaded as new function code.")
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.uploadLambdaCode()
		}
	})
	dialog.Present()
}

func (w *mainWindow) uploadLambdaCode() {
	name, directory := w.lambdaEditorFunction, w.lambdaEditorDirectory
	files := append([]lambdaEditableFile(nil), w.lambdaEditorFiles...)
	if name == "" || directory == "" || w.options.Lambda == nil || w.lambdaActionPending {
		return
	}
	w.lambdaActionPending = true
	w.lambdaEditorUploadButton.SetSensitive(false)
	ctx, generation := w.startRequest("Uploading code for " + name + "…")
	go func() {
		err := writeLambdaEditableFiles(directory, files)
		var archive []byte
		if err == nil {
			archive, err = service.ZipDirectory(directory)
		}
		if err == nil {
			err = w.options.Lambda.UpdateCode(ctx, name, archive)
		}
		glib.IdleAdd(func() {
			if ctx.Err() != nil || generation != w.generation {
				return
			}
			w.spinner.Stop()
			w.setWorkspaceBusy("", false)
			w.lambdaActionPending = false
			w.lambdaEditorUploadButton.SetSensitive(true)
			if err != nil {
				w.setStatus(err.Error(), true)
				w.updateActionSensitivity()
				return
			}
			w.closeLambdaCodeEditorNow()
			w.lastSuccessfulLoad = time.Now()
			w.setStatus("Updated Lambda code for "+name, false)
			w.loadLambdaDetail(name)
		})
	}()
}

func writeLambdaEditableFiles(directory string, files []lambdaEditableFile) error {
	cleanRoot := filepath.Clean(directory) + string(os.PathSeparator)
	for _, file := range files {
		path := filepath.Join(directory, file.Path)
		if !strings.HasPrefix(filepath.Clean(path), cleanRoot) {
			return fmt.Errorf("write Lambda deployment: invalid path %q", file.Path)
		}
		if err := os.WriteFile(path, []byte(file.Content), 0o600); err != nil {
			return fmt.Errorf("write Lambda deployment file %q: %w", file.Path, err)
		}
	}
	return nil
}

func (w *mainWindow) closeEditor() {
	w.closeEditorThen(nil)
}

func (w *mainWindow) closeEditorThen(after func()) {
	if w.editorKind == editorKindLambda {
		w.closeLambdaCodeEditorThen(after)
		return
	}
	if w.editorKind == editorKindTofuVariables {
		w.closeTofuVariablesEditorThen(after)
		return
	}
	w.closeTaskDefinitionEditorThen(after)
}

func (w *mainWindow) guardEditorNavigation(after func()) bool {
	if !w.showingEditor {
		return false
	}
	w.closeEditorThen(after)
	return true
}

func (w *mainWindow) saveActiveEditor() {
	switch w.editorKind {
	case editorKindTaskDefinition:
		w.confirmRegisterTaskDefinition()
	case editorKindTofuVariables:
		w.saveTofuVariables()
	}
}

func (w *mainWindow) closeLambdaCodeEditor() {
	w.closeLambdaCodeEditorThen(nil)
}

func (w *mainWindow) closeLambdaCodeEditorThen(after func()) {
	if !w.showingEditor || w.editorKind != editorKindLambda {
		if after != nil {
			after()
		}
		return
	}
	if !w.lambdaEditorDirty {
		w.closeLambdaCodeEditorNow()
		if after != nil {
			after()
		}
		return
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsYesNo)
	dialog.SetTitle("Discard Lambda code changes")
	dialog.SetMarkup("Discard the unsaved edits to <b>" + html.EscapeString(w.lambdaEditorFunction) + "</b>?")
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.closeLambdaCodeEditorNow()
			if after != nil {
				after()
			}
		}
	})
	dialog.Present()
}

func (w *mainWindow) closeLambdaCodeEditorNow() {
	w.showingEditor = false
	w.editorKind = ""
	w.discardLambdaEditor()
	w.detailStack.SetVisibleChildName("detail")
	w.updateActionSensitivity()
}

func (w *mainWindow) discardLambdaEditor() {
	if w.lambdaEditorDirectory != "" {
		_ = os.RemoveAll(w.lambdaEditorDirectory)
	}
	w.lambdaEditorDirectory = ""
	w.lambdaEditorFunction = ""
	w.lambdaEditorFiles = nil
	w.lambdaEditorFileIndex = -1
	w.lambdaEditorDirty = false
	if w.lambdaTable != nil {
		w.lambdaTable.view.SetSensitive(true)
	}
	if w.search != nil && w.currentPage != pageModulePicker {
		w.search.SetSensitive(true)
	}
}
