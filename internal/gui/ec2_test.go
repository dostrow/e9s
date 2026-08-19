//go:build gui

package gui

import (
	"strings"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

func TestFormatEC2Age(t *testing.T) {
	now := time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC)
	for name, test := range map[string]struct {
		launched time.Time
		want     string
	}{
		"unknown": {want: "—"},
		"minutes": {launched: now.Add(-15 * time.Minute), want: "15m"},
		"hours":   {launched: now.Add(-6 * time.Hour), want: "6h"},
		"days":    {launched: now.Add(-48 * time.Hour), want: "2d"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := formatEC2Age(test.launched, now); got != test.want {
				t.Fatalf("formatEC2Age() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestFormatEC2DetailIncludesInfrastructureSections(t *testing.T) {
	detail := model.EC2InstanceDetail{
		EC2Instance: model.EC2Instance{
			InstanceID: "i-123", Name: "api", State: "running", Type: "m7g.large", AZ: "us-east-2a",
			PrivateIP: "10.0.0.4", VpcID: "vpc-1", SubnetID: "subnet-1", AMI: "ami-1",
			SecurityGroups: []model.EC2SecurityGroupRef{{Name: "api", ID: "sg-1"}},
			Tags:           map[string]string{"Team": "platform"},
		},
		Architecture: "arm64", RootDeviceName: "/dev/xvda", RootDeviceType: "ebs",
		Volumes:            []model.EC2Volume{{DeviceName: "/dev/xvda", VolumeID: "vol-1", Size: 20, VolumeType: "gp3", State: "attached"}},
		SecurityGroupRules: []model.EC2SGRule{{Direction: "inbound", Protocol: "tcp", PortRange: "443", Source: "10.0.0.0/8"}},
	}
	got := formatEC2Detail(detail)
	for _, want := range []string{"i-123", "NETWORKING", "vpc-1", "SECURITY GROUPS", "sg-1", "443", "VOLUMES", "vol-1", "20 GiB", "TAGS", "platform"} {
		if !strings.Contains(got, want) {
			t.Fatalf("detail missing %q:\n%s", want, got)
		}
	}
}

func TestEC2ActionAvailabilityTracksInstanceState(t *testing.T) {
	tests := []struct {
		action string
		state  string
		want   bool
	}{
		{action: "start", state: "stopped", want: true},
		{action: "start", state: "running"},
		{action: "stop", state: "running", want: true},
		{action: "reboot", state: "running", want: true},
		{action: "terminate", state: "stopped", want: true},
		{action: "terminate", state: "terminated"},
	}
	for _, test := range tests {
		if got := ec2ActionAllowed(test.action, test.state); got != test.want {
			t.Errorf("ec2ActionAllowed(%q, %q) = %t, want %t", test.action, test.state, got, test.want)
		}
	}
}

func TestFormatEC2ConsoleOutputNormalizesLineEndings(t *testing.T) {
	instance := model.EC2InstanceDetail{EC2Instance: model.EC2Instance{InstanceID: "i-123", Name: "api", State: "running"}}
	got := formatEC2ConsoleOutput(instance, "booting\r\nready\r\n")
	if strings.Contains(got, "\r") || !strings.Contains(got, "booting\nready") {
		t.Fatalf("formatEC2ConsoleOutput() = %q", got)
	}
}
