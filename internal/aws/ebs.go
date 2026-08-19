package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/dostrow/e9s/internal/model"
)

// ListEBSVolumes returns every EBS volume in the active region.
func (c *Client) ListEBSVolumes(ctx context.Context) ([]model.EC2Volume, error) {
	var volumes []model.EC2Volume
	paginator := ec2.NewDescribeVolumesPaginator(c.EC2, &ec2.DescribeVolumesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, volume := range page.Volumes {
			volumes = append(volumes, volumeFromSDK(volume))
		}
	}
	return volumes, nil
}

// DescribeEBSVolume returns one volume by ID.
func (c *Client) DescribeEBSVolume(ctx context.Context, volumeID string) (*model.EC2Volume, error) {
	out, err := c.EC2.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{VolumeIds: []string{volumeID}})
	if err != nil {
		return nil, err
	}
	if len(out.Volumes) == 0 {
		return nil, fmt.Errorf("volume %s not found", volumeID)
	}
	volume := volumeFromSDK(out.Volumes[0])
	return &volume, nil
}

func volumeFromSDK(volume ec2types.Volume) model.EC2Volume {
	tags, name := ec2Tags(volume.Tags)
	result := model.EC2Volume{
		VolumeID:    derefStrAws(volume.VolumeId),
		Name:        name,
		VolumeType:  string(volume.VolumeType),
		State:       string(volume.State),
		AZ:          derefStrAws(volume.AvailabilityZone),
		Encrypted:   volume.Encrypted != nil && *volume.Encrypted,
		KMSKeyID:    derefStrAws(volume.KmsKeyId),
		SnapshotID:  derefStrAws(volume.SnapshotId),
		MultiAttach: volume.MultiAttachEnabled != nil && *volume.MultiAttachEnabled,
		Tags:        tags,
	}
	if volume.Size != nil {
		result.Size = *volume.Size
	}
	if volume.Iops != nil {
		result.IOPS = *volume.Iops
	}
	if volume.Throughput != nil {
		result.Throughput = *volume.Throughput
	}
	if volume.CreateTime != nil {
		result.CreatedAt = *volume.CreateTime
	}
	for _, attachment := range volume.Attachments {
		item := model.EC2VolumeAttachment{
			InstanceID:          derefStrAws(attachment.InstanceId),
			DeviceName:          derefStrAws(attachment.Device),
			State:               string(attachment.State),
			DeleteOnTermination: attachment.DeleteOnTermination != nil && *attachment.DeleteOnTermination,
		}
		if attachment.AttachTime != nil {
			item.AttachedAt = *attachment.AttachTime
		}
		result.Attachments = append(result.Attachments, item)
	}
	if len(result.Attachments) > 0 {
		result.DeviceName = result.Attachments[0].DeviceName
	}
	return result
}
