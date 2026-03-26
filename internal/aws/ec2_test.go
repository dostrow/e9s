package aws

import (
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestStateOrder(t *testing.T) {
	tests := []struct {
		state string
		want  int
	}{
		{"running", 0},
		{"pending", 1},
		{"stopping", 2},
		{"stopped", 3},
		{"shutting-down", 4},
		{"terminated", 5},
		{"unknown-state", 9},
		{"", 9},
	}
	for _, tt := range tests {
		got := stateOrder(tt.state)
		if got != tt.want {
			t.Errorf("stateOrder(%q) = %d, want %d", tt.state, got, tt.want)
		}
	}
}

func TestStateOrder_RunningBeforeStopped(t *testing.T) {
	if stateOrder("running") >= stateOrder("stopped") {
		t.Error("running should sort before stopped")
	}
}

func TestSgRulesFromPerm_BasicInbound(t *testing.T) {
	from := int32(80)
	to := int32(80)
	cidr := "0.0.0.0/0"
	proto := "tcp"
	perm := ec2types.IpPermission{
		IpProtocol: &proto,
		FromPort:   &from,
		ToPort:     &to,
		IpRanges: []ec2types.IpRange{
			{CidrIp: &cidr},
		},
	}
	rules := sgRulesFromPerm("inbound", perm)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	r := rules[0]
	if r.Direction != "inbound" {
		t.Errorf("Direction = %q, want %q", r.Direction, "inbound")
	}
	if r.Protocol != "tcp" {
		t.Errorf("Protocol = %q, want %q", r.Protocol, "tcp")
	}
	if r.PortRange != "80" {
		t.Errorf("PortRange = %q, want %q", r.PortRange, "80")
	}
	if r.Source != "0.0.0.0/0" {
		t.Errorf("Source = %q, want %q", r.Source, "0.0.0.0/0")
	}
}

func TestSgRulesFromPerm_PortRange(t *testing.T) {
	from := int32(8080)
	to := int32(8090)
	cidr := "10.0.0.0/8"
	proto := "tcp"
	perm := ec2types.IpPermission{
		IpProtocol: &proto,
		FromPort:   &from,
		ToPort:     &to,
		IpRanges: []ec2types.IpRange{
			{CidrIp: &cidr},
		},
	}
	rules := sgRulesFromPerm("inbound", perm)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].PortRange != "8080-8090" {
		t.Errorf("PortRange = %q, want %q", rules[0].PortRange, "8080-8090")
	}
}

func TestSgRulesFromPerm_AllProtocol(t *testing.T) {
	proto := "-1"
	cidr := "0.0.0.0/0"
	perm := ec2types.IpPermission{
		IpProtocol: &proto,
		IpRanges: []ec2types.IpRange{
			{CidrIp: &cidr},
		},
	}
	rules := sgRulesFromPerm("outbound", perm)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Protocol != "All" {
		t.Errorf("Protocol = %q, want %q", rules[0].Protocol, "All")
	}
	if rules[0].PortRange != "All" {
		t.Errorf("PortRange = %q, want %q", rules[0].PortRange, "All")
	}
}

func TestSgRulesFromPerm_NoSources_WildcardFallback(t *testing.T) {
	proto := "tcp"
	from := int32(443)
	to := int32(443)
	perm := ec2types.IpPermission{
		IpProtocol: &proto,
		FromPort:   &from,
		ToPort:     &to,
		// No IpRanges, Ipv6Ranges, or UserIdGroupPairs
	}
	rules := sgRulesFromPerm("inbound", perm)
	if len(rules) != 1 {
		t.Fatalf("expected wildcard fallback rule, got %d rules", len(rules))
	}
	if rules[0].Source != "*" {
		t.Errorf("Source = %q, want %q", rules[0].Source, "*")
	}
}

func TestSgRulesFromPerm_MultipleRanges(t *testing.T) {
	proto := "tcp"
	from := int32(22)
	to := int32(22)
	cidr1 := "10.0.0.0/8"
	cidr2 := "192.168.0.0/16"
	perm := ec2types.IpPermission{
		IpProtocol: &proto,
		FromPort:   &from,
		ToPort:     &to,
		IpRanges: []ec2types.IpRange{
			{CidrIp: &cidr1},
			{CidrIp: &cidr2},
		},
	}
	rules := sgRulesFromPerm("inbound", perm)
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
}
