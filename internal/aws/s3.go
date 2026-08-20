package aws

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/dostrow/e9s/internal/model"
)

type S3Bucket = model.S3Bucket
type S3Object = model.S3Object
type S3ObjectDetail = model.S3ObjectDetail

// ListBuckets returns all S3 buckets, optionally filtered by name substring.
func (c *Client) ListBuckets(ctx context.Context, filter string) ([]S3Bucket, error) {
	out, err := c.S3.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}

	lf := strings.ToLower(filter)
	var buckets []S3Bucket
	for _, b := range out.Buckets {
		name := derefStrAws(b.Name)
		if filter != "" && !strings.Contains(strings.ToLower(name), lf) {
			continue
		}
		var created time.Time
		if b.CreationDate != nil {
			created = *b.CreationDate
		}
		buckets = append(buckets, S3Bucket{Name: name, CreatedAt: created})
	}
	return buckets, nil
}

// ListObjects lists objects and common prefixes (folders) in a bucket under a prefix.
// Limits results to maxKeys per call to avoid expensive full-bucket listings.
// Returns a continuation token if there are more results.
func (c *Client) ListObjects(ctx context.Context, bucket, prefix string) ([]S3Object, error) {
	delimiter := "/"
	maxKeys := int32(500)
	input := &s3.ListObjectsV2Input{
		Bucket:    &bucket,
		Prefix:    &prefix,
		Delimiter: &delimiter,
		MaxKeys:   &maxKeys,
	}

	var objects []S3Object
	// Single page only — use ] to load more
	page, err := c.S3.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, err
	}
	// Common prefixes (folders)
	for _, cp := range page.CommonPrefixes {
		p := derefStrAws(cp.Prefix)
		objects = append(objects, S3Object{
			Key:      p,
			IsPrefix: true,
		})
	}
	// Objects
	for _, obj := range page.Contents {
		key := derefStrAws(obj.Key)
		// Skip the prefix itself if it appears as an object
		if key == prefix {
			continue
		}
		var lastMod time.Time
		if obj.LastModified != nil {
			lastMod = *obj.LastModified
		}
		objects = append(objects, S3Object{
			Key:          key,
			Size:         derefInt64(obj.Size),
			LastModified: lastMod,
		})
	}
	return objects, nil
}

// SearchObjects searches for objects by key prefix without a delimiter,
// returning all matching keys at any depth (not folder-grouped).
func (c *Client) SearchObjects(ctx context.Context, bucket, prefix string) ([]S3Object, error) {
	maxKeys := int32(500)
	input := &s3.ListObjectsV2Input{
		Bucket:  &bucket,
		Prefix:  &prefix,
		MaxKeys: &maxKeys,
	}

	var objects []S3Object
	page, err := c.S3.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, err
	}
	for _, obj := range page.Contents {
		key := derefStrAws(obj.Key)
		var lastMod time.Time
		if obj.LastModified != nil {
			lastMod = *obj.LastModified
		}
		objects = append(objects, S3Object{
			Key:          key,
			Size:         derefInt64(obj.Size),
			LastModified: lastMod,
		})
	}
	return objects, nil
}

// GetObjectTags returns the tags on an S3 object.
func (c *Client) GetObjectTags(ctx context.Context, bucket, key string) (map[string]string, error) {
	out, err := c.S3.GetObjectTagging(ctx, &s3.GetObjectTaggingInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, err
	}
	tags := make(map[string]string)
	for _, t := range out.TagSet {
		if t.Key != nil && t.Value != nil {
			tags[*t.Key] = *t.Value
		}
	}
	return tags, nil
}

// GetObjectDetail returns metadata and tags for an S3 object.
func (c *Client) GetObjectDetail(ctx context.Context, bucket, key string) (*S3ObjectDetail, error) {
	head, err := c.S3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, err
	}

	detail := &S3ObjectDetail{
		Key:          key,
		ContentType:  derefStrAws(head.ContentType),
		ETag:         derefStrAws(head.ETag),
		StorageClass: normalizeS3StorageClass(head.StorageClass),
	}
	if head.ContentLength != nil {
		detail.Size = *head.ContentLength
	}
	if head.LastModified != nil {
		detail.LastModified = *head.LastModified
	}

	tags, err := c.GetObjectTags(ctx, bucket, key)
	if err == nil {
		detail.Tags = tags
	}

	return detail, nil
}

// HeadObject omits the storage-class header for objects in S3 Standard.
// Preserve explicit and future SDK values while presenting that omission as
// the class it represents rather than as missing metadata.
func normalizeS3StorageClass(storageClass types.StorageClass) string {
	if storageClass == "" {
		return string(types.StorageClassStandard)
	}
	return string(storageClass)
}

// DownloadObject downloads a single S3 object to a local file path.
func (c *Client) DownloadObject(ctx context.Context, bucket, key, destPath string, progress func(int64)) error {
	out, err := c.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return err
	}
	defer out.Body.Close()

	// Ensure parent directory exists
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.CreateTemp(dir, "."+filepath.Base(destPath)+".e9s-*")
	if err != nil {
		return err
	}
	temporary := f.Name()
	complete := false
	defer func() {
		_ = f.Close()
		if !complete {
			_ = os.Remove(temporary)
		}
	}()

	writer := io.Writer(f)
	if progress != nil {
		writer = &s3ProgressWriter{writer: f, progress: progress}
	}
	if _, err = io.Copy(writer, out.Body); err != nil {
		return err
	}
	if err = f.Chmod(0644); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(temporary, destPath); err != nil {
		return err
	}
	complete = true
	return nil
}

// DownloadPrefix recursively downloads all objects under a prefix to a local directory.
// Returns the number of files downloaded.
func (c *Client) DownloadPrefix(ctx context.Context, bucket, prefix, destDir string, progress func(model.S3DownloadProgress)) (int, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: &bucket,
		Prefix: &prefix,
	}

	count := 0
	var totalBytes int64
	paginator := s3.NewListObjectsV2Paginator(c.S3, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return count, err
		}
		for _, obj := range page.Contents {
			key := derefStrAws(obj.Key)
			// Build local path: strip the prefix and join with destDir
			relPath := strings.TrimPrefix(key, prefix)
			if relPath == "" {
				continue
			}
			localPath, err := s3PrefixDestination(destDir, relPath)
			if err != nil {
				return count, err
			}
			var fileBytes int64
			if err := c.DownloadObject(ctx, bucket, key, localPath, func(bytes int64) {
				fileBytes = bytes
				if progress != nil {
					progress(model.S3DownloadProgress{CurrentKey: key, FilesCompleted: count, BytesCompleted: totalBytes + bytes})
				}
			}); err != nil {
				return count, err
			}
			count++
			totalBytes += fileBytes
			if progress != nil {
				progress(model.S3DownloadProgress{CurrentKey: key, FilesCompleted: count, BytesCompleted: totalBytes})
			}
		}
	}
	return count, nil
}

func s3PrefixDestination(destination, relative string) (string, error) {
	clean := filepath.Clean(relative)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe S3 object key path %q", relative)
	}
	return filepath.Join(destination, clean), nil
}

type s3ProgressWriter struct {
	writer   io.Writer
	total    int64
	progress func(int64)
}

func (w *s3ProgressWriter) Write(data []byte) (int, error) {
	written, err := w.writer.Write(data)
	w.total += int64(written)
	w.progress(w.total)
	return written, err
}

func derefInt64(p *int64) int64 {
	if p != nil {
		return *p
	}
	return 0
}
