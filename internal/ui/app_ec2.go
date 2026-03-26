package ui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	e9saws "github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/views"
)

// --- EC2 ---

func (a App) openEC2Instances(filter string) (App, tea.Cmd) {
	a.mode = modeEC2
	a.state = viewEC2Instances
	a.ec2InstancesView = views.NewEC2Instances()
	a.ec2InstancesView = a.ec2InstancesView.SetSize(a.width-3, a.height-6)
	a.loading = true
	client := a.client
	return a, func() tea.Msg {
		instances, err := client.ListEC2Instances(context.Background(), filter)
		if err != nil {
			return errMsg{err}
		}
		return ec2InstancesLoadedMsg{instances}
	}
}

func (a App) openEC2Detail() (App, tea.Cmd) {
	inst := a.ec2InstancesView.SelectedInstance()
	if inst == nil {
		return a, nil
	}
	a.state = viewEC2Detail
	a.loading = true
	client := a.client
	instanceID := inst.InstanceID
	return a, func() tea.Msg {
		detail, err := client.DescribeEC2Instance(context.Background(), instanceID)
		if err != nil {
			return errMsg{err}
		}
		return ec2DetailLoadedMsg{detail}
	}
}

func (a App) openEC2Console() (App, tea.Cmd) {
	instanceID := a.ec2DetailView.InstanceID()
	if instanceID == "" {
		return a, nil
	}
	a.state = viewEC2Console
	a.ec2ConsoleView = views.NewEC2Console(instanceID)
	a.ec2ConsoleView = a.ec2ConsoleView.SetSize(a.width-3, a.height-6)
	a.loading = true
	client := a.client
	return a, func() tea.Msg {
		output, err := client.GetConsoleOutput(context.Background(), instanceID)
		if err != nil {
			return errMsg{err}
		}
		return ec2ConsoleLoadedMsg{output}
	}
}

func (a App) startEC2SSMSession() (App, tea.Cmd) {
	instanceID := a.ec2DetailView.InstanceID()
	if instanceID == "" {
		return a, nil
	}
	state := a.ec2DetailView.InstanceState()
	if state != "running" {
		a.err = fmt.Errorf("instance must be running for SSM session (current state: %s)", state)
		return a, nil
	}
	client := a.client
	return a, func() tea.Msg {
		pluginPath, err := e9saws.SessionManagerPluginPath()
		if err != nil {
			return errMsg{err}
		}
		session, err := client.StartSSMSession(context.Background(), instanceID)
		if err != nil {
			return errMsg{err}
		}
		args, err := session.BuildSSMPluginArgs()
		if err != nil {
			return errMsg{err}
		}
		return execSessionReadyMsg{pluginPath: pluginPath, args: args}
	}
}

func (a App) startEC2Instance() (App, tea.Cmd) {
	instanceID := a.ec2DetailView.InstanceID()
	a.confirm = NewConfirm(ConfirmEC2Start, fmt.Sprintf("Start instance %s?", instanceID))
	return a, nil
}

func (a App) stopEC2Instance() (App, tea.Cmd) {
	instanceID := a.ec2DetailView.InstanceID()
	a.confirm = NewConfirm(ConfirmEC2Stop, fmt.Sprintf("Stop instance %s?", instanceID))
	return a, nil
}

func (a App) rebootEC2Instance() (App, tea.Cmd) {
	instanceID := a.ec2DetailView.InstanceID()
	a.confirm = NewConfirm(ConfirmEC2Reboot, fmt.Sprintf("Reboot instance %s?", instanceID))
	return a, nil
}

func (a App) terminateEC2Instance() (App, tea.Cmd) {
	instanceID := a.ec2DetailView.InstanceID()
	a.confirm = NewConfirm(ConfirmEC2Terminate,
		fmt.Sprintf("TERMINATE instance %s? This cannot be undone!", instanceID))
	return a, nil
}

func (a App) doStartEC2() tea.Cmd {
	client := a.client
	instanceID := a.ec2DetailView.InstanceID()
	return func() tea.Msg {
		err := client.StartEC2Instance(context.Background(), instanceID)
		if err != nil {
			return errMsg{err}
		}
		return ec2ActionDoneMsg{fmt.Sprintf("Starting instance %s", instanceID)}
	}
}

func (a App) doStopEC2() tea.Cmd {
	client := a.client
	instanceID := a.ec2DetailView.InstanceID()
	return func() tea.Msg {
		err := client.StopEC2Instance(context.Background(), instanceID)
		if err != nil {
			return errMsg{err}
		}
		return ec2ActionDoneMsg{fmt.Sprintf("Stopping instance %s", instanceID)}
	}
}

func (a App) doRebootEC2() tea.Cmd {
	client := a.client
	instanceID := a.ec2DetailView.InstanceID()
	return func() tea.Msg {
		err := client.RebootEC2Instance(context.Background(), instanceID)
		if err != nil {
			return errMsg{err}
		}
		return ec2ActionDoneMsg{fmt.Sprintf("Rebooting instance %s", instanceID)}
	}
}

func (a App) doTerminateEC2() tea.Cmd {
	client := a.client
	instanceID := a.ec2DetailView.InstanceID()
	return func() tea.Msg {
		err := client.TerminateEC2Instance(context.Background(), instanceID)
		if err != nil {
			return errMsg{err}
		}
		return ec2ActionDoneMsg{fmt.Sprintf("Terminating instance %s", instanceID)}
	}
}

func (a App) refreshEC2Detail() tea.Cmd {
	instanceID := a.ec2DetailView.InstanceID()
	if instanceID == "" {
		return nil
	}
	client := a.client
	return func() tea.Msg {
		detail, err := client.DescribeEC2Instance(context.Background(), instanceID)
		if err != nil {
			return errMsg{err}
		}
		return ec2DetailLoadedMsg{detail}
	}
}

func (a App) refreshEC2Instances() tea.Cmd {
	client := a.client
	return func() tea.Msg {
		instances, err := client.ListEC2Instances(context.Background(), "")
		if err != nil {
			return errMsg{err}
		}
		return ec2InstancesLoadedMsg{instances}
	}
}

func (a App) handleEC2Action(msg ec2ActionDoneMsg) (App, tea.Cmd) {
	a.flashMessage = msg.message
	a.flashExpiry = time.Now().Add(5 * time.Second)
	a.loading = false
	if a.state == viewEC2Detail {
		return a, a.refreshEC2Detail()
	}
	return a, nil
}
