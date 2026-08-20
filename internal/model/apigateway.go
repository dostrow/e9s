package model

import "time"

type APIGatewayKind string

const (
	APIGatewayREST      APIGatewayKind = "rest-api"
	APIGatewayHTTP      APIGatewayKind = "http-api"
	APIGatewayWebSocket APIGatewayKind = "websocket-api"
	APIGatewayDomain    APIGatewayKind = "custom-domain"
)

type APIGatewayAPI struct {
	Kind                   APIGatewayKind
	ID                     string
	Name                   string
	ARN                    string
	Description            string
	Protocol               string
	Version                string
	Endpoint               string
	EndpointTypes          []string
	DisableExecuteEndpoint bool
	Created                time.Time
	Stages                 []APIGatewayStage
	Routes                 []APIGatewayRoute
	Integrations           []APIGatewayIntegration
	Mappings               []APIGatewayMapping
	Tags                   map[string]string
	SecurityPolicy         string
	CertificateARN         string
	HostedZoneID           string
	Status                 string
}

type APIGatewayStage struct {
	Name          string
	DeploymentID  string
	AutoDeploy    bool
	Description   string
	InvokeURL     string
	LogGroupARN   string
	Throttling    string
	LastUpdatedAt time.Time
}

type APIGatewayRoute struct {
	ID            string
	Key           string
	Path          string
	Methods       []string
	Authorization string
	Target        string
}

type APIGatewayIntegration struct {
	ID                string
	Type              string
	Method            string
	URI               string
	ConnectionType    string
	PayloadVersion    string
	TimeoutMS         int32
	CredentialsARN    string
	LambdaFunctionARN string
}

type APIGatewayMapping struct {
	APIID      string
	Stage      string
	MappingKey string
}
