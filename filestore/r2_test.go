package filestore

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	r2AccountId       = ""
	r2AccessKeyId     = ""
	r2SecretAccessKey = ""
	r2Region          = ""
	r2Bucket          = ""
)

func TestR2FileStore_UploadTextFile(t *testing.T) {
	fileStore, err := NewR2FileStore(
		r2AccountId,
		r2AccessKeyId,
		r2SecretAccessKey,
		r2Region,
		r2Bucket,
	)
	assert.NoError(t, err)

	file, err := os.Open("testdata/test.js")
	assert.NoError(t, err)

	err = fileStore.UploadFile(
		context.Background(),
		file,
		"text/javascript",
		"static/test.js",
	)
	assert.NoError(t, err)
}

func TestR2FileStore_UploadTextFileData(t *testing.T) {
	fileStore, err := NewR2FileStore(
		r2AccountId,
		r2AccessKeyId,
		r2SecretAccessKey,
		r2Region,
		r2Bucket,
	)
	assert.NoError(t, err)

	file, err := os.Open("testdata/test.js")
	assert.NoError(t, err)
	data, err := io.ReadAll(file)
	assert.NoError(t, err)

	err = fileStore.UploadFileData(
		context.Background(),
		data,
		"text/javascript",
		"static/test.js",
	)
	assert.NoError(t, err)
}

func TestR2FileStore_UploadImageFile(t *testing.T) {
	fileStore, err := NewR2FileStore(
		r2AccountId,
		r2AccessKeyId,
		r2SecretAccessKey,
		r2Region,
		r2Bucket,
	)
	assert.NoError(t, err)

	file, err := os.Open("testdata/test.png")
	assert.NoError(t, err)

	err = fileStore.UploadFile(
		context.Background(),
		file,
		"image/png",
		"static/test.png",
	)
	assert.NoError(t, err)
}

func TestR2FileStore_UploadImageFileData(t *testing.T) {
	fileStore, err := NewR2FileStore(
		r2AccountId,
		r2AccessKeyId,
		r2SecretAccessKey,
		r2Region,
		r2Bucket,
	)
	assert.NoError(t, err)

	file, err := os.Open("testdata/test.png")
	assert.NoError(t, err)
	data, err := io.ReadAll(file)
	assert.NoError(t, err)

	err = fileStore.UploadFileData(
		context.Background(),
		data,
		"image/png",
		"static/test.png",
	)
	assert.NoError(t, err)
}
