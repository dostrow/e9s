package model

import "testing"

func TestIsS3ArchiveAccessStatus(t *testing.T) {
	for _, status := range []string{S3ArchiveAccessStatus, S3DeepArchiveAccessStatus} {
		if !IsS3ArchiveAccessStatus(status) {
			t.Errorf("IsS3ArchiveAccessStatus(%q) = false", status)
		}
	}
	for _, status := range []string{"", "FREQUENT", "ARCHIVE_INSTANT_ACCESS"} {
		if IsS3ArchiveAccessStatus(status) {
			t.Errorf("IsS3ArchiveAccessStatus(%q) = true", status)
		}
	}
}
