package model

// Route53Zone is one public or private hosted zone.
type Route53Zone struct {
	ID          string
	Name        string
	Private     bool
	RecordCount int64
	Comment     string
}

// Route53Record is a complete Route53 resource-record set. Routing fields are
// retained so an edit can round-trip an existing non-simple record safely.
type Route53Record struct {
	Name                 string
	Type                 string
	TTL                  int64
	Values               []string
	AliasTarget          string
	AliasZoneID          string
	EvaluateTargetHealth bool
	RoutingPolicy        string
	SetIdentifier        string
	Weight               int64
	Region               string
	Failover             string
	GeoContinentCode     string
	GeoCountryCode       string
	GeoSubdivisionCode   string
	MultiValue           bool
	HealthCheckID        string
}

// Route53DNSAnswer contains the result returned by Route53 TestDNSAnswer.
type Route53DNSAnswer struct {
	RecordName   string
	RecordType   string
	ResponseCode string
	Nameserver   string
	Protocol     string
	RecordData   []string
}

// Route53RecordTemplate is the portable JSON document edited by either
// frontend. Alias records use Alias instead of TTL and Values.
type Route53RecordTemplate struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	TTL    int64    `json:"ttl,omitempty"`
	Values []string `json:"values,omitempty"`
	Alias  *struct {
		DNSName              string `json:"dnsName"`
		HostedZoneID         string `json:"hostedZoneId"`
		EvaluateTargetHealth bool   `json:"evaluateTargetHealth,omitempty"`
	} `json:"alias,omitempty"`
	SetIdentifier      string `json:"setIdentifier,omitempty"`
	Weight             int64  `json:"weight,omitempty"`
	Region             string `json:"region,omitempty"`
	Failover           string `json:"failover,omitempty"`
	GeoContinentCode   string `json:"geoContinentCode,omitempty"`
	GeoCountryCode     string `json:"geoCountryCode,omitempty"`
	GeoSubdivisionCode string `json:"geoSubdivisionCode,omitempty"`
	MultiValue         bool   `json:"multiValue,omitempty"`
	HealthCheckID      string `json:"healthCheckId,omitempty"`
}
