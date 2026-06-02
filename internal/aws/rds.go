package aws

import (
	"context"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

// RDSInstance represents an RDS DB instance summary.
type RDSInstance struct {
	Identifier         string
	Engine             string
	Version            string
	Class              string
	Status             string
	// Role is "writer", "reader", "primary", "replica", or "" for standalone.
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
	// ReadReplicaSource is set when this instance is a read replica.
	ReadReplicaSource  string
	Created            time.Time
}

// RDSInstanceDetail holds extended information for a single DB instance.
type RDSInstanceDetail struct {
	RDSInstance
	SubnetGroup             string
	VPCID                   string
	SecurityGroups          []string
	ParameterGroups         []string
	BackupRetentionDays     int32
	BackupWindow            string
	MaintenanceWindow       string
	LatestRestorableTime    time.Time
	CACertificate           string
	PromotionTier           int32 // Aurora reader failover priority
	Tags                    map[string]string
	// CloudWatch metrics (last 5 minutes, average)
	CPUPercent     float64
	DBConnections  float64
	FreeStorageGB  float64
	ReadIOPS       float64
	WriteIOPS      float64
	ReadLatencyMs  float64
	WriteLatencyMs float64
	MetricsLoaded  bool
}

// ListRDSInstances returns all RDS DB instances annotated with their cluster role.
// It fetches Aurora cluster data to determine writer vs reader status.
func (c *Client) ListRDSInstances(ctx context.Context, filter string) ([]RDSInstance, error) {
	// Build writer map: instanceID → isWriter from Aurora clusters
	writerMap := map[string]bool{}
	clusterPaginator := rds.NewDescribeDBClustersPaginator(c.RDS, &rds.DescribeDBClustersInput{})
	for clusterPaginator.HasMorePages() {
		page, err := clusterPaginator.NextPage(ctx)
		if err == nil {
			for _, cl := range page.DBClusters {
				for _, m := range cl.DBClusterMembers {
					if m.DBInstanceIdentifier != nil {
						writerMap[*m.DBInstanceIdentifier] = awssdk.ToBool(m.IsClusterWriter)
					}
				}
			}
		}
	}

	var instances []RDSInstance
	lf := strings.ToLower(filter)

	paginator := rds.NewDescribeDBInstancesPaginator(c.RDS, &rds.DescribeDBInstancesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, db := range page.DBInstances {
			inst := RDSInstance{}
			if db.DBInstanceIdentifier != nil {
				inst.Identifier = *db.DBInstanceIdentifier
			}
			if db.Engine != nil {
				inst.Engine = *db.Engine
			}
			if db.EngineVersion != nil {
				inst.Version = *db.EngineVersion
			}
			if db.DBInstanceClass != nil {
				inst.Class = *db.DBInstanceClass
			}
			if db.DBInstanceStatus != nil {
				inst.Status = *db.DBInstanceStatus
			}
			if db.AvailabilityZone != nil {
				inst.AZ = *db.AvailabilityZone
			}
			if db.MultiAZ != nil {
				inst.MultiAZ = *db.MultiAZ
			}
			if db.DBClusterIdentifier != nil {
				inst.ClusterID = *db.DBClusterIdentifier
			}
			if db.Endpoint != nil {
				if db.Endpoint.Address != nil {
					inst.Endpoint = *db.Endpoint.Address
				}
				if db.Endpoint.Port != nil {
					inst.Port = *db.Endpoint.Port
				}
			}
			if db.AllocatedStorage != nil {
				inst.StorageGB = *db.AllocatedStorage
			}
			if db.StorageType != nil {
				inst.StorageType = *db.StorageType
			}
			if db.StorageEncrypted != nil {
				inst.Encrypted = *db.StorageEncrypted
			}
			if db.DeletionProtection != nil {
				inst.DeletionProtection = *db.DeletionProtection
			}
			if db.ReadReplicaSourceDBInstanceIdentifier != nil {
				inst.ReadReplicaSource = *db.ReadReplicaSourceDBInstanceIdentifier
			}
			if db.InstanceCreateTime != nil {
				inst.Created = *db.InstanceCreateTime
			}

			inst.Role = rdsRole(inst.Identifier, inst.ClusterID, inst.ReadReplicaSource,
				len(db.ReadReplicaDBInstanceIdentifiers) > 0, writerMap)

			if lf != "" && !strings.Contains(strings.ToLower(inst.Identifier), lf) &&
				!strings.Contains(strings.ToLower(inst.Engine), lf) {
				continue
			}
			instances = append(instances, inst)
		}
	}
	return instances, nil
}

// DescribeRDSInstance returns full detail for a single DB instance, including CloudWatch metrics.
func (c *Client) DescribeRDSInstance(ctx context.Context, identifier string) (*RDSInstanceDetail, error) {
	out, err := c.RDS.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: awssdk.String(identifier),
	})
	if err != nil {
		return nil, err
	}
	if len(out.DBInstances) == 0 {
		return nil, nil
	}
	db := out.DBInstances[0]

	detail := &RDSInstanceDetail{}
	detail.Identifier = identifier
	if db.Engine != nil {
		detail.Engine = *db.Engine
	}
	if db.EngineVersion != nil {
		detail.Version = *db.EngineVersion
	}
	if db.DBInstanceClass != nil {
		detail.Class = *db.DBInstanceClass
	}
	if db.DBInstanceStatus != nil {
		detail.Status = *db.DBInstanceStatus
	}
	if db.AvailabilityZone != nil {
		detail.AZ = *db.AvailabilityZone
	}
	if db.MultiAZ != nil {
		detail.MultiAZ = *db.MultiAZ
	}
	if db.DBClusterIdentifier != nil {
		detail.ClusterID = *db.DBClusterIdentifier
	}
	if db.Endpoint != nil {
		if db.Endpoint.Address != nil {
			detail.Endpoint = *db.Endpoint.Address
		}
		if db.Endpoint.Port != nil {
			detail.Port = *db.Endpoint.Port
		}
	}
	if db.AllocatedStorage != nil {
		detail.StorageGB = *db.AllocatedStorage
	}
	if db.StorageType != nil {
		detail.StorageType = *db.StorageType
	}
	if db.StorageEncrypted != nil {
		detail.Encrypted = *db.StorageEncrypted
	}
	if db.DeletionProtection != nil {
		detail.DeletionProtection = *db.DeletionProtection
	}
	if db.ReadReplicaSourceDBInstanceIdentifier != nil {
		detail.ReadReplicaSource = *db.ReadReplicaSourceDBInstanceIdentifier
	}
	if db.InstanceCreateTime != nil {
		detail.Created = *db.InstanceCreateTime
	}
	if db.DBSubnetGroup != nil && db.DBSubnetGroup.DBSubnetGroupName != nil {
		detail.SubnetGroup = *db.DBSubnetGroup.DBSubnetGroupName
		if db.DBSubnetGroup.VpcId != nil {
			detail.VPCID = *db.DBSubnetGroup.VpcId
		}
	}
	for _, sg := range db.VpcSecurityGroups {
		if sg.VpcSecurityGroupId != nil {
			s := *sg.VpcSecurityGroupId
			if sg.Status != nil {
				s += " (" + *sg.Status + ")"
			}
			detail.SecurityGroups = append(detail.SecurityGroups, s)
		}
	}
	for _, pg := range db.DBParameterGroups {
		if pg.DBParameterGroupName != nil {
			detail.ParameterGroups = append(detail.ParameterGroups, *pg.DBParameterGroupName)
		}
	}
	if db.BackupRetentionPeriod != nil {
		detail.BackupRetentionDays = *db.BackupRetentionPeriod
	}
	if db.PreferredBackupWindow != nil {
		detail.BackupWindow = *db.PreferredBackupWindow
	}
	if db.PreferredMaintenanceWindow != nil {
		detail.MaintenanceWindow = *db.PreferredMaintenanceWindow
	}
	if db.LatestRestorableTime != nil {
		detail.LatestRestorableTime = *db.LatestRestorableTime
	}
	if db.CACertificateIdentifier != nil {
		detail.CACertificate = *db.CACertificateIdentifier
	}
	if db.PromotionTier != nil {
		detail.PromotionTier = *db.PromotionTier
	}
	detail.Tags = map[string]string{}
	for _, tag := range db.TagList {
		if tag.Key != nil && tag.Value != nil {
			detail.Tags[*tag.Key] = *tag.Value
		}
	}

	// Determine role from cluster data
	if detail.ClusterID != "" {
		clOut, err := c.RDS.DescribeDBClusters(ctx, &rds.DescribeDBClustersInput{
			DBClusterIdentifier: awssdk.String(detail.ClusterID),
		})
		if err == nil && len(clOut.DBClusters) > 0 {
			for _, m := range clOut.DBClusters[0].DBClusterMembers {
				if m.DBInstanceIdentifier != nil && *m.DBInstanceIdentifier == identifier {
					if awssdk.ToBool(m.IsClusterWriter) {
						detail.Role = "writer"
					} else {
						detail.Role = "reader"
						if m.PromotionTier != nil {
							detail.PromotionTier = *m.PromotionTier
						}
					}
					break
				}
			}
		}
	} else {
		detail.Role = rdsRole(identifier, detail.ClusterID, detail.ReadReplicaSource,
			len(db.ReadReplicaDBInstanceIdentifiers) > 0, nil)
	}

	// Fetch CloudWatch metrics (last 5 minutes)
	metrics, err := c.fetchRDSMetrics(ctx, identifier)
	if err == nil {
		detail.CPUPercent = metrics.CPUPercent
		detail.DBConnections = metrics.DBConnections
		detail.FreeStorageGB = metrics.FreeStorageGB
		detail.ReadIOPS = metrics.ReadIOPS
		detail.WriteIOPS = metrics.WriteIOPS
		detail.ReadLatencyMs = metrics.ReadLatencyMs
		detail.WriteLatencyMs = metrics.WriteLatencyMs
		detail.MetricsLoaded = true
	}

	return detail, nil
}

