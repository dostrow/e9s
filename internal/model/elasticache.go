package model

import "time"

type ElastiCacheKind string

const (
	ElastiCacheReplicationGroup ElastiCacheKind = "replication-group"
	ElastiCacheCluster          ElastiCacheKind = "cache-cluster"
	ElastiCacheServerless       ElastiCacheKind = "serverless-cache"
)

// ElastiCacheResource is the frontend-neutral representation shared by the
// node-based and serverless ElastiCache browsers. Fields that do not apply to
// a resource kind remain empty.
type ElastiCacheResource struct {
	Kind               ElastiCacheKind
	ID                 string
	ARN                string
	Description        string
	Engine             string
	Version            string
	Status             string
	NodeType           string
	NodeCount          int
	ShardCount         int
	Endpoint           string
	ReaderEndpoint     string
	Port               int32
	ReplicationGroupID string
	MemberClusterIDs   []string
	SubnetGroup        string
	SubnetIDs          []string
	SecurityGroupIDs   []string
	ParameterGroup     string
	AvailabilityZones  []string
	NetworkType        string
	ClusterMode        string
	AutomaticFailover  string
	MultiAZ            string
	AtRestEncrypted    bool
	TransitEncrypted   bool
	AuthTokenEnabled   bool
	KMSKeyID           string
	UserGroupIDs       []string
	SnapshotRetention  int32
	SnapshotWindow     string
	MaintenanceWindow  string
	LogGroups          []string
	DataStorageLimitGB float64
	ECPUPerSecondLimit int64
	Created            time.Time
	Tags               map[string]string
	Nodes              []ElastiCacheNode
}

type ElastiCacheNode struct {
	ClusterID        string
	NodeID           string
	ShardID          string
	Role             string
	Status           string
	Endpoint         string
	Port             int32
	AvailabilityZone string
	Created          time.Time
}
