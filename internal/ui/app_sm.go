package ui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/ui/views"
)

// --- Secrets Manager ---

func (a App) promptSMFilter() (App, tea.Cmd) {
	saved := a.cfg.SMFilters
	if len(saved) == 0 {
		a.input = NewInput(InputSMFilter, "Secret name filter (substring match)", "")
		return a, nil
	}

	items := make([]string, 0, len(saved)+1)
	for _, f := range saved {
		items = append(items, fmt.Sprintf("%s  (%s)", f.Name, f.Filter))
	}
	savedCount := len(items)
	items = append(items, "[enter a custom filter]")
	a.picker = NewPickerWithDelete(PickerSMFilter, "Select secrets filter", items, savedCount)
	return a, nil
}

func (a App) openSecrets(nameFilter string) (App, tea.Cmd) {
	a.mode = modeSM
	a.state = viewSecrets
	a.secretsView = views.NewSecrets(nameFilter)
	a.secretsView = a.secretsView.SetSize(a.width, a.height-3)
	a.loading = true
	client := a.client
	return a, func() tea.Msg {
		secrets, err := client.ListSecrets(context.Background(), nameFilter)
		if err != nil {
			return errMsg{err}
		}
		return smSecretsLoadedMsg{secrets}
	}
}

func (a App) fetchSecretValue(name string, tags map[string]string) tea.Cmd {
	client := a.client
	return func() tea.Msg {
		sv, err := client.GetSecretValueByName(context.Background(), name)
		if err != nil {
			return errMsg{err}
		}
		return smValueReadyMsg{name: sv.Name, value: sv.Value, tags: tags}
	}
}

func (a App) saveSMFilter() (App, tea.Cmd) {
	filter := a.secretsView.NameFilter()
	a.input = NewInput(InputSMSaveName,
		fmt.Sprintf("Save filter %q — enter a name", filter), "")
	return a, nil
}

func (a App) doSaveSMFilter(name string) (App, tea.Cmd) {
	filter := a.secretsView.NameFilter()
	a.cfg.AddSMFilter(name, filter)
	if err := a.cfg.Save(); err != nil {
		a.err = err
		return a, nil
	}
	a.flashMessage = fmt.Sprintf("Saved SM filter %q as %q", filter, name)
	a.flashExpiry = time.Now().Add(5 * time.Second)
	return a, nil
}

func (a App) editSecret() (App, tea.Cmd) {
	s := a.secretsView.SelectedSecret()
	if s == nil {
		return a, nil
	}
	a.smEditName = s.Name
	client := a.client
	return a, func() tea.Msg {
		sv, err := client.GetSecretValueByName(context.Background(), s.Name)
		if err != nil {
			return errMsg{fmt.Errorf("failed to read current value: %w", err)}
		}
		return smEditReadyMsg{name: s.Name, currentValue: sv.Value}
	}
}

func (a App) confirmSMUpdate(newValue string) (App, tea.Cmd) {
	a.smEditValue = newValue
	a.confirm = NewConfirm(ConfirmSMUpdate,
		fmt.Sprintf("Update secret %q?", a.smEditName))
	return a, nil
}

func (a App) doSMUpdate() tea.Cmd {
	client := a.client
	name := a.smEditName
	value := a.smEditValue
	nameFilter := a.secretsView.NameFilter()
	return func() tea.Msg {
		err := client.PutSecretValue(context.Background(), name, value)
		if err != nil {
			return errMsg{err}
		}
		secrets, err := client.ListSecrets(context.Background(), nameFilter)
		if err != nil {
			return actionSuccessMsg{fmt.Sprintf("Updated %q (reload failed: %v)", name, err)}
		}
		return smUpdatedMsg{name: name, secrets: secrets}
	}
}
