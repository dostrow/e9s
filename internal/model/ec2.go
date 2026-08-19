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
	VolumeID   string
	DeviceName string
	Size       int32
	VolumeType string
	State      string
}

// EC2SGRule is a presentation-neutral security-group rule.
type EC2SGRule struct {
	Direction string
	Protocol  string
	PortRange string
	Source    string
}
