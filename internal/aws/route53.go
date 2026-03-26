package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// R53Zone represents a Route53 hosted zone.
type R53Zone struct {
	ID          string
	Name        string
	Private     bool
	RecordCount int64
	Comment     string
}

// R53Record represents a Route53 resource record set.
type R53Record struct {
	Name          string
	Type          string
	TTL           int64
	Values        []string
	AliasTarget   string // DNS name of alias target, empty if not an alias
	AliasZoneID   string
	RoutingPolicy string // Simple, Weighted, Latency, Failover, Geolocation, MultiValue
	SetIdentifier string
	Weight        int64
	Region        string
	Failover      string
	HealthCheckID string
}

// R53DNSAnswer represents the result of a TestDNSAnswer call.
type R53DNSAnswer struct {
	RecordName   string
	RecordType   string
	ResponseCode string
	Nameserver   string
	Protocol     string
	RecordData   []string
}

// ListR53Zones returns all hosted zones, optionally filtered by name.
func (c *Client) ListR53Zones(ctx context.Context, filter string) ([]R53Zone, error) {
	var zones []R53Zone
	lf := strings.ToLower(filter)

	paginator := route53.NewListHostedZonesPaginator(c.Route53, &route53.ListHostedZonesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, z := range page.HostedZones {
			zone := zoneFromSDK(z)
			if lf != "" && !strings.Contains(strings.ToLower(zone.Name), lf) {
				continue
			}
			zones = append(zones, zone)
		}
	}
	return zones, nil
}

// ListR53Records returns all record sets for a hosted zone.
func (c *Client) ListR53Records(ctx context.Context, zoneID string) ([]R53Record, error) {
	var records []R53Record

	paginator := route53.NewListResourceRecordSetsPaginator(c.Route53,
		&route53.ListResourceRecordSetsInput{
			HostedZoneId: &zoneID,
		})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range page.ResourceRecordSets {
			records = append(records, recordFromSDK(r))
		}
	}
	return records, nil
}

// TestR53DNS tests DNS resolution for a record via the Route53 API.
func (c *Client) TestR53DNS(ctx context.Context, zoneID, recordName, recordType string) (*R53DNSAnswer, error) {
	rrType := r53types.RRType(recordType)
	out, err := c.Route53.TestDNSAnswer(ctx, &route53.TestDNSAnswerInput{
		HostedZoneId: &zoneID,
		RecordName:   &recordName,
		RecordType:   rrType,
	})
	if err != nil {
		return nil, err
	}
	answer := &R53DNSAnswer{
		RecordName:   derefStrAws(out.RecordName),
		RecordType:   string(out.RecordType),
		ResponseCode: derefStrAws(out.ResponseCode),
		Nameserver:   derefStrAws(out.Nameserver),
		Protocol:     derefStrAws(out.Protocol),
		RecordData:   out.RecordData,
	}
	return answer, nil
}

// CreateR53Record creates a new record set in a hosted zone.
func (c *Client) CreateR53Record(ctx context.Context, zoneID string, record R53Record) error {
	change := buildChange(r53types.ChangeActionCreate, record)
	_, err := c.Route53.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: &zoneID,
		ChangeBatch:  &r53types.ChangeBatch{Changes: []r53types.Change{change}},
	})
	return err
}

// UpdateR53Record updates an existing record set.
func (c *Client) UpdateR53Record(ctx context.Context, zoneID string, record R53Record) error {
	change := buildChange(r53types.ChangeActionUpsert, record)
	_, err := c.Route53.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: &zoneID,
		ChangeBatch:  &r53types.ChangeBatch{Changes: []r53types.Change{change}},
	})
	return err
}

// DeleteR53Record deletes a record set.
func (c *Client) DeleteR53Record(ctx context.Context, zoneID string, record R53Record) error {
	change := buildChange(r53types.ChangeActionDelete, record)
	_, err := c.Route53.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: &zoneID,
		ChangeBatch:  &r53types.ChangeBatch{Changes: []r53types.Change{change}},
	})
	return err
}

// R53RecordTemplate represents the JSON structure for editing records in $EDITOR.
type R53RecordTemplate struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	TTL    int64    `json:"ttl,omitempty"`
	Values []string `json:"values,omitempty"`
	Alias  *struct {
		DNSName    string `json:"dnsName"`
		HostedZone string `json:"hostedZoneId"`
	} `json:"alias,omitempty"`
}

