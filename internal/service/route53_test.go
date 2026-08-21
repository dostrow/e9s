package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

type fakeRoute53API struct {
	zones   []model.Route53Zone
	records []model.Route53Record
	answer  *model.Route53DNSAnswer
	err     error
	action  string
	zoneID  string
	record  model.Route53Record
}

func (f *fakeRoute53API) ListR53Zones(context.Context, string) ([]model.Route53Zone, error) {
	return append([]model.Route53Zone(nil), f.zones...), f.err
}
func (f *fakeRoute53API) ListR53Records(context.Context, string) ([]model.Route53Record, error) {
	return append([]model.Route53Record(nil), f.records...), f.err
}
func (f *fakeRoute53API) TestR53DNS(context.Context, string, string, string) (*model.Route53DNSAnswer, error) {
	return f.answer, f.err
}
func (f *fakeRoute53API) CreateR53Record(_ context.Context, zoneID string, record model.Route53Record) error {
	f.action, f.zoneID, f.record = "create", zoneID, record
	return f.err
}
func (f *fakeRoute53API) UpdateR53Record(_ context.Context, zoneID string, record model.Route53Record) error {
	f.action, f.zoneID, f.record = "update", zoneID, record
	return f.err
}
func (f *fakeRoute53API) DeleteR53Record(_ context.Context, zoneID string, record model.Route53Record) error {
	f.action, f.zoneID, f.record = "delete", zoneID, record
	return f.err
}

func TestRoute53ListsSortAndWrapErrors(t *testing.T) {
	api := &fakeRoute53API{
		zones: []model.Route53Zone{{Name: "z.example."}, {Name: "A.example."}},
		records: []model.Route53Record{
			{Name: "z.example.", Type: "TXT"}, {Name: "a.example.", Type: "TXT"}, {Name: "a.example.", Type: "A"},
		},
	}
	service := NewRoute53(api)
	zones, err := service.Zones(context.Background(), "")
	if err != nil || zones[0].Name != "A.example." {
		t.Fatalf("Zones() = %#v, %v", zones, err)
	}
	records, err := service.Records(context.Background(), "/hostedzone/Z123")
	if err != nil || records[0].Type != "A" || records[2].Name != "z.example." {
		t.Fatalf("Records() = %#v, %v", records, err)
	}
	api.err = errors.New("denied")
	if _, err := service.Zones(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "hosted zones") {
		t.Fatalf("Zones() error = %v", err)
	}
}

func TestRoute53TemplateRoundTripsRoutingAndAlias(t *testing.T) {
	record := model.Route53Record{
		Name: "app.example.", Type: "A", AliasTarget: "lb.example.", AliasZoneID: "ZLB",
		EvaluateTargetHealth: true, SetIdentifier: "east", Region: "us-east-2", HealthCheckID: "hc-1",
	}
	document := BuildRoute53RecordTemplate(&record)
	parsed, err := ParseRoute53RecordTemplate(document)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.AliasTarget != record.AliasTarget || parsed.AliasZoneID != record.AliasZoneID || !parsed.EvaluateTargetHealth || parsed.Region != record.Region {
		t.Fatalf("round trip = %#v", parsed)
	}
}

func TestRoute53MutationsValidateAndProtectApexRecords(t *testing.T) {
	api := &fakeRoute53API{}
	service := NewRoute53(api)
	valid := model.Route53Record{Name: "app.example.", Type: "A", TTL: 60, Values: []string{"192.0.2.1"}}
	if err := service.Create(context.Background(), "/hostedzone/Z123", valid); err != nil {
		t.Fatal(err)
	}
	if api.action != "create" || api.zoneID != "Z123" {
		t.Fatalf("create routed as action=%q zone=%q", api.action, api.zoneID)
	}
	if err := service.Update(context.Background(), "Z123", model.Route53Record{Name: "bad", Type: "A"}); err == nil {
		t.Fatal("Update() accepted an empty record value")
	}
	if err := service.Delete(context.Background(), "Z123", model.Route53Record{Name: "example.", Type: "SOA"}); err == nil {
		t.Fatal("Delete() accepted a protected SOA record")
	}
}
