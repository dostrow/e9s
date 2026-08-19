package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/dostrow/e9s/internal/model"
)

// ListECRRepos returns ECR repositories, optionally filtered by name substring.
func (c *Client) ListECRRepos(ctx context.Context, filter string) ([]model.ECRRepo, error) {
	var repos []model.ECRRepo
	lf := strings.ToLower(filter)

	paginator := ecr.NewDescribeRepositoriesPaginator(c.ECR, &ecr.DescribeRepositoriesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range page.Repositories {
			repo := repoFromSDK(r)
			if lf != "" && !strings.Contains(strings.ToLower(repo.Name), lf) {
				continue
			}
			repos = append(repos, repo)
		}
	}
	return repos, nil
}

// ListECRImages returns images in a repository, sorted by push date (newest first).
func (c *Client) ListECRImages(ctx context.Context, repoName string) ([]model.ECRImage, error) {
	var images []model.ECRImage

	paginator := ecr.NewDescribeImagesPaginator(c.ECR, &ecr.DescribeImagesInput{
		RepositoryName: &repoName,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, d := range page.ImageDetails {
			images = append(images, imageFromSDK(d))
		}
	}

	// Sort newest first
	for i, j := 0, len(images)-1; i < j; i, j = i+1, j-1 {
		// Check if already sorted
		if images[i].PushedAt.Before(images[j].PushedAt) {
			images[i], images[j] = images[j], images[i]
		}
	}
	// Proper sort
	sortImagesByPushDate(images)

	return images, nil
}

// GetECRScanFindings returns scan findings for an image.
func (c *Client) GetECRScanFindings(ctx context.Context, repoName, imageDigest string) ([]model.ECRFinding, error) {
	var findings []model.ECRFinding

	paginator := ecr.NewDescribeImageScanFindingsPaginator(c.ECR, &ecr.DescribeImageScanFindingsInput{
		RepositoryName: &repoName,
		ImageId:        &ecrtypes.ImageIdentifier{ImageDigest: &imageDigest},
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		if page.ImageScanFindings != nil {
			for _, f := range page.ImageScanFindings.Findings {
				finding := model.ECRFinding{
					Name:        derefStrAws(f.Name),
					Severity:    string(f.Severity),
					Description: derefStrAws(f.Description),
					URI:         derefStrAws(f.Uri),
				}
				for _, attr := range f.Attributes {
					if attr.Key != nil {
						switch *attr.Key {
						case "package_name":
							finding.Package = derefStrAws(attr.Value)
						case "package_version":
							finding.Version = derefStrAws(attr.Value)
						}
					}
				}
				findings = append(findings, finding)
			}
		}
	}

	// Sort by severity
	sortFindingsBySeverity(findings)
	return findings, nil
}

// StartECRScan initiates an on-demand scan for an image.
func (c *Client) StartECRScan(ctx context.Context, repoName, imageDigest string, tags []string) error {
	imageID := &ecrtypes.ImageIdentifier{ImageDigest: &imageDigest}
	if len(tags) > 0 {
		imageID.ImageTag = &tags[0]
	}
	_, err := c.ECR.StartImageScan(ctx, &ecr.StartImageScanInput{
		RepositoryName: &repoName,
		ImageId:        imageID,
	})
	return err
}

// DeleteECRImage deletes an image by digest.
func (c *Client) DeleteECRImage(ctx context.Context, repoName, imageDigest string) error {
	_, err := c.ECR.BatchDeleteImage(ctx, &ecr.BatchDeleteImageInput{
		RepositoryName: &repoName,
		ImageIds:       []ecrtypes.ImageIdentifier{{ImageDigest: &imageDigest}},
	})
	return err
}

func repoFromSDK(r ecrtypes.Repository) model.ECRRepo {
	repo := model.ECRRepo{
		Name: derefStrAws(r.RepositoryName),
		URI:  derefStrAws(r.RepositoryUri),
		ARN:  derefStrAws(r.RepositoryArn),
	}
	if r.ImageScanningConfiguration != nil {
		repo.ScanOnPush = r.ImageScanningConfiguration.ScanOnPush
	}
	repo.TagMutability = string(r.ImageTagMutability)
	if r.EncryptionConfiguration != nil {
		repo.EncryptionType = string(r.EncryptionConfiguration.EncryptionType)
	}
	if r.CreatedAt != nil {
		repo.CreatedAt = *r.CreatedAt
	}
	return repo
}

func imageFromSDK(d ecrtypes.ImageDetail) model.ECRImage {
	img := model.ECRImage{
		Digest: derefStrAws(d.ImageDigest),
		Tags:   d.ImageTags,
	}
	if d.ImagePushedAt != nil {
		img.PushedAt = *d.ImagePushedAt
	}
	if d.ImageSizeInBytes != nil {
		img.SizeBytes = *d.ImageSizeInBytes
	}
	if d.ArtifactMediaType != nil {
		img.MediaType = *d.ArtifactMediaType
	}
	if d.ImageScanStatus != nil {
		img.ScanStatus = string(d.ImageScanStatus.Status)
	}
	if d.ImageScanFindingsSummary != nil {
		img.ScanSeverity = d.ImageScanFindingsSummary.FindingSeverityCounts
	}
	return img
}

func sortImagesByPushDate(images []model.ECRImage) {
	for i := 1; i < len(images); i++ {
		for j := i; j > 0 && images[j].PushedAt.After(images[j-1].PushedAt); j-- {
			images[j], images[j-1] = images[j-1], images[j]
		}
	}
}

func severityOrder(s string) int {
	order := map[string]int{
		"CRITICAL":      0,
		"HIGH":          1,
		"MEDIUM":        2,
		"LOW":           3,
		"INFORMATIONAL": 4,
		"UNDEFINED":     5,
	}
	if o, ok := order[s]; ok {
		return o
	}
	return 9
}

func sortFindingsBySeverity(findings []model.ECRFinding) {
	for i := 1; i < len(findings); i++ {
		for j := i; j > 0 && severityOrder(findings[j].Severity) < severityOrder(findings[j-1].Severity); j-- {
			findings[j], findings[j-1] = findings[j-1], findings[j]
		}
	}
}
