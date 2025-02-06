package request

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequest(t *testing.T) {
	statusCode, responseBody, err := Request(
		context.Background(),
		http.MethodGet,
		"https://httpbin.org/get",
	)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.NotEmpty(t, responseBody)
}

func TestRequestWithDebugEnabled(t *testing.T) {
	statusCode, responseBody, err := Request(
		context.Background(),
		http.MethodGet,
		"https://httpbin.org/get",
		WithDebugEnabled(true),
		WithRequestSigner(HmacSha256Signer, "test-api-key", "test-api-key-secret"),
	)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.NotEmpty(t, responseBody)
}

func TestRequestWithHttpGetAndRequestSigner(t *testing.T) {
	statusCode, responseBody, err := Request(
		context.Background(),
		http.MethodGet,
		"https://httpbin.org/get",
		WithDebugEnabled(true),
		WithQueryParams(map[string]string{
			"Key-A": "Value-A",
			"Key-B": "Value-B",
		}),
		WithRequestSigner(HmacSha256Signer, "test-api-key", "test-api-key-secret"),
	)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.NotEmpty(t, responseBody)
}

func TestRequestWithHttpPostAndRequestSigner(t *testing.T) {
	statusCode, responseBody, err := Request(
		context.Background(),
		http.MethodPost,
		"https://httpbin.org/post",
		WithDebugEnabled(true),
		WithRequestBody([]byte(`{"Key-A": "Value-A", "Key-B": "Value-B"}`)),
		WithRequestSigner(HmacSha256Signer, "test-api-key", "test-api-key-secret"),
	)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.NotEmpty(t, responseBody)
}
