package aws

import (
	"strings"
	"testing"
)

func TestBuildR53RecordTemplate_NewRecord(t *testing.T) {
	tmpl := BuildR53RecordTemplate(nil)
	if !strings.Contains(tmpl, `"type"`) {
		t.Error("template should contain type field")
	}
	if !strings.Contains(tmpl, `"ttl"`) {
		t.Error("template should contain ttl field")
	}
	// Defaults: type A, TTL 300
	if !strings.Contains(tmpl, `"A"`) {
		t.Error("default type should be A")
	}
	if !strings.Contains(tmpl, "300") {
		t.Error("default TTL should be 300")
	}
}

func TestBuildR53RecordTemplate_ExistingRecord(t *testing.T) {
	record := &R53Record{
		Name:   "example.com.",
		Type:   "CNAME",
		TTL:    60,
		Values: []string{"target.example.com."},
	}
	tmpl := BuildR53RecordTemplate(record)
	if !strings.Contains(tmpl, "example.com.") {
		t.Error("template should contain record name")
	}
	if !strings.Contains(tmpl, "CNAME") {
		t.Error("template should contain record type")
	}
	if !strings.Contains(tmpl, "target.example.com.") {
		t.Error("template should contain record value")
	}
}

func TestBuildR53RecordTemplate_AliasRecord(t *testing.T) {
	record := &R53Record{
		Name:        "www.example.com.",
		Type:        "A",
		TTL:         300,
		AliasTarget: "my-lb.us-east-1.elb.amazonaws.com.",
		AliasZoneID: "Z35SXDOTRQ7X7K",
	}
	tmpl := BuildR53RecordTemplate(record)
	if !strings.Contains(tmpl, "alias") {
		t.Error("template should contain alias field for alias record")
	}
	if !strings.Contains(tmpl, "my-lb.us-east-1.elb.amazonaws.com.") {
		t.Error("template should contain alias DNS name")
	}
	// TTL should be omitted (0) for alias records
	if strings.Contains(tmpl, `"ttl"`) {
		t.Error("alias record template should omit ttl field")
	}
}

func TestParseR53RecordTemplate_Valid(t *testing.T) {
	data := `{"name": "example.com.", "type": "A", "ttl": 300, "values": ["1.2.3.4"]}`
	rec, err := ParseR53RecordTemplate(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Name != "example.com." {
		t.Errorf("Name = %q, want %q", rec.Name, "example.com.")
	}
	if rec.Type != "A" {
		t.Errorf("Type = %q, want %q", rec.Type, "A")
	}
	if rec.TTL != 300 {
		t.Errorf("TTL = %d, want 300", rec.TTL)
	}
	if len(rec.Values) != 1 || rec.Values[0] != "1.2.3.4" {
		t.Errorf("Values = %v, want [1.2.3.4]", rec.Values)
	}
}

func TestParseR53RecordTemplate_EmptyName(t *testing.T) {
	data := `{"name": "", "type": "A"}`
	_, err := ParseR53RecordTemplate(data)
	if err == nil {
		t.Error("expected error for empty name")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should mention 'name', got: %v", err)
	}
}

func TestParseR53RecordTemplate_InvalidJSON(t *testing.T) {
	_, err := ParseR53RecordTemplate("not valid json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseR53RecordTemplate_AliasRecord(t *testing.T) {
	data := `{
		"name": "www.example.com.",
		"type": "A",
		"alias": {
			"dnsName": "my-lb.us-east-1.elb.amazonaws.com.",
			"hostedZoneId": "Z35SXDOTRQ7X7K"
		}
	}`
	rec, err := ParseR53RecordTemplate(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.AliasTarget != "my-lb.us-east-1.elb.amazonaws.com." {
		t.Errorf("AliasTarget = %q, want %q", rec.AliasTarget, "my-lb.us-east-1.elb.amazonaws.com.")
	}
	if rec.AliasZoneID != "Z35SXDOTRQ7X7K" {
		t.Errorf("AliasZoneID = %q, want %q", rec.AliasZoneID, "Z35SXDOTRQ7X7K")
	}
}

func TestParseR53RecordTemplate_MissingType(t *testing.T) {
	data := `{"name": "example.com.", "type": ""}`
	_, err := ParseR53RecordTemplate(data)
	if err == nil {
		t.Error("expected error for missing type")
	}
}

func TestBuildParseR53RecordTemplate_RoundTrip(t *testing.T) {
	original := &R53Record{
		Name:   "test.example.com.",
		Type:   "MX",
		TTL:    3600,
		Values: []string{"10 mail.example.com.", "20 mail2.example.com."},
	}
	tmpl := BuildR53RecordTemplate(original)
	parsed, err := ParseR53RecordTemplate(tmpl)
	if err != nil {
		t.Fatalf("round-trip parse failed: %v", err)
	}
	if parsed.Name != original.Name {
		t.Errorf("Name = %q, want %q", parsed.Name, original.Name)
	}
	if parsed.Type != original.Type {
		t.Errorf("Type = %q, want %q", parsed.Type, original.Type)
	}
	if parsed.TTL != original.TTL {
		t.Errorf("TTL = %d, want %d", parsed.TTL, original.TTL)
	}
	if len(parsed.Values) != len(original.Values) {
		t.Errorf("len(Values) = %d, want %d", len(parsed.Values), len(original.Values))
	}
}
