package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dostrow/e9s/internal/model"
)

// ECRAPI is the low-level registry behavior used by shared ECR workflows.
type ECRAPI interface {
	ListECRRepos(context.Context, string) ([]model.ECRRepo, error)
	ListECRImages(context.Context, string) ([]model.ECRImage, error)
	GetECRScanFindings(context.Context, string, string) (model.ECRScan, error)
	StartECRScan(context.Context, string, string, []string) error
	DeleteECRImage(context.Context, string, string) error
}

// ECR centralizes registry discovery, ordering, validation, and mutations.
type ECR struct{ api ECRAPI }

func NewECR(api ECRAPI) *ECR { return &ECR{api: api} }

func (s *ECR) ListRepositories(ctx context.Context, filter string) ([]model.ECRRepo, error) {
	repositories, err := s.api.ListECRRepos(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("list ECR repositories: %w", err)
	}
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter != "" {
		filtered := make([]model.ECRRepo, 0, len(repositories))
		for _, repository := range repositories {
			if strings.Contains(strings.ToLower(repository.Name), filter) || strings.Contains(strings.ToLower(repository.URI), filter) {
				filtered = append(filtered, repository)
			}
		}
		repositories = filtered
	}
	sort.SliceStable(repositories, func(i, j int) bool {
		return strings.ToLower(repositories[i].Name) < strings.ToLower(repositories[j].Name)
	})
	return repositories, nil
}

func (s *ECR) ListImages(ctx context.Context, repository string) ([]model.ECRImage, error) {
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return nil, fmt.Errorf("list ECR images: repository name is required")
	}
	images, err := s.api.ListECRImages(ctx, repository)
	if err != nil {
		return nil, fmt.Errorf("list ECR images in %q: %w", repository, err)
	}
	sort.SliceStable(images, func(i, j int) bool {
		if images[i].PushedAt.Equal(images[j].PushedAt) {
			return images[i].Digest < images[j].Digest
		}
		return images[i].PushedAt.After(images[j].PushedAt)
	})
	return images, nil
}

func (s *ECR) Findings(ctx context.Context, repository, digest string) ([]model.ECRFinding, error) {
	scan, err := s.ScanFindings(ctx, repository, digest)
	return scan.Findings, err
}

func (s *ECR) ScanFindings(ctx context.Context, repository, digest string) (model.ECRScan, error) {
	repository, digest = strings.TrimSpace(repository), strings.TrimSpace(digest)
	if repository == "" || digest == "" {
		return model.ECRScan{}, fmt.Errorf("read ECR scan findings: repository and image digest are required")
	}
	scan, err := s.api.GetECRScanFindings(ctx, repository, digest)
	if err != nil {
		return model.ECRScan{}, fmt.Errorf("read ECR scan findings for %q: %w", repository, err)
	}
	sort.SliceStable(scan.Findings, func(i, j int) bool {
		left, right := ecrSeverityOrder(scan.Findings[i].Severity), ecrSeverityOrder(scan.Findings[j].Severity)
		if left == right {
			return strings.ToLower(scan.Findings[i].Name) < strings.ToLower(scan.Findings[j].Name)
		}
		return left < right
	})
	if len(scan.Severity) == 0 && len(scan.Findings) > 0 {
		scan.Severity = make(map[string]int32)
		for _, finding := range scan.Findings {
			scan.Severity[strings.ToUpper(finding.Severity)]++
		}
	}
	return scan, nil
}

func (s *ECR) StartScan(ctx context.Context, repository string, image model.ECRImage) error {
	repository = strings.TrimSpace(repository)
	if repository == "" || strings.TrimSpace(image.Digest) == "" {
		return fmt.Errorf("start ECR image scan: repository and image digest are required")
	}
	status := strings.ToUpper(strings.TrimSpace(image.ScanStatus))
	if status == "ACTIVE" {
		return fmt.Errorf("start ECR image scan for %q: enhanced continuous scanning is already active", repository)
	}
	if status == "IN_PROGRESS" || status == "PENDING" {
		return fmt.Errorf("start ECR image scan for %q: scan is already %s", repository, strings.ToLower(status))
	}
	if err := s.api.StartECRScan(ctx, repository, image.Digest, image.Tags); err != nil {
		return fmt.Errorf("start ECR image scan for %q: %w", repository, err)
	}
	return nil
}

func (s *ECR) DeleteImage(ctx context.Context, repository, digest string) error {
	repository, digest = strings.TrimSpace(repository), strings.TrimSpace(digest)
	if repository == "" || digest == "" {
		return fmt.Errorf("delete ECR image: repository and image digest are required")
	}
	if err := s.api.DeleteECRImage(ctx, repository, digest); err != nil {
		return fmt.Errorf("delete ECR image from %q: %w", repository, err)
	}
	return nil
}

func (s *ECR) ImageURI(repositoryURI string, image model.ECRImage) (string, error) {
	repositoryURI = strings.TrimSpace(repositoryURI)
	if repositoryURI == "" {
		return "", fmt.Errorf("build ECR image URI: repository URI is required")
	}
	if len(image.Tags) > 0 && strings.TrimSpace(image.Tags[0]) != "" {
		return repositoryURI + ":" + image.Tags[0], nil
	}
	if image.Digest == "" {
		return "", fmt.Errorf("build ECR image URI: image tag or digest is required")
	}
	return repositoryURI + "@" + image.Digest, nil
}

func ecrSeverityOrder(severity string) int {
	switch severity {
	case "CRITICAL":
		return 0
	case "HIGH":
		return 1
	case "MEDIUM":
		return 2
	case "LOW":
		return 3
	case "INFORMATIONAL":
		return 4
	case "UNDEFINED":
		return 5
	default:
		return 9
	}
}
