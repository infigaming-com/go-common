package filestore

import (
	"context"
	"io"
)

type FileStore interface {
	UploadFile(ctx context.Context, reader io.Reader, contentType, key string) error
}
