//go:build gui

package gui

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

func TestFilterRoute53ZonesMatchesNameCommentAndID(t *testing.T) {
	zones := []model.Route53Zone{
		{ID: "ZPUBLIC", Name: "example.com.", Comment: "production"},
		{ID: "ZPRIVATE", Name: "internal.example.", Comment: "private services"},
	}
	for query, want := range map[string]string{"EXAMPLE.COM": "ZPUBLIC", "private services": "ZPRIVATE", "zpublic": "ZPUBLIC"} {
		filtered := filterRoute53Zones(zones, query)
		if len(filtered) != 1 || filtered[0].ID != want {
			t.Fatalf("filterRoute53Zones(%q) = %#v, want %s", query, filtered, want)
		}
	}
}

func TestFormatRoute53ZoneIncludesVisibilityAndCounts(t *testing.T) {
	detail := formatRoute53Zone(model.Route53Zone{ID: "Z123", Name: "internal.example.", Private: true, RecordCount: 12})
	for _, want := range []string{"internal.example.", "Z123", "Private", "12"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("formatRoute53Zone() missing %q in %q", want, detail)
		}
	}
}

func TestFilterRoute53RecordsMatchesValuesAliasesAndRouting(t *testing.T) {
	records := []model.Route53Record{
		{Name: "api.example.", Type: "A", Values: []string{"192.0.2.1"}},
		{Name: "www.example.", Type: "A", AliasTarget: "dualstack.lb.example.", RoutingPolicy: "Latency", SetIdentifier: "east"},
	}
	for query, want := range map[string]string{"192.0.2.1": "api.example.", "dualstack": "www.example.", "latency": "www.example.", "east": "www.example."} {
		filtered := filterRoute53Records(records, query)
		if len(filtered) != 1 || filtered[0].Name != want {
			t.Fatalf("filterRoute53Records(%q) = %#v, want %s", query, filtered, want)
		}
	}
}

func TestRoute53RecordIdentityDistinguishesRoutingRecords(t *testing.T) {
	left := model.Route53Record{Name: "api.example.", Type: "A", SetIdentifier: "east"}
	right := model.Route53Record{Name: "api.example.", Type: "A", SetIdentifier: "west"}
	if route53RecordIdentity(left) == route53RecordIdentity(right) {
		t.Fatal("routing record identities collided")
	}
}

func TestFormatRoute53RecordIncludesAliasAndDNSAnswer(t *testing.T) {
	detail := formatRoute53Record(
		model.Route53Zone{Name: "example."},
		model.Route53Record{Name: "www.example.", Type: "A", AliasTarget: "lb.example.", AliasZoneID: "ZLB", EvaluateTargetHealth: true},
		&model.Route53DNSAnswer{ResponseCode: "NOERROR", Nameserver: "ns.example.", Protocol: "UDP", RecordData: []string{"192.0.2.10"}},
	)
	for _, want := range []string{"www.example.", "lb.example.", "ZLB", "true", "NOERROR", "192.0.2.10"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("formatRoute53Record() missing %q in %q", want, detail)
		}
	}
}
