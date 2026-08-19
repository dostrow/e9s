package ui

import (
	"context"
	"testing"

	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
)

type sharedS3APIFake struct {
	bucket string
	prefix string
}

func (*sharedS3APIFake) ListBuckets(context.Context, string) ([]model.S3Bucket, error) {
	return nil, nil
}

func (f *sharedS3APIFake) ListObjects(_ context.Context, bucket, prefix string) ([]model.S3Object, error) {
	f.bucket, f.prefix = bucket, prefix
	return []model.S3Object{{Key: prefix + "folder/", IsPrefix: true}, {Key: prefix + "object.txt"}}, nil
}

func (*sharedS3APIFake) SearchObjects(context.Context, string, string) ([]model.S3Object, error) {
	return nil, nil
}

func (*sharedS3APIFake) GetObjectDetail(context.Context, string, string) (*model.S3ObjectDetail, error) {
	return nil, nil
}

func (*sharedS3APIFake) DownloadObject(context.Context, string, string, string, func(int64)) error {
	return nil
}

func (*sharedS3APIFake) DownloadPrefix(context.Context, string, string, string, func(model.S3DownloadProgress)) (int, error) {
	return 0, nil
}

func TestTUIS3ObjectBrowserUsesSharedService(t *testing.T) {
	api := &sharedS3APIFake{}
	app := App{ctx: context.Background(), s3: service.NewS3(api), width: 120, height: 40}
	updated, command := app.openS3Objects("archive", "reports/")
	if command == nil || updated.state != viewS3Objects {
		t.Fatalf("openS3Objects() state = %v, command nil = %v", updated.state, command == nil)
	}
	raw := command()
	message, ok := raw.(s3ObjectsLoadedMsg)
	if !ok {
		t.Fatalf("openS3Objects command returned %T", raw)
	}
	if api.bucket != "archive" || api.prefix != "reports/" {
		t.Fatalf("shared service scope = %q %q", api.bucket, api.prefix)
	}
	if len(message.objects) != 2 || !message.objects[0].IsPrefix {
		t.Fatalf("loaded objects = %#v", message.objects)
	}
}
