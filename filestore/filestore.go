package filestore

import (
	"context"
	"io"
)

type FileStore interface {
	UploadFileData(ctx context.Context, data []byte, contentType, key string) error
	UploadFile(ctx context.Context, reader io.Reader, contentType, key string) error
}
