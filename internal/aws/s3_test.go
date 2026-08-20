package aws

import (
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestNormalizeS3StorageClass(t *testing.T) {
	for _, test := range []struct {
		name  string
		value types.StorageClass
		want  string
	}{
		{name: "standard is omitted by HeadObject", want: "STANDARD"},
		{name: "explicit class", value: types.StorageClassDeepArchive, want: "DEEP_ARCHIVE"},
		{name: "future class", value: types.StorageClass("FUTURE_CLASS"), want: "FUTURE_CLASS"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeS3StorageClass(test.value); got != test.want {
				t.Fatalf("normalizeS3StorageClass(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestS3PrefixDestinationStaysWithinSelectedDirectory(t *testing.T) {
	destination := filepath.Join("tmp", "download")
	got, err := s3PrefixDestination(destination, filepath.Join("reports", "summary.csv"))
	if err != nil || got != filepath.Join(destination, "reports", "summary.csv") {
		t.Fatalf("s3PrefixDestination() = %q, %v", got, err)
	}
	for _, relative := range []string{"../escape", filepath.Join("..", "escape"), string(filepath.Separator) + "absolute"} {
		if _, err := s3PrefixDestination(destination, relative); err == nil {
			t.Fatalf("s3PrefixDestination(%q) accepted an unsafe path", relative)
		}
	}
}

func TestS3ProgressWriterReportsCumulativeBytes(t *testing.T) {
	var updates []int64
	writer := &s3ProgressWriter{writer: &discardWriter{}, progress: func(total int64) { updates = append(updates, total) }}
	_, _ = writer.Write([]byte("abc"))
	_, _ = writer.Write([]byte("de"))
	if len(updates) != 2 || updates[0] != 3 || updates[1] != 5 {
		t.Fatalf("progress updates = %#v", updates)
	}
}

type discardWriter struct{}

func (*discardWriter) Write(data []byte) (int, error) { return len(data), nil }
