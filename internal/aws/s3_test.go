package aws

import (
	"path/filepath"
	"testing"
)

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
