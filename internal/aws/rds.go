package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/dostrow/e9s/internal/model"
)

type RDSInstance = model.RDSInstance
type RDSInstanceDetail = model.RDSInstanceDetail

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
		detail.Metrics = metrics.Snapshot
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
	Snapshot       *model.MetricSnapshot
}

func (c *Client) fetchRDSMetrics(ctx context.Context, identifier string) (*rdsMetrics, error) {
	snapshot, err := c.GetRDSMetrics(ctx, identifier, 15*time.Minute)
	if err != nil {
		return nil, err
	}

	m := &rdsMetrics{Snapshot: snapshot}
	for _, series := range snapshot.Series {
		if len(series.Points) == 0 {
			continue
		}
		v := series.Points[len(series.Points)-1].Value
		switch series.ID {
		case "cpu":
			m.CPUPercent = v
		case "connections":
			m.DBConnections = v
		case "free_storage":
			m.FreeStorageGB = v
		case "read_iops":
			m.ReadIOPS = v
		case "write_iops":
			m.WriteIOPS = v
		case "read_latency":
			m.ReadLatencyMs = v
		case "write_latency":
			m.WriteLatencyMs = v
		}
	}
	return m, nil
}

// GetRDSMetrics returns standard AWS/RDS CloudWatch histories. Unsupported
// engine-specific metrics simply contain no points.
func (c *Client) GetRDSMetrics(ctx context.Context, identifier string, window time.Duration) (*model.MetricSnapshot, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, fmt.Errorf("DB instance identifier is required")
	}
	end := time.Now()
	dimensions := []model.MetricDimension{{Name: "DBInstanceIdentifier", Value: identifier}}
	metric := func(id, label, name, statistic, unit string, scale float64) model.MetricQuery {
		return model.MetricQuery{ID: id, Label: label, Namespace: "AWS/RDS", MetricName: name,
			Dimensions: dimensions, Statistic: statistic, Unit: unit, Scale: scale}
	}
	return c.GetMetricSeries(ctx, model.MetricRequest{
		StartTime: end.Add(-window), EndTime: end, MaxPoints: defaultMetricMaxPoints,
		Queries: []model.MetricQuery{
			metric("cpu", "Average", "CPUUtilization", "Average", "%", 1),
			metric("cpu_max", "Maximum", "CPUUtilization", "Maximum", "%", 1),
			metric("connections", "Connections", "DatabaseConnections", "Average", "count", 1),
			metric("free_memory", "Free memory", "FreeableMemory", "Average", "bytes", 1),
			metric("free_storage", "Free storage", "FreeStorageSpace", "Average", "GiB", 1/(1024*1024*1024.0)),
			metric("read_iops", "Read", "ReadIOPS", "Average", "iops", 1),
			metric("write_iops", "Write", "WriteIOPS", "Average", "iops", 1),
			metric("read_latency", "Read", "ReadLatency", "Average", "ms", 1000),
			metric("write_latency", "Write", "WriteLatency", "Average", "ms", 1000),
			metric("read_throughput", "Read", "ReadThroughput", "Average", "bytes/s", 1),
			metric("write_throughput", "Write", "WriteThroughput", "Average", "bytes/s", 1),
			metric("disk_queue", "Queue depth", "DiskQueueDepth", "Average", "count", 1),
			metric("network_receive", "Received", "NetworkReceiveThroughput", "Average", "bytes/s", 1),
			metric("network_transmit", "Transmitted", "NetworkTransmitThroughput", "Average", "bytes/s", 1),
			metric("burst_balance", "Burst balance", "BurstBalance", "Average", "%", 1),
			metric("replica_lag", "Replica lag", "ReplicaLag", "Average", "seconds", 1),
		},
	})
}
