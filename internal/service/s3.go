package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dostrow/e9s/internal/model"
)

// S3API is the low-level object-storage behavior shared by both frontends.
type S3API interface {
	ListBuckets(context.Context, string) ([]model.S3Bucket, error)
	ListObjects(context.Context, string, string) ([]model.S3Object, error)
	SearchObjects(context.Context, string, string) ([]model.S3Object, error)
	GetObjectDetail(context.Context, string, string) (*model.S3ObjectDetail, error)
	DownloadObject(context.Context, string, string, string) error
	DownloadPrefix(context.Context, string, string, string) (int, error)
}

// S3 centralizes validation, ordering, filtering, and download routing.
type S3 struct{ api S3API }

func NewS3(api S3API) *S3 { return &S3{api: api} }

func (s *S3) Buckets(ctx context.Context, filter string) ([]model.S3Bucket, error) {
	buckets, err := s.api.ListBuckets(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("list S3 buckets: %w", err)
	}
	filter = strings.ToLower(strings.TrimSpace(filter))
	result := make([]model.S3Bucket, 0, len(buckets))
	for _, bucket := range buckets {
		if filter == "" || strings.Contains(strings.ToLower(bucket.Name), filter) {
			result = append(result, bucket)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result, nil
}

func (s *S3) Objects(ctx context.Context, bucket, prefix string) ([]model.S3Object, error) {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return nil, fmt.Errorf("list S3 objects: bucket is required")
	}
	objects, err := s.api.ListObjects(ctx, bucket, prefix)
	if err != nil {
		return nil, fmt.Errorf("list S3 objects in %q: %w", bucket, err)
	}
	sortS3Objects(objects)
	return objects, nil
}

func (s *S3) Search(ctx context.Context, bucket, prefix string) ([]model.S3Object, error) {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return nil, fmt.Errorf("search S3 objects: bucket is required")
	}
	objects, err := s.api.SearchObjects(ctx, bucket, prefix)
	if err != nil {
		return nil, fmt.Errorf("search S3 objects in %q: %w", bucket, err)
	}
	sortS3Objects(objects)
	return objects, nil
}

func (s *S3) Detail(ctx context.Context, bucket, key string) (*model.S3ObjectDetail, error) {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" || key == "" {
		return nil, fmt.Errorf("read S3 object detail: bucket and key are required")
	}
	detail, err := s.api.GetObjectDetail(ctx, bucket, key)
	if err != nil {
		return nil, fmt.Errorf("read s3://%s/%s: %w", bucket, key, err)
	}
	if detail == nil {
		return nil, fmt.Errorf("read s3://%s/%s: object was not found", bucket, key)
	}
	return detail, nil
}

func (s *S3) Download(ctx context.Context, request model.S3DownloadRequest) (model.S3DownloadResult, error) {
	request.Bucket = strings.TrimSpace(request.Bucket)
	request.Destination = strings.TrimSpace(request.Destination)
	if request.Bucket == "" || request.Key == "" || request.Destination == "" {
		return model.S3DownloadResult{}, fmt.Errorf("download S3 object: bucket, key, and destination are required")
	}
	result := model.S3DownloadResult{Destination: request.Destination, Files: 1}
	if request.IsPrefix {
		count, err := s.api.DownloadPrefix(ctx, request.Bucket, request.Key, request.Destination)
		if err != nil {
			return model.S3DownloadResult{}, fmt.Errorf("download s3://%s/%s: %w", request.Bucket, request.Key, err)
		}
		result.Files = count
		return result, nil
	}
	if err := s.api.DownloadObject(ctx, request.Bucket, request.Key, request.Destination); err != nil {
		return model.S3DownloadResult{}, fmt.Errorf("download s3://%s/%s: %w", request.Bucket, request.Key, err)
	}
	return result, nil
}

func sortS3Objects(objects []model.S3Object) {
	sort.SliceStable(objects, func(i, j int) bool {
		if objects[i].IsPrefix != objects[j].IsPrefix {
			return objects[i].IsPrefix
		}
		return strings.ToLower(objects[i].Key) < strings.ToLower(objects[j].Key)
	})
}
