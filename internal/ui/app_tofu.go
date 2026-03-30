package ui

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/tofu"
	"github.com/dostrow/e9s/internal/ui/views"
)

// --- OpenTofu ---

func (a App) promptTofuBrowser() (App, tea.Cmd) {
	saved := a.cfg.TofuDirs
	if len(saved) == 0 {
		cwd, _ := os.Getwd()
		a.pathInput = &PathInput{}
		pi := NewPathInput(InputTofuDir, "OpenTofu directory path", cwd+"/")
		a.pathInput = &pi
		return a, nil
	}
	items := make([]string, 0, len(saved)+1)
	for _, d := range saved {
		label := d.Name
		dir := d.Dir
		if len(dir) > 40 {
			dir = "..." + dir[len(dir)-37:]
		}
		label += fmt.Sprintf("  (%s)", dir)
		items = append(items, label)
	}
	savedCount := len(items)
	items = append(items, "[enter a directory path]")
	a.picker = NewPickerWithDelete(PickerTofuDir, "Select OpenTofu workspace", items, savedCount)
	return a, nil
}

func (a App) openTofuResources(dir string) (App, tea.Cmd) {
	a.mode = modeTofu
	a.state = viewTofuResources
	a.tofuDir = dir
	a.tofuResourcesView = views.NewTofuResources(dir)
	a.tofuResourcesView = a.tofuResourcesView.SetSize(a.width-3, a.height-6)
	a.loading = true
	return a, func() tea.Msg {
		runner, err := tofu.NewRunner(dir)
		if err != nil {
			return errMsg{err}
		}
		resources, err := runner.StateList()
		if err != nil {
			return errMsg{fmt.Errorf("state list: %w", err)}
		}
		return tofuResourcesLoadedMsg{resources}
	}
}

func (a App) openTofuStateDetail() (App, tea.Cmd) {
	res := a.tofuResourcesView.SelectedResource()
	if res == nil {
		return a, nil
	}
	a.state = viewTofuStateDetail
	a.tofuStateDetailView = views.NewTofuStateDetail(res.Address)
	a.tofuStateDetailView = a.tofuStateDetailView.SetSize(a.width-3, a.height-6)
	a.loading = true
	dir := a.tofuDir
	addr := res.Address
	return a, func() tea.Msg {
		runner, err := tofu.NewRunner(dir)
		if err != nil {
			return errMsg{err}
		}
		output, err := runner.StateShow(addr)
		if err != nil {
			return errMsg{err}
		}
		return tofuStateDetailMsg{output}
	}
}

func (a App) runTofuPlan() (App, tea.Cmd) {
	dir := a.tofuDir
	a.state = viewTofuPlan
	a.tofuPlanView = views.NewTofuPlan(dir)
	a.tofuPlanView = a.tofuPlanView.SetSize(a.width-3, a.height-6)
	a.loading = true
	return a, func() tea.Msg {
		runner, err := tofu.NewRunner(dir)
		if err != nil {
			return errMsg{err}
		}
		jsonOut, err := runner.PlanJSON()
		if err != nil {
			return errMsg{err}
		}
		plan, err := tofu.ParsePlan(jsonOut)
		if err != nil {
			return errMsg{err}
		}
		return tofuPlanLoadedMsg{plan}
	}
}

func (a App) openTofuPlanDetail() (App, tea.Cmd) {
	change := a.tofuPlanView.SelectedChange()
	if change == nil {
		return a, nil
	}
	a.state = viewTofuPlanDetail
	a.tofuPlanDetailView = views.NewTofuPlanDetail(change)
	a.tofuPlanDetailView = a.tofuPlanDetailView.SetSize(a.width-3, a.height-6)
	return a, nil
}

func (a App) runTofuApply() (App, tea.Cmd) {
	dir := a.tofuDir
	runner, err := tofu.NewRunner(dir)
	if err != nil {
		a.err = err
		return a, nil
	}
	// Run apply interactively — user sees the plan and confirms
	wrap := NewExecWrap(runner.Binary, []string{"-chdir=" + dir, "apply", "-no-color"})
	return a, tea.Exec(wrap, func(err error) tea.Msg {
		if err != nil {
			return tofuApplyDoneMsg{fmt.Sprintf("Apply finished with error: %v", err)}
		}
		return tofuApplyDoneMsg{"Apply completed successfully"}
	})
}

func (a App) runTofuInit() (App, tea.Cmd) {
	dir := a.tofuDir
	a.loading = true
	return a, func() tea.Msg {
		runner, err := tofu.NewRunner(dir)
		if err != nil {
			return errMsg{err}
		}
		_, err = runner.Init()
		if err != nil {
			return errMsg{err}
		}
		return tofuInitDoneMsg{"Init completed successfully"}
	}
}

func (a App) saveTofuDir() (App, tea.Cmd) {
	dir := a.tofuDir
	if dir == "" {
		return a, nil
	}
	a.input = NewInput(InputTofuSaveName,
		fmt.Sprintf("Save workspace %q — enter a name", dir), "")
	return a, nil
}

func (a App) doSaveTofuDir(name string) (App, tea.Cmd) {
	a.cfg.AddTofuDir(name, a.tofuDir)
	if err := a.cfg.Save(); err != nil {
		a.err = err
		return a, nil
	}
	a.flashMessage = fmt.Sprintf("Saved workspace as %q", name)
	a.flashExpiry = time.Now().Add(5 * time.Second)
	return a, nil
}

func (a App) refreshTofuResources() tea.Cmd {
	dir := a.tofuDir
	return func() tea.Msg {
		runner, err := tofu.NewRunner(dir)
		if err != nil {
			return errMsg{err}
		}
		resources, err := runner.StateList()
		if err != nil {
			return errMsg{err}
		}
		return tofuResourcesLoadedMsg{resources}
	}
}
