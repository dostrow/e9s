package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/dostrow/e9s/internal/model"
)

// Route53API is the low-level DNS behavior shared by both frontends.
type Route53API interface {
	ListR53Zones(context.Context, string) ([]model.Route53Zone, error)
	ListR53Records(context.Context, string) ([]model.Route53Record, error)
	TestR53DNS(context.Context, string, string, string) (*model.Route53DNSAnswer, error)
	CreateR53Record(context.Context, string, model.Route53Record) error
	UpdateR53Record(context.Context, string, model.Route53Record) error
	DeleteR53Record(context.Context, string, model.Route53Record) error
}

type Route53 struct{ api Route53API }

func NewRoute53(api Route53API) *Route53 { return &Route53{api: api} }

func (s *Route53) Zones(ctx context.Context, filter string) ([]model.Route53Zone, error) {
	zones, err := s.api.ListR53Zones(ctx, strings.TrimSpace(filter))
	if err != nil {
		return nil, fmt.Errorf("list Route53 hosted zones: %w", err)
	}
	sort.SliceStable(zones, func(i, j int) bool {
		return strings.ToLower(zones[i].Name) < strings.ToLower(zones[j].Name)
	})
	return zones, nil
}

func (s *Route53) Records(ctx context.Context, zoneID string) ([]model.Route53Record, error) {
	zoneID = normalizeRoute53ZoneID(zoneID)
	if zoneID == "" {
		return nil, fmt.Errorf("list Route53 records: hosted zone ID is required")
	}
	records, err := s.api.ListR53Records(ctx, zoneID)
	if err != nil {
		return nil, fmt.Errorf("list Route53 records for hosted zone %q: %w", zoneID, err)
	}
	sort.SliceStable(records, func(i, j int) bool {
		left, right := strings.ToLower(records[i].Name), strings.ToLower(records[j].Name)
		if left != right {
			return left < right
		}
		if records[i].Type != records[j].Type {
			return records[i].Type < records[j].Type
		}
		return records[i].SetIdentifier < records[j].SetIdentifier
	})
	return records, nil
}

func (s *Route53) TestDNS(ctx context.Context, zoneID string, record model.Route53Record) (*model.Route53DNSAnswer, error) {
	zoneID = normalizeRoute53ZoneID(zoneID)
	if zoneID == "" || strings.TrimSpace(record.Name) == "" || strings.TrimSpace(record.Type) == "" {
		return nil, fmt.Errorf("test Route53 DNS answer: hosted zone, record name, and type are required")
	}
	answer, err := s.api.TestR53DNS(ctx, zoneID, record.Name, record.Type)
	if err != nil {
		return nil, fmt.Errorf("test Route53 %s record %q: %w", record.Type, record.Name, err)
	}
	if answer == nil {
		return nil, fmt.Errorf("test Route53 %s record %q: no answer was returned", record.Type, record.Name)
	}
	return answer, nil
}

func (s *Route53) Create(ctx context.Context, zoneID string, record model.Route53Record) error {
	zoneID = normalizeRoute53ZoneID(zoneID)
	if err := validateRoute53Mutation(zoneID, record); err != nil {
		return fmt.Errorf("create Route53 record: %w", err)
	}
	if err := s.api.CreateR53Record(ctx, zoneID, record); err != nil {
		return fmt.Errorf("create Route53 %s record %q: %w", record.Type, record.Name, err)
	}
	return nil
}

func (s *Route53) Update(ctx context.Context, zoneID string, record model.Route53Record) error {
	zoneID = normalizeRoute53ZoneID(zoneID)
	if err := validateRoute53Mutation(zoneID, record); err != nil {
		return fmt.Errorf("update Route53 record: %w", err)
	}
	if err := s.api.UpdateR53Record(ctx, zoneID, record); err != nil {
		return fmt.Errorf("update Route53 %s record %q: %w", record.Type, record.Name, err)
	}
	return nil
}

