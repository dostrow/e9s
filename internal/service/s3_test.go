package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

type fakeS3API struct {
	buckets         []model.S3Bucket
	objects         []model.S3Object
	detail          *model.S3ObjectDetail
	err             error
	bucket          string
	prefix          string
	downloadRequest model.S3DownloadRequest
	prefixCount     int
}

func (f *fakeS3API) ListBuckets(context.Context, string) ([]model.S3Bucket, error) {
	return append([]model.S3Bucket(nil), f.buckets...), f.err
}
func (f *fakeS3API) ListObjects(_ context.Context, bucket, prefix string) ([]model.S3Object, error) {
	f.bucket, f.prefix = bucket, prefix
	return append([]model.S3Object(nil), f.objects...), f.err
}
func (f *fakeS3API) SearchObjects(_ context.Context, bucket, prefix string) ([]model.S3Object, error) {
	f.bucket, f.prefix = bucket, prefix
	return append([]model.S3Object(nil), f.objects...), f.err
}
func (f *fakeS3API) GetObjectDetail(_ context.Context, bucket, key string) (*model.S3ObjectDetail, error) {
	f.bucket, f.prefix = bucket, key
	return f.detail, f.err
}
func (f *fakeS3API) DownloadObject(_ context.Context, bucket, key, destination string, progress func(int64)) error {
	f.downloadRequest = model.S3DownloadRequest{Bucket: bucket, Key: key, Destination: destination}
	if progress != nil {
		progress(42)
	}
	return f.err
}
func (f *fakeS3API) DownloadPrefix(_ context.Context, bucket, key, destination string, progress func(model.S3DownloadProgress)) (int, error) {
	f.downloadRequest = model.S3DownloadRequest{Bucket: bucket, Key: key, Destination: destination, IsPrefix: true}
	if progress != nil {
		progress(model.S3DownloadProgress{CurrentKey: key + "file", FilesCompleted: f.prefixCount, BytesCompleted: 84})
	}
	return f.prefixCount, f.err
}

func TestS3BucketsFilterAndSort(t *testing.T) {
	api := &fakeS3API{buckets: []model.S3Bucket{{Name: "z-archive"}, {Name: "API-data"}, {Name: "api-backup"}}}
	got, err := NewS3(api).Buckets(context.Background(), " API ")
	if err != nil {
		t.Fatal(err)
	}
	want := []model.S3Bucket{{Name: "api-backup"}, {Name: "API-data"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Buckets() = %#v, want %#v", got, want)
	}
}

func TestS3DownloadReportsProgress(t *testing.T) {
	api := &fakeS3API{prefixCount: 3}
	var updates []model.S3DownloadProgress
	_, err := NewS3(api).DownloadWithProgress(context.Background(), model.S3DownloadRequest{
		Bucket: "bucket", Key: "path/", Destination: "/tmp/path", IsPrefix: true,
	}, func(progress model.S3DownloadProgress) { updates = append(updates, progress) })
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates[0].FilesCompleted != 3 || updates[0].BytesCompleted != 84 {
		t.Fatalf("progress = %#v", updates)
	}
}

func TestS3ObjectsSortPrefixesBeforeObjects(t *testing.T) {
	api := &fakeS3API{objects: []model.S3Object{
		{Key: "root/z.txt"}, {Key: "root/b/", IsPrefix: true},
		{Key: "root/A.txt"}, {Key: "root/a/", IsPrefix: true},
	}}
	got, err := NewS3(api).Objects(context.Background(), " bucket ", "root/")
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]string, len(got))
	for i := range got {
		keys[i] = got[i].Key
	}
	if want := []string{"root/a/", "root/b/", "root/A.txt", "root/z.txt"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("Objects() keys = %#v, want %#v", keys, want)
	}
	if api.bucket != "bucket" || api.prefix != "root/" {
		t.Fatalf("API scope = %q %q", api.bucket, api.prefix)
	}
}

func TestS3DetailAndDownloadValidation(t *testing.T) {
	service := NewS3(&fakeS3API{})
	if _, err := service.Detail(context.Background(), "", "key"); err == nil {
		t.Fatal("Detail() accepted an empty bucket")
	}
	if _, err := service.Download(context.Background(), model.S3DownloadRequest{Bucket: "bucket", Key: "key"}); err == nil {
		t.Fatal("Download() accepted an empty destination")
	}
}

func TestS3DownloadRoutesObjectsAndPrefixes(t *testing.T) {
	api := &fakeS3API{prefixCount: 3}
	service := NewS3(api)
	object := model.S3DownloadRequest{Bucket: "bucket", Key: "path/file", Destination: "/tmp/file"}
	result, err := service.Download(context.Background(), object)
	if err != nil || result.Files != 1 || api.downloadRequest != object {
		t.Fatalf("object download = %#v, %v; request %#v", result, err, api.downloadRequest)
	}
	prefix := model.S3DownloadRequest{Bucket: "bucket", Key: "path/", Destination: "/tmp/path", IsPrefix: true}
	result, err = service.Download(context.Background(), prefix)
	if err != nil || result.Files != 3 || api.downloadRequest != prefix {
		t.Fatalf("prefix download = %#v, %v; request %#v", result, err, api.downloadRequest)
	}
}

func TestS3WrapsContextualErrors(t *testing.T) {
	service := NewS3(&fakeS3API{err: errors.New("denied")})
	if _, err := service.Buckets(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "list S3 buckets") {
		t.Fatalf("Buckets() error = %v", err)
	}
	if _, err := service.Objects(context.Background(), "bucket", ""); err == nil || !strings.Contains(err.Error(), `in "bucket"`) {
		t.Fatalf("Objects() error = %v", err)
	}
}
