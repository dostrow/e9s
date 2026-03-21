package ui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/ui/views"
)

// --- SSM Parameter Store ---

func (a App) promptSSMPath() (App, tea.Cmd) {
	saved := a.cfg.SSMPrefixes
	if len(saved) == 0 {
		a.input = NewInput(InputSSMPath, "SSM Parameter path prefix", "/")
		return a, nil
	}

	items := make([]string, 0, len(saved)+1)
	for _, p := range saved {
		items = append(items, fmt.Sprintf("%s  (%s)", p.Name, p.Prefix))
	}
	savedCount := len(items)
	items = append(items, "[enter a custom path]")
	a.picker = NewPickerWithDelete(PickerSSMPrefix, "Select SSM prefix", items, savedCount)
	return a, nil
}

func (a App) saveSSMPrefix() (App, tea.Cmd) {
	prefix := a.ssmView.PathPrefix()
	if prefix == "" {
		return a, nil
	}
	a.input = NewInput(InputSSMSaveName,
		fmt.Sprintf("Save prefix %q — enter a name", prefix),
		"")
	return a, nil
}

func (a App) doSaveSSMPrefix(name string) (App, tea.Cmd) {
	prefix := a.ssmView.PathPrefix()
	a.cfg.AddSSMPrefix(name, prefix)
	if err := a.cfg.Save(); err != nil {
		a.err = err
		return a, nil
	}
	a.flashMessage = fmt.Sprintf("Saved SSM prefix %q as %q", prefix, name)
	a.flashExpiry = time.Now().Add(5 * time.Second)
	return a, nil
}

func (a App) editSSMParam() (App, tea.Cmd) {
	p := a.ssmView.SelectedParam()
	if p == nil {
		return a, nil
	}
	if p.Type == "SecureString" {
		a.ssmEditName = p.Name
		client := a.client
		return a, func() tea.Msg {
			resolved, err := client.GetParameter(context.Background(), p.Name)
			if err != nil {
				return errMsg{fmt.Errorf("failed to read current value: %w", err)}
			}
			return ssmEditReadyMsg{name: p.Name, currentValue: resolved.Value, paramType: p.Type}
		}
	}
	a.ssmEditName = p.Name
	a.input = NewInput(InputSSMEditValue,
		fmt.Sprintf("Edit %s (type: %s)", p.Name, p.Type),
		p.Value)
	return a, nil
}

func (a App) confirmSSMUpdate(newValue string) (App, tea.Cmd) {
	a.ssmEditValue = newValue
	a.confirm = NewConfirm(ConfirmSSMUpdate,
		fmt.Sprintf("Update %q to new value?", a.ssmEditName))
	return a, nil
}

func (a App) doSSMUpdate() tea.Cmd {
	client := a.client
	name := a.ssmEditName
	value := a.ssmEditValue
	pathPrefix := a.ssmView.PathPrefix()
	return func() tea.Msg {
		err := client.PutParameter(context.Background(), name, value)
		if err != nil {
			return errMsg{err}
		}
		params, err := client.ListParameters(context.Background(), pathPrefix)
		if err != nil {
			return actionSuccessMsg{fmt.Sprintf("Updated %q (reload failed: %v)", name, err)}
		}
		return ssmUpdatedMsg{name: name, params: params}
	}
}

func (a App) openSSM(pathPrefix string) (App, tea.Cmd) {
	a.mode = modeSSM
	a.state = viewSSM
	a.ssmView = views.NewSSM(pathPrefix)
	a.ssmView = a.ssmView.SetSize(a.width, a.height-3)
	a.loading = true
	return a, a.loadSSMParams(pathPrefix)
}

func (a App) loadSSMParams(pathPrefix string) tea.Cmd {
	client := a.client
	return func() tea.Msg {
		params, err := client.ListParameters(context.Background(), pathPrefix)
		if err != nil {
			return errMsg{err}
		}
		return ssmParamsLoadedMsg{params: params}
	}
}
