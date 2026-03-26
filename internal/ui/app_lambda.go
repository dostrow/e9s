package ui

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/views"
)

// --- Lambda ---

func (a App) promptLambdaBrowser() (App, tea.Cmd) {
	saved := a.cfg.LambdaSearches
	if len(saved) == 0 {
		a.input = NewInput(InputLambdaSearch, "Search functions (substring match, or empty for all)", "")
		return a, nil
	}
	items := make([]string, 0, len(saved)+1)
	for _, s := range saved {
		items = append(items, fmt.Sprintf("%s  (%s)", s.Name, s.Filter))
	}
	savedCount := len(items)
	items = append(items, "[enter a custom search]")
	a.picker = NewPickerWithDelete(PickerLambdaSearch, "Select Lambda search", items, savedCount)
	return a, nil
}

func (a App) openLambdaList(filter string) (App, tea.Cmd) {
	a.mode = modeLambda
	a.state = viewLambdaList
	a.lambdaListView = views.NewLambdaList(filter)
	a.lambdaListView = a.lambdaListView.SetSize(a.width, a.height-3)
	a.loading = true
	client := a.client
	return a, func() tea.Msg {
		functions, err := client.ListLambdaFunctions(context.Background(), filter)
		if err != nil {
			return errMsg{err}
		}
		return lambdaFunctionsLoadedMsg{functions}
	}
}

func (a App) saveLambdaSearch() (App, tea.Cmd) {
	filter := a.lambdaListView.SearchTerm()
	a.input = NewInput(InputLambdaSaveName,
		fmt.Sprintf("Save Lambda search %q — enter a name", filter), "")
	return a, nil
}

func (a App) doSaveLambdaSearch(name string) (App, tea.Cmd) {
	filter := a.lambdaListView.SearchTerm()
	a.cfg.AddLambdaSearch(name, filter)
	if err := a.cfg.Save(); err != nil {
		a.err = err
		return a, nil
	}
	a.flashMessage = fmt.Sprintf("Saved Lambda search %q as %q", filter, name)
	a.flashExpiry = time.Now().Add(5 * time.Second)
	return a, nil
}

func (a App) tailLambdaLogs() (App, tea.Cmd) {
	fn := a.lambdaListView.SelectedFunction()
	if fn == nil {
		return a, nil
	}
	a.prevState = viewLambdaList
	logGroup := fn.LogGroup
	return a, func() tea.Msg {
		return logReadyMsg{
			title:    fmt.Sprintf("λ %s", fn.Name),
			logGroup: logGroup,
			streams:  nil, // tail entire log group
		}
	}
}

func (a App) showLambdaEnvVars() (App, tea.Cmd) {
	fn := a.lambdaDetailView.Function()
	if fn == nil || len(fn.EnvVars) == 0 {
		a.err = fmt.Errorf("no environment variables")
		return a, nil
	}
	a.prevState = viewLambdaDetail
	client := a.client

	// Resolve any SSM/SM references
	return a, func() tea.Msg {
		resolved := client.ResolveEnvVars(context.Background(), fn.EnvVars)
		return envVarsReadyMsg{
			title:   fmt.Sprintf("λ %s", fn.Name),
			envVars: resolved,
		}
	}
}

func (a App) searchLambdaLogs() (App, tea.Cmd) {
	fn := a.lambdaListView.SelectedFunction()
	if fn == nil {
		return a, nil
	}
	a.prevState = viewLambdaList
	a.logSearchGroup = fn.LogGroup
	a.logSearchGroups = []string{fn.LogGroup}
	a.logSearchStream = ""
	return a.promptLogSearchTimeRange()
}

func (a App) searchLambdaDetailLogs() (App, tea.Cmd) {
	fn := a.lambdaDetailView.Function()
	if fn == nil {
		return a, nil
	}
	a.prevState = viewLambdaDetail
	a.logSearchGroup = fn.LogGroup
	a.logSearchGroups = []string{fn.LogGroup}
	a.logSearchStream = ""
	return a.promptLogSearchTimeRange()
}

func (a App) editLambdaCode() (App, tea.Cmd) {
	fn := a.lambdaDetailView.Function()
	if fn == nil {
		return a, nil
	}
	a.loading = true
	client := a.client
	name := fn.Name
	return a, func() tea.Msg {
		// Check package type
		pkgType, err := client.LambdaPackageType(context.Background(), name)
		if err != nil {
			return errMsg{err}
		}
		if pkgType == "Image" {
			return errMsg{fmt.Errorf("cannot edit container image functions — only ZIP deployments are supported")}
		}

		dir, err := client.DownloadLambdaCode(context.Background(), name)
		if err != nil {
			return errMsg{err}
		}
		return lambdaCodeReadyMsg{functionName: name, dir: dir}
	}
}

func (a App) handleLambdaCodeReady(msg lambdaCodeReadyMsg) (App, tea.Cmd) {
	a.loading = false
	a.lambdaEditDir = msg.dir
	a.lambdaEditFunc = msg.functionName

	editor := NewEditorCmd(msg.dir)
	return a, tea.Exec(editor, func(err error) tea.Msg {
		if err != nil {
			os.RemoveAll(msg.dir)
			return errMsg{err}
		}
		// Repackage the directory
		zipData, err := aws.ZipDirectory(msg.dir)
		os.RemoveAll(msg.dir)
		if err != nil {
			return errMsg{fmt.Errorf("failed to create zip: %w", err)}
		}
		return lambdaCodeEditedMsg{
			functionName: msg.functionName,
			zipData:      zipData,
		}
	})
}

func (a App) doLambdaCodeUpdate() tea.Cmd {
	client := a.client
	name := a.lambdaEditFunc
	data := a.lambdaEditZip
	return func() tea.Msg {
		err := client.UpdateLambdaCode(context.Background(), name, data)
		if err != nil {
			return errMsg{err}
		}
		return lambdaCodeUpdatedMsg{fmt.Sprintf("Code updated for %s", name)}
	}
}

func (a App) tailLambdaDetailLogs() (App, tea.Cmd) {
	fn := a.lambdaDetailView.Function()
	if fn == nil {
		return a, nil
	}
	a.prevState = viewLambdaDetail
	logGroup := fn.LogGroup
	return a, func() tea.Msg {
		return logReadyMsg{
			title:    fmt.Sprintf("λ %s", fn.Name),
			logGroup: logGroup,
			streams:  nil,
		}
	}
}
