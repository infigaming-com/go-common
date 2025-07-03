package filestore

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	r2AccountId       = "ddaedd929fb29cb074cd488fc486acfe"
	r2AccessKeyId     = "424b7b30d3f075cccff372e4a268cb43"
	r2SecretAccessKey = "e2d12830529b67497cf78e48a888d4f104c906d9c5ddde5766aabedc7aeb8fff"
	r2Region          = "WEUR"
	r2Bucket          = "resources"
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
