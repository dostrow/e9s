package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

// ECRRepo represents an ECR repository.
type ECRRepo struct {
	Name           string
	URI            string
	ARN            string
	ScanOnPush     bool
	TagMutability  string // MUTABLE or IMMUTABLE
	EncryptionType string
	CreatedAt      time.Time
}

// ECRImage represents an image in an ECR repository.
type ECRImage struct {
	Digest       string
	Tags         []string
	PushedAt     time.Time
	SizeBytes    int64
	MediaType    string
	ScanStatus   string // COMPLETE, IN_PROGRESS, FAILED, etc.
	ScanSeverity map[string]int32 // severity → count
}

// ECRFinding represents a vulnerability finding from an image scan.
type ECRFinding struct {
	Name        string
	Severity    string // CRITICAL, HIGH, MEDIUM, LOW, INFORMATIONAL, UNDEFINED
	Description string
	URI         string
	Package     string
	Version     string
}

// ListECRRepos returns ECR repositories, optionally filtered by name substring.
func (c *Client) ListECRRepos(ctx context.Context, filter string) ([]ECRRepo, error) {
	var repos []ECRRepo
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
func (c *Client) ListECRImages(ctx context.Context, repoName string) ([]ECRImage, error) {
	var images []ECRImage

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
func (c *Client) GetECRScanFindings(ctx context.Context, repoName, imageDigest string) ([]ECRFinding, error) {
	var findings []ECRFinding

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
				finding := ECRFinding{
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

// ECRImageURI returns the full image URI for an image.
func ECRImageURI(repoURI, tag string) string {
	if tag != "" {
		return fmt.Sprintf("%s:%s", repoURI, tag)
	}
	return repoURI
}

func repoFromSDK(r ecrtypes.Repository) ECRRepo {
	repo := ECRRepo{
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

func imageFromSDK(d ecrtypes.ImageDetail) ECRImage {
	img := ECRImage{
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

func sortImagesByPushDate(images []ECRImage) {
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

func sortFindingsBySeverity(findings []ECRFinding) {
	for i := 1; i < len(findings); i++ {
		for j := i; j > 0 && severityOrder(findings[j].Severity) < severityOrder(findings[j-1].Severity); j-- {
			findings[j], findings[j-1] = findings[j-1], findings[j]
		}
	}
}