// rdsRole returns the role string for an instance given its cluster membership,
// replica relationships, and the writer map built from DescribeDBClusters.
// writerMap maps instanceID → isWriter; nil is safe (treated as empty).
func rdsRole(identifier, clusterID, readReplicaSource string, hasReplicas bool, writerMap map[string]bool) string {
	if clusterID != "" {
		if isWriter, ok := writerMap[identifier]; ok {
			if isWriter {
				return "writer"
			}
			return "reader"
		}
		return ""
	}
	if readReplicaSource != "" {
		return "replica"
	}
	if hasReplicas {
		return "primary"
	}
	return ""
}

type rdsMetrics struct {
	CPUPercent     float64
	DBConnections  float64
	FreeStorageGB  float64
	ReadIOPS       float64
	WriteIOPS      float64
	ReadLatencyMs  float64
	WriteLatencyMs float64
}

func (c *Client) fetchRDSMetrics(ctx context.Context, identifier string) (*rdsMetrics, error) {
	now := time.Now()
	start := now.Add(-10 * time.Minute)
	period := int32(300) // 5-minute period

	dims := []cwtypes.Dimension{
		{Name: awssdk.String("DBInstanceIdentifier"), Value: awssdk.String(identifier)},
	}

	makeQ := func(id, metric string) cwtypes.MetricDataQuery {
		return cwtypes.MetricDataQuery{
			Id: awssdk.String(id),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  awssdk.String("AWS/RDS"),
					MetricName: awssdk.String(metric),
					Dimensions: dims,
				},
				Period: &period,
				Stat:   awssdk.String("Average"),
			},
		}
	}

	out, err := c.CW.GetMetricData(ctx, &cloudwatch.GetMetricDataInput{
		StartTime: &start,
		EndTime:   &now,
		MetricDataQueries: []cwtypes.MetricDataQuery{
			makeQ("cpu", "CPUUtilization"),
			makeQ("conns", "DatabaseConnections"),
			makeQ("free_storage", "FreeStorageSpace"),
			makeQ("read_iops", "ReadIOPS"),
			makeQ("write_iops", "WriteIOPS"),
			makeQ("read_lat", "ReadLatency"),
			makeQ("write_lat", "WriteLatency"),
		},
	})
	if err != nil {
		return nil, err
	}

	m := &rdsMetrics{}
	for _, r := range out.MetricDataResults {
		if r.Id == nil || len(r.Values) == 0 {
			continue
		}
		v := r.Values[0]
		switch *r.Id {
		case "cpu":
			m.CPUPercent = v
		case "conns":
			m.DBConnections = v
		case "free_storage":
			m.FreeStorageGB = v / (1024 * 1024 * 1024)
		case "read_iops":
			m.ReadIOPS = v
		case "write_iops":
			m.WriteIOPS = v
		case "read_lat":
			m.ReadLatencyMs = v * 1000
		case "write_lat":
			m.WriteLatencyMs = v * 1000
		}
	}
	return m, nil
}
