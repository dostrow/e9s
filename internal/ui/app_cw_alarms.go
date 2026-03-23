package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/ui/views"
)

// --- CloudWatch Alarms ---

func (a App) promptCWAlarmsBrowser() (App, tea.Cmd) {
	a.picker = NewPicker(PickerCWAlarmState, "Filter alarms by state", []string{
		"All alarms",
		"ALARM only",
		"OK only",
		"INSUFFICIENT_DATA only",
	})
	return a, nil
}

func (a App) openCWAlarms(stateFilter string) (App, tea.Cmd) {
	a.mode = modeCWAlarms
	a.state = viewAlarms
	a.alarmsView = views.NewAlarms(stateFilter)
	a.alarmsView = a.alarmsView.SetSize(a.width-3, a.height-6)
	a.loading = true
	client := a.client
	return a, func() tea.Msg {
		alarms, err := client.ListCWAlarms(context.Background(), stateFilter)
		if err != nil {
			return errMsg{err}
		}
		return alarmsLoadedMsg{alarms}
	}
}

func (a App) openAlarmDetail() (App, tea.Cmd) {
	alarm := a.alarmsView.SelectedAlarm()
	if alarm == nil {
		return a, nil
	}
	a.state = viewAlarmDetail
	a.loading = true
	client := a.client
	name := alarm.Name
	return a, func() tea.Msg {
		detail, err := client.DescribeCWAlarm(context.Background(), name)
		if err != nil {
			return errMsg{err}
		}
		return alarmDetailLoadedMsg{detail}
	}
}

func (a App) toggleAlarmActions() (App, tea.Cmd) {
	detail := a.alarmDetailView.Detail()
	if detail == nil {
		return a, nil
	}
	name := detail.Name
	enabled := detail.ActionsEnabled
	client := a.client
	a.loading = true
	return a, func() tea.Msg {
		var err error
		if enabled {
			err = client.DisableAlarmActions(context.Background(), name)
		} else {
			err = client.EnableAlarmActions(context.Background(), name)
		}
		if err != nil {
			return errMsg{err}
		}
		action := "enabled"
		if enabled {
			action = "disabled"
		}
		return alarmActionDoneMsg{
			message:   fmt.Sprintf("Actions %s for %q", action, name),
			alarmName: name,
		}
	}
}

func (a App) promptSetAlarmState() (App, tea.Cmd) {
	detail := a.alarmDetailView.Detail()
	if detail == nil {
		return a, nil
	}
	a.picker = NewPicker(PickerSetAlarmState, fmt.Sprintf("Set state for %q", detail.Name), []string{
		"OK",
		"ALARM",
		"INSUFFICIENT_DATA",
	})
	return a, nil
}

func (a App) doSetAlarmState(state string) (App, tea.Cmd) {
	name := a.alarmDetailView.AlarmName()
	client := a.client
	a.loading = true
	return a, func() tea.Msg {
		err := client.SetAlarmState(context.Background(), name, state, "Set manually via e9s")
		if err != nil {
			return errMsg{err}
		}
		return alarmActionDoneMsg{
			message:   fmt.Sprintf("State set to %s for %q", state, name),
			alarmName: name,
		}
	}
}

func (a App) refreshAlarmDetail() tea.Cmd {
	name := a.alarmDetailView.AlarmName()
	if name == "" {
		return nil
	}
	client := a.client
	return func() tea.Msg {
		detail, err := client.DescribeCWAlarm(context.Background(), name)
		if err != nil {
			return errMsg{err}
		}
		return alarmDetailLoadedMsg{detail}
	}
}

func (a App) refreshAlarms() tea.Cmd {
	client := a.client
	stateFilter := a.alarmsView.StateFilter()
	return func() tea.Msg {
		alarms, err := client.ListCWAlarms(context.Background(), stateFilter)
		if err != nil {
			return errMsg{err}
		}
		return alarmsLoadedMsg{alarms}
	}
}

func (a App) handleCWAlarmStatePick(value string) (App, tea.Cmd) {
	stateMap := map[string]string{
		"All alarms":              "",
		"ALARM only":              "ALARM",
		"OK only":                 "OK",
		"INSUFFICIENT_DATA only":  "INSUFFICIENT_DATA",
	}
	state := stateMap[value]
	return a.openCWAlarms(state)
}

func (a App) handleSetAlarmStatePick(value string) (App, tea.Cmd) {
	return a.doSetAlarmState(value)
}

