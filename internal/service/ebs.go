package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dostrow/e9s/internal/model"
)

// EBSAPI is the low-level EBS behavior used by shared frontend workflows.
type EBSAPI interface {
	ListEBSVolumes(context.Context) ([]model.EC2Volume, error)
	DescribeEBSVolume(context.Context, string) (*model.EC2Volume, error)
}

// EBS exposes UI-neutral volume discovery and detail workflows.
type EBS struct {
	api EBSAPI
}

func NewEBS(api EBSAPI) *EBS {
	return &EBS{api: api}
}

func (s *EBS) List(ctx context.Context, filter string) ([]model.EC2Volume, error) {
	volumes, err := s.api.ListEBSVolumes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list EBS volumes: %w", err)
	}
	return FilterEBSVolumes(volumes, filter), nil
}

func FilterEBSVolumes(volumes []model.EC2Volume, filter string) []model.EC2Volume {
	filter = normalizedFilter(filter)
	filtered := make([]model.EC2Volume, 0, len(volumes))
	for _, volume := range volumes {
		values := []string{volume.VolumeID, volume.Name, volume.State, volume.VolumeType,
			volume.AZ, volume.KMSKeyID, volume.SnapshotID}
		for _, attachment := range volume.Attachments {
			values = append(values, attachment.InstanceID, attachment.DeviceName, attachment.State)
		}
		if filter == "" || containsAny(filter, values...) || tagsMatch(volume.Tags, filter) {
			filtered = append(filtered, volume)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		leftAttached, rightAttached := len(filtered[i].Attachments) > 0, len(filtered[j].Attachments) > 0
		if leftAttached != rightAttached {
			return !leftAttached
		}
		return compareNameID(filtered[i].Name, filtered[i].VolumeID, filtered[j].Name, filtered[j].VolumeID)
	})
	return filtered
}

func (s *EBS) Detail(ctx context.Context, volumeID string) (*model.EC2Volume, error) {
	volumeID, err := requireResourceID("read EBS volume", "volume ID", volumeID)
	if err != nil {
		return nil, err
	}
	volume, err := s.api.DescribeEBSVolume(ctx, volumeID)
	if err != nil {
		return nil, fmt.Errorf("read EBS volume %q: %w", volumeID, err)
	}
	if volume == nil {
		return nil, fmt.Errorf("read EBS volume %q: volume was not found", volumeID)
	}
	sort.SliceStable(volume.Attachments, func(i, j int) bool {
		left := strings.Join([]string{volume.Attachments[i].InstanceID, volume.Attachments[i].DeviceName}, "\x00")
		right := strings.Join([]string{volume.Attachments[j].InstanceID, volume.Attachments[j].DeviceName}, "\x00")
		return left < right
	})
	return volume, nil
}