// BuildR53RecordTemplate creates a JSON template for creating/editing a record.
func BuildR53RecordTemplate(record *R53Record) string {
	tmpl := R53RecordTemplate{
		Type: "A",
		TTL:  300,
	}
	if record != nil {
		tmpl.Name = record.Name
		tmpl.Type = record.Type
		tmpl.TTL = record.TTL
		tmpl.Values = record.Values
		if record.AliasTarget != "" {
			tmpl.Alias = &struct {
				DNSName    string `json:"dnsName"`
				HostedZone string `json:"hostedZoneId"`
			}{
				DNSName:    record.AliasTarget,
				HostedZone: record.AliasZoneID,
			}
			tmpl.TTL = 0
		}
	}
	b, _ := json.MarshalIndent(tmpl, "", "  ")
	return string(b)
}

// ParseR53RecordTemplate parses the edited JSON template back into a record.
func ParseR53RecordTemplate(data string) (*R53Record, error) {
	var tmpl R53RecordTemplate
	if err := json.Unmarshal([]byte(data), &tmpl); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if tmpl.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if tmpl.Type == "" {
		return nil, fmt.Errorf("type is required")
	}
	r := &R53Record{
		Name:   tmpl.Name,
		Type:   tmpl.Type,
		TTL:    tmpl.TTL,
		Values: tmpl.Values,
	}
	if tmpl.Alias != nil && tmpl.Alias.DNSName != "" {
		r.AliasTarget = tmpl.Alias.DNSName
		r.AliasZoneID = tmpl.Alias.HostedZone
	}
	return r, nil
}

func buildChange(action r53types.ChangeAction, record R53Record) r53types.Change {
	rrType := r53types.RRType(record.Type)
	rrs := &r53types.ResourceRecordSet{
		Name: &record.Name,
		Type: rrType,
	}

	if record.AliasTarget != "" {
		evalTarget := false
		rrs.AliasTarget = &r53types.AliasTarget{
			DNSName:              &record.AliasTarget,
			HostedZoneId:         &record.AliasZoneID,
			EvaluateTargetHealth: evalTarget,
		}
	} else {
		if record.TTL > 0 {
			rrs.TTL = &record.TTL
		}
		for _, v := range record.Values {
			rrs.ResourceRecords = append(rrs.ResourceRecords,
				r53types.ResourceRecord{Value: strPtr(v)})
		}
	}

	if record.SetIdentifier != "" {
		rrs.SetIdentifier = &record.SetIdentifier
	}
	if record.Weight > 0 {
		rrs.Weight = &record.Weight
	}

	return r53types.Change{
		Action:            action,
		ResourceRecordSet: rrs,
	}
}

func strPtr(s string) *string { return &s }

func zoneFromSDK(z r53types.HostedZone) R53Zone {
	zone := R53Zone{
		ID:   derefStrAws(z.Id),
		Name: derefStrAws(z.Name),
	}
	if z.ResourceRecordSetCount != nil {
		zone.RecordCount = *z.ResourceRecordSetCount
	}
	if z.Config != nil {
		zone.Private = z.Config.PrivateZone
		zone.Comment = derefStrAws(z.Config.Comment)
	}
	// Clean up zone ID (remove /hostedzone/ prefix)
	zone.ID = strings.TrimPrefix(zone.ID, "/hostedzone/")
	return zone
}

func recordFromSDK(r r53types.ResourceRecordSet) R53Record {
	rec := R53Record{
		Name:          derefStrAws(r.Name),
		Type:          string(r.Type),
		HealthCheckID: derefStrAws(r.HealthCheckId),
		SetIdentifier: derefStrAws(r.SetIdentifier),
	}
	if r.TTL != nil {
		rec.TTL = *r.TTL
	}
	for _, rr := range r.ResourceRecords {
		if rr.Value != nil {
			rec.Values = append(rec.Values, *rr.Value)
		}
	}
	if r.AliasTarget != nil {
		rec.AliasTarget = derefStrAws(r.AliasTarget.DNSName)
		rec.AliasZoneID = derefStrAws(r.AliasTarget.HostedZoneId)
	}

	// Determine routing policy
	rec.RoutingPolicy = "Simple"
	if r.Weight != nil {
		rec.RoutingPolicy = "Weighted"
		rec.Weight = *r.Weight
	}
	if r.Region != "" {
		rec.RoutingPolicy = "Latency"
		rec.Region = string(r.Region)
	}
	if r.Failover != "" {
		rec.RoutingPolicy = "Failover"
		rec.Failover = string(r.Failover)
	}
	if r.GeoLocation != nil {
		rec.RoutingPolicy = "Geolocation"
	}
	if r.MultiValueAnswer != nil && *r.MultiValueAnswer {
		rec.RoutingPolicy = "MultiValue"
	}

	return rec
}
