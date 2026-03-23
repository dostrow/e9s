package views

import (
	"fmt"
	"strings"

	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type S3DetailModel struct {
	detail *aws.S3ObjectDetail
	bucket string
	width  int
	height int
}

func NewS3Detail(bucket string, detail *aws.S3ObjectDetail) S3DetailModel {
	return S3DetailModel{detail: detail, bucket: bucket}
}

func (m S3DetailModel) View() string {
	if m.detail == nil {
		return theme.HelpStyle.Render("  Loading...")
	}

	d := m.detail
	var b strings.Builder

	title := fmt.Sprintf("  s3://%s/%s", m.bucket, d.Key)
	b.WriteString(theme.TitleStyle.Render(title))
	b.WriteString("\n\n")

	fmt.Fprintf(&b, "  %-18s %s\n", "Key:", d.Key)
	fmt.Fprintf(&b, "  %-18s %s\n", "Size:", formatBytesS3(d.Size))
	fmt.Fprintf(&b, "  %-18s %s\n", "Content Type:", d.ContentType)
	fmt.Fprintf(&b, "  %-18s %s\n", "ETag:", d.ETag)
	fmt.Fprintf(&b, "  %-18s %s\n", "Storage Class:", d.StorageClass)
	if !d.LastModified.IsZero() {
		fmt.Fprintf(&b, "  %-18s %s (%s ago)\n", "Last Modified:", d.LastModified.Format("2006-01-02 15:04:05"), formatAge(d.LastModified))
	}

	if len(d.Tags) > 0 {
		b.WriteString("\n")
		b.WriteString(theme.TitleStyle.Render("  Tags"))
		b.WriteString("\n\n")
		for k, v := range d.Tags {
			fmt.Fprintf(&b, "    %s = %s\n",
				theme.HeaderStyle.Render(k), v)
		}
	}

	return b.String()
}

func (m S3DetailModel) Detail() *aws.S3ObjectDetail {
	return m.detail
}

func (m S3DetailModel) Bucket() string {
	return m.bucket
}

func (m S3DetailModel) SetSize(w, h int) S3DetailModel {
	m.width = w
	m.height = h
	return m
}
