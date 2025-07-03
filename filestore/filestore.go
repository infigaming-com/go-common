package filestore

import (
	"context"
	"io"
)

type FileStore interface {
	UploadFile(ctx context.Context, key string, reader io.Reader) error
}
