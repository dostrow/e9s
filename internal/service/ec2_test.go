package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

type fakeEC2API struct {
	instances []model.EC2Instance
	detail    *model.EC2InstanceDetail
	console   string
	session   *model.ExecSession
	err       error
	action    string
	id        string
	metrics   *model.MetricSnapshot
}

func (f *fakeEC2API) ListEC2Instances(context.Context, string) ([]model.EC2Instance, error) {
	return append([]model.EC2Instance(nil), f.instances...), f.err
}
func (f *fakeEC2API) DescribeEC2Instance(_ context.Context, id string) (*model.EC2InstanceDetail, error) {
	f.id = id
	return f.detail, f.err
}
func (f *fakeEC2API) GetConsoleOutput(_ context.Context, id string) (string, error) {
	f.id = id
	return f.console, f.err
}
func (f *fakeEC2API) StartEC2Instance(_ context.Context, id string) error {
	f.action, f.id = "start", id
	return f.err
}
func (f *fakeEC2API) StopEC2Instance(_ context.Context, id string) error {
	f.action, f.id = "stop", id
	return f.err
}
func (f *fakeEC2API) RebootEC2Instance(_ context.Context, id string) error {
	f.action, f.id = "reboot", id
	return f.err
}
func (f *fakeEC2API) TerminateEC2Instance(_ context.Context, id string) error {
	f.action, f.id = "terminate", id
	return f.err
}
func (f *fakeEC2API) StartSSMSession(_ context.Context, id string) (*model.ExecSession, error) {
	f.action, f.id = "session", id
	return f.session, f.err
}
func (f *fakeEC2API) GetEC2Metrics(_ context.Context, id string, _ time.Duration) (*model.MetricSnapshot, error) {
	f.id = id
	return f.metrics, f.err
}

func TestEC2MetricsValidatesAndWrapsErrors(t *testing.T) {
	want := &model.MetricSnapshot{Period: time.Minute}
	api := &fakeEC2API{metrics: want}
	got, err := NewEC2(api).Metrics(context.Background(), " i-1 ", time.Hour)
	if err != nil || got != want || api.id != "i-1" {
		t.Fatalf("Metrics() = %#v, %v; id = %q", got, err, api.id)
	}
	if _, err := NewEC2(api).Metrics(context.Background(), "", time.Hour); err == nil {
		t.Fatal("Metrics() accepted an empty instance ID")
	}
}

func TestEC2ListFiltersLoadedMetadataAndSortsStates(t *testing.T) {
	api := &fakeEC2API{instances: []model.EC2Instance{
		{InstanceID: "i-stop", Name: "worker", State: "stopped", PrivateIP: "10.0.0.2"},
		{InstanceID: "i-run", Name: "api", State: "running", Tags: map[string]string{"Team": "Platform"}},
	}}
	service := NewEC2(api)
	instances, err := service.List(context.Background(), "")
	if err != nil || len(instances) != 2 || instances[0].InstanceID != "i-run" {
		t.Fatalf("List() = %#v, %v", instances, err)
	}
	filtered, err := service.List(context.Background(), "platform")
	if err != nil || len(filtered) != 1 || filtered[0].InstanceID != "i-run" {
		t.Fatalf("List(platform) = %#v, %v", filtered, err)
	}
	filtered, err = service.List(context.Background(), "10.0.0.2")
	if err != nil || len(filtered) != 1 || filtered[0].InstanceID != "i-stop" {
		t.Fatalf("List(IP) = %#v, %v", filtered, err)
	}
	filtered = FilterEC2Instances(api.instances, "stopped")
	if len(filtered) != 1 || filtered[0].InstanceID != "i-stop" {
		t.Fatalf("FilterEC2Instances() = %#v", filtered)
	}
}

func TestEC2DetailSortsNestedResources(t *testing.T) {
	api := &fakeEC2API{detail: &model.EC2InstanceDetail{
		EC2Instance:        model.EC2Instance{InstanceID: "i-1", SecurityGroups: []model.EC2SecurityGroupRef{{Name: "z"}, {Name: "a"}}},
		Volumes:            []model.EC2Volume{{DeviceName: "/dev/xvdb"}, {DeviceName: "/dev/xvda"}},
		SecurityGroupRules: []model.EC2SGRule{{Direction: "outbound"}, {Direction: "inbound"}},
	}}
	detail, err := NewEC2(api).Detail(context.Background(), " i-1 ")
	if err != nil {
		t.Fatal(err)
	}
	if detail.SecurityGroups[0].Name != "a" || detail.Volumes[0].DeviceName != "/dev/xvda" || detail.SecurityGroupRules[0].Direction != "inbound" {
		t.Fatalf("Detail() = %#v", detail)
	}
}

func TestEC2LifecycleValidationAndContext(t *testing.T) {
	api := &fakeEC2API{}
	service := NewEC2(api)
	tests := []struct {
		name   string
		state  string
		call   func(context.Context, string, string) error
		action string
	}{
		{name: "start", state: "stopped", call: service.Start, action: "start"},
		{name: "stop", state: "running", call: service.Stop, action: "stop"},
		{name: "reboot", state: "running", call: service.Reboot, action: "reboot"},
		{name: "terminate", state: "stopped", call: service.Terminate, action: "terminate"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(context.Background(), " i-1 ", test.state); err != nil {
				t.Fatal(err)
			}
			if api.action != test.action || api.id != "i-1" {
				t.Fatalf("action = %q, id = %q", api.action, api.id)
			}
		})
	}
	if err := service.Start(context.Background(), "i-1", "running"); err == nil {
		t.Fatal("Start(running) succeeded")
	}
	api.err = errors.New("denied")
	if err := service.Stop(context.Background(), "i-1", "running"); err == nil || !strings.Contains(err.Error(), "stop EC2 instance") {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestEC2PreparesSessionManagerLaunch(t *testing.T) {
	api := &fakeEC2API{session: &model.ExecSession{SessionID: "session", Region: "us-east-2", Target: "i-1"}}
	service := NewEC2(api)
	service.pluginPath = func() (string, error) { return "/usr/bin/session-manager-plugin", nil }
	launch, err := service.PrepareSession(context.Background(), "i-1", "running")
	if err != nil {
		t.Fatal(err)
	}
	if launch.Executable != "/usr/bin/session-manager-plugin" || len(launch.Args) != 6 || api.action != "session" {
		t.Fatalf("PrepareSession() = %#v, action %q", launch, api.action)
	}
	if _, err := service.PrepareSession(context.Background(), "i-1", "stopped"); err == nil {
		t.Fatal("PrepareSession(stopped) succeeded")
	}
}

func TestEC2StateOrder(t *testing.T) {
	states := []string{"running", "pending", "stopping", "stopped", "shutting-down", "terminated", "unknown"}
	got := make([]int, len(states))
	for i, state := range states {
		got[i] = ec2StateOrder(state)
	}
	if want := []int{0, 1, 2, 3, 4, 5, 9}; !reflect.DeepEqual(got, want) {
		t.Fatalf("orders = %#v, want %#v", got, want)
	}
}
