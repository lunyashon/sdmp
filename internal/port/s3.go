package port

import (
	"context"
	"io"
	"strings"
	"time"
)

type PutOptions struct {
	ContentType string
}

type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
}

type Object struct {
	Key           string
	ContentType   string
	ContentLength int64
	Body          io.ReadCloser
}

type S3 interface {
	Put(ctx context.Context, key string, body io.Reader, opts PutOptions) error
	Get(ctx context.Context, key string) (*Object, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)
	Ping(ctx context.Context) error
}

func JoinKey(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(strings.ReplaceAll(part, "\\", "/"), "/")
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return strings.Join(out, "/")
}
