package service

import "github.com/dostrow/e9s/internal/model"

// ECSServiceResourceRefs returns implemented infrastructure resources from an
// ECS service configuration. Target groups are configuration-level links; they
// do not imply that every individual task is currently registered and healthy.
func ECSServiceResourceRefs(parent model.Service) []model.ResourceRef {
	refs := make([]model.ResourceRef, 0, len(parent.SecurityGroups)+len(parent.TargetGroups))
	for _, group := range parent.SecurityGroups {
		refs = append(refs, model.ResourceRef{Kind: "ec2-security-group", ID: group.ID, Name: group.Name})
	}
	refs = append(refs, parent.TargetGroups...)
	return UniqueResourceRefs(refs)
}

// ECSTaskResourceRefs returns implemented infrastructure resources associated
// with a task and, when supplied, its parent service.
func ECSTaskResourceRefs(task model.Task, parent *model.Service) []model.ResourceRef {
	refs := make([]model.ResourceRef, 0, 5+len(task.SecurityGroups)+len(task.VolumeIDs))
	if task.EC2InstanceID != "" {
		refs = append(refs, model.ResourceRef{Kind: "ec2-instance", ID: task.EC2InstanceID})
	}
	if task.VpcID != "" {
		refs = append(refs, model.ResourceRef{Kind: "ec2-vpc", ID: task.VpcID})
	}
	if task.SubnetID != "" {
		refs = append(refs, model.ResourceRef{Kind: "ec2-subnet", ID: task.SubnetID})
	}
	for _, group := range task.SecurityGroups {
		refs = append(refs, model.ResourceRef{Kind: "ec2-security-group", ID: group.ID, Name: group.Name})
	}
	for _, volumeID := range task.VolumeIDs {
		refs = append(refs, model.ResourceRef{Kind: "ec2-volume", ID: volumeID})
	}
	if parent != nil {
		refs = append(refs, ECSServiceResourceRefs(*parent)...)
	}
	return UniqueResourceRefs(refs)
}

// UniqueResourceRefs preserves order while removing empty and duplicate refs.
func UniqueResourceRefs(refs []model.ResourceRef) []model.ResourceRef {
	seen := make(map[string]bool, len(refs))
	unique := make([]model.ResourceRef, 0, len(refs))
	for _, ref := range refs {
		key := ref.Kind + "\x00" + ref.ID
		if ref.ID == "" || seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, ref)
	}
	return unique
}
