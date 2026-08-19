package model

import "time"

// EC2Instance is the UI-neutral summary used by both frontends.
type EC2Instance struct {
	InstanceID     string
	Name           string
	State          string
	Type           string
	AZ             string
	PrivateIP      string
	PublicIP       string
	VpcID          string
	SubnetID       string
	KeyName        string
	LaunchTime     time.Time
	Platform       string
	AMI            string
	IAMRole        string
	Tags           map[string]string
	SecurityGroups []EC2SecurityGroupRef
}

// EC2SecurityGroupRef is a minimal security-group reference on an instance.
type EC2SecurityGroupRef struct {
	ID   string
	Name string
}

// EC2InstanceDetail contains the extended state for one instance.
type EC2InstanceDetail struct {
	EC2Instance
	Architecture       string
	RootDeviceType     string
	RootDeviceName     string
	EBSOptimized       bool
	Monitoring         string
	Volumes            []EC2Volume
	SecurityGroupRules []EC2SGRule
}

// EC2Volume describes an attached EBS volume.
type EC2Volume struct {
	VolumeID    string
	Name        string
	DeviceName  string
	Size        int32
	VolumeType  string
	State       string
	AZ          string
	IOPS        int32
	Throughput  int32
	Encrypted   bool
	KMSKeyID    string
	SnapshotID  string
	MultiAttach bool
	CreatedAt   time.Time
	Tags        map[string]string
	Attachments []EC2VolumeAttachment
}

// EC2VolumeAttachment describes one instance attachment for an EBS volume.
type EC2VolumeAttachment struct {
	InstanceID          string
	DeviceName          string
	State               string
	AttachedAt          time.Time
	DeleteOnTermination bool
}

// EC2SGRule is a presentation-neutral security-group rule.
type EC2SGRule struct {
	RuleID      string
	Direction   string
	Protocol    string
	PortRange   string
	Source      string
	Description string
}

// EC2SecurityGroup is the UI-neutral representation of a security group.
type EC2SecurityGroup struct {
	GroupID     string
	Name        string
	Description string
	VpcID       string
	OwnerID     string
	Tags        map[string]string
	Rules       []EC2SGRule
}

// EC2VPC is the UI-neutral representation of a VPC.
type EC2VPC struct {
	VpcID       string
	Name        string
	State       string
	CIDRs       []string
	IPv6CIDRs   []string
	IsDefault   bool
	Tenancy     string
	OwnerID     string
	DHCPOptions string
	Tags        map[string]string
}

// EC2Subnet is the UI-neutral representation of a subnet.
type EC2Subnet struct {
	SubnetID           string
	Name               string
	VpcID              string
	State              string
	AZ                 string
	AZID               string
	CIDR               string
	IPv6CIDRs          []string
	AvailableIPs       int32
	MapPublicIP        bool
	DefaultForAZ       bool
	AssignIPv6OnCreate bool
	OwnerID            string
	Tags               map[string]string
}

// EC2LoadBalancer is the shared ALB/NLB summary and detail value.
type EC2LoadBalancer struct {
	ARN               string
	Name              string
	Type              string
	Scheme            string
	State             string
	StateReason       string
	DNSName           string
	HostedZoneID      string
	VpcID             string
	IPAddressType     string
	CreatedAt         time.Time
	SecurityGroups    []string
	AvailabilityZones []EC2LoadBalancerZone
	Listeners         []EC2Listener
	TargetGroups      []EC2TargetGroup
}

// EC2LoadBalancerZone describes one enabled load-balancer zone and subnet.
type EC2LoadBalancerZone struct {
	AZ       string
	SubnetID string
	IPv4     string
	IPv6     string
}

// EC2Listener describes an ELBv2 listener and its default actions.
type EC2Listener struct {
	ARN          string
	Protocol     string
	Port         int32
	SSLPolicy    string
	Certificates []string
	Actions      []string
}

// EC2TargetGroup describes a target group and its current registered targets.
type EC2TargetGroup struct {
	ARN                 string
	Name                string
	Protocol            string
	ProtocolVersion     string
	Port                int32
	TargetType          string
	VpcID               string
	HealthCheckProtocol string
	HealthCheckPort     string
	HealthCheckPath     string
	Targets             []EC2TargetHealth
}

// EC2TargetHealth describes one target registered with a target group.
type EC2TargetHealth struct {
	ID          string
	Port        int32
	AZ          string
	State       string
	Reason      string
	Description string
}