func (s *Route53) Delete(ctx context.Context, zoneID string, record model.Route53Record) error {
	zoneID = normalizeRoute53ZoneID(zoneID)
	if zoneID == "" || strings.TrimSpace(record.Name) == "" || strings.TrimSpace(record.Type) == "" {
		return fmt.Errorf("delete Route53 record: hosted zone, record name, and type are required")
	}
	if record.Type == "NS" || record.Type == "SOA" {
		return fmt.Errorf("delete Route53 record: apex %s records are protected", record.Type)
	}
	if err := s.api.DeleteR53Record(ctx, zoneID, record); err != nil {
		return fmt.Errorf("delete Route53 %s record %q: %w", record.Type, record.Name, err)
	}
	return nil
}

func BuildRoute53RecordTemplate(record *model.Route53Record) string {
	template := model.Route53RecordTemplate{Type: "A", TTL: 300}
	if record != nil {
		template.Name = record.Name
		template.Type = record.Type
		template.TTL = record.TTL
		template.Values = append([]string(nil), record.Values...)
		template.SetIdentifier = record.SetIdentifier
		template.Weight = record.Weight
		template.Region = record.Region
		template.Failover = record.Failover
		template.GeoContinentCode = record.GeoContinentCode
		template.GeoCountryCode = record.GeoCountryCode
		template.GeoSubdivisionCode = record.GeoSubdivisionCode
		template.MultiValue = record.MultiValue
		template.HealthCheckID = record.HealthCheckID
		if record.AliasTarget != "" {
			template.Alias = &struct {
				DNSName              string `json:"dnsName"`
				HostedZoneID         string `json:"hostedZoneId"`
				EvaluateTargetHealth bool   `json:"evaluateTargetHealth,omitempty"`
			}{record.AliasTarget, record.AliasZoneID, record.EvaluateTargetHealth}
			template.TTL = 0
		}
	}
	data, _ := json.MarshalIndent(template, "", "  ")
	return string(data)
}

func ParseRoute53RecordTemplate(document string) (*model.Route53Record, error) {
	var template model.Route53RecordTemplate
	if err := json.Unmarshal([]byte(document), &template); err != nil {
		return nil, fmt.Errorf("parse Route53 record template: %w", err)
	}
	record := &model.Route53Record{
		Name: template.Name, Type: strings.ToUpper(template.Type), TTL: template.TTL,
		Values: append([]string(nil), template.Values...), SetIdentifier: template.SetIdentifier,
		Weight: template.Weight, Region: template.Region, Failover: template.Failover,
		GeoContinentCode: template.GeoContinentCode, GeoCountryCode: template.GeoCountryCode,
		GeoSubdivisionCode: template.GeoSubdivisionCode, MultiValue: template.MultiValue,
		HealthCheckID: template.HealthCheckID,
	}
	if template.Alias != nil {
		record.AliasTarget = template.Alias.DNSName
		record.AliasZoneID = template.Alias.HostedZoneID
		record.EvaluateTargetHealth = template.Alias.EvaluateTargetHealth
	}
	if err := validateRoute53Mutation("template", *record); err != nil {
		return nil, fmt.Errorf("parse Route53 record template: %w", err)
	}
	return record, nil
}

func validateRoute53Mutation(zoneID string, record model.Route53Record) error {
	if zoneID == "" {
		return fmt.Errorf("hosted zone ID is required")
	}
	if strings.TrimSpace(record.Name) == "" {
		return fmt.Errorf("record name is required")
	}
	if strings.TrimSpace(record.Type) == "" {
		return fmt.Errorf("record type is required")
	}
	if record.AliasTarget != "" {
		if record.AliasZoneID == "" {
			return fmt.Errorf("alias hosted zone ID is required")
		}
		if len(record.Values) > 0 {
			return fmt.Errorf("alias records cannot also contain values")
		}
		return nil
	}
	if len(record.Values) == 0 {
		return fmt.Errorf("at least one record value is required")
	}
	if record.TTL <= 0 {
		return fmt.Errorf("TTL must be greater than zero for non-alias records")
	}
	return nil
}

func normalizeRoute53ZoneID(zoneID string) string {
	return strings.TrimPrefix(strings.TrimSpace(zoneID), "/hostedzone/")
}
