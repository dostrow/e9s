package model

import "time"

// S3Bucket is the UI-neutral summary used by bucket browsers.
type S3Bucket struct {
	Name      string
	CreatedAt time.Time
}

// S3Object represents either an object or a common-prefix folder.
type S3Object struct {
	Key          string
	Size         int64
	LastModified time.Time
	IsPrefix     bool
}

// S3ObjectDetail contains metadata and tags for a single object.
type S3ObjectDetail struct {
	Key          string
	Size         int64
	LastModified time.Time
	ContentType  string
	ETag         string
	StorageClass string
	Tags         map[string]string
}

// S3DownloadRequest describes a single object or recursive-prefix download.
type S3DownloadRequest struct {
	Bucket      string
	Key         string
	Destination string
	IsPrefix    bool
}

// S3DownloadResult reports the completed local transfer.
type S3DownloadResult struct {
	Destination string
	Files       int
}
