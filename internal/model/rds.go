package model

import "time"

// RDSInstance is the browser summary for an RDS DB instance.
type RDSInstance struct {
	Identifier         string
	Engine             string
	Version            string
	Class              string
	Status             string
	Role               string
	ClusterID          string
	AZ                 string
	MultiAZ            bool
	Endpoint           string
	Port               int32
	StorageGB          int32
	StorageType        string
	Encrypted          bool
	DeletionProtection bool
	ReadReplicaSource  string
	Created            time.Time
}

// RDSInstanceDetail contains configuration and a compact current metric
// summary. Metrics retains the complete history for TUI and GUI renderers.
type RDSInstanceDetail struct {
	RDSInstance
	SubnetGroup          string
	VPCID                string
	SecurityGroups       []string
	ParameterGroups      []string
	BackupRetentionDays  int32
	BackupWindow         string
	MaintenanceWindow    string
	LatestRestorableTime time.Time
	CACertificate        string
	PromotionTier        int32
	Tags                 map[string]string
	CPUPercent           float64
	DBConnections        float64
	FreeStorageGB        float64
	ReadIOPS             float64
	WriteIOPS            float64
	ReadLatencyMs        float64
	WriteLatencyMs       float64
	MetricsLoaded        bool
	Metrics              *MetricSnapshot
}

// RDSCluster is the shared browser and detail representation for an RDS or
// Aurora DB cluster. Members retain enough information for both frontends to
// drill into the cluster's DB instances without inventing frontend state.
type RDSCluster struct {
	Identifier           string
	Engine               string
	Version              string
	EngineMode           string
	Status               string
	DatabaseName         string
	Endpoint             string
	ReaderEndpoint       string
	Port                 int32
	MultiAZ              bool
	Encrypted            bool
	DeletionProtection   bool
	SubnetGroup          string
	SecurityGroups       []string
	ParameterGroup       string
	AvailabilityZones    []string
	BackupRetentionDays  int32
	BackupWindow         string
	MaintenanceWindow    string
	LatestRestorableTime time.Time
	Created              time.Time
	Members              []RDSClusterMember
	Tags                 map[string]string
}

type RDSClusterMember struct {
	Identifier    string
	Writer        bool
	PromotionTier int32
}
