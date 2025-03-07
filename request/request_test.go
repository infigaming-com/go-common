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
		WithRequestSigner(HmacSha256Signer, HmacSha256SignerKeys{
			ApiKeyHeader:    "X-API-KEY",
			SignatureHeader: "X-SIGNATURE",
			ApiKey:          "test-api-key",
			ApiKeySecret:    "test-api-key-secret",
		}),
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
		WithRequestSigner(HmacSha256Signer, HmacSha256SignerKeys{
			ApiKeyHeader:    "X-API-KEY",
			SignatureHeader: "X-SIGNATURE",
			ApiKey:          "test-api-key",
			ApiKeySecret:    "test-api-key-secret",
		}),
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
		WithRequestSigner(HmacSha256Signer, HmacSha256SignerKeys{
			ApiKeyHeader:    "X-API-KEY",
			SignatureHeader: "X-SIGNATURE",
			ApiKey:          "test-api-key",
			ApiKeySecret:    "test-api-key-secret",
		}),
	)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.NotEmpty(t, responseBody)
}

func TestRequestWithQueryParamsAndRequestSigner(t *testing.T) {
	statusCode, responseBody, err := Request(
		context.Background(),
		http.MethodPost,
		"https://httpbin.org/post",
		WithDebugEnabled(true),
		WithQueryParams(map[string]string{
			"Key-A": "Value-A",
			"Key-B": "Value-B",
		}),
		WithRequestSigner(Md5QueryParametersSigner, Md5SignerKeys{Secret: "1234567890ABCDEFGH"}),
	)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.NotEmpty(t, responseBody)
}

func TestPostWithQueryParams(t *testing.T) {
	request := struct {
		SecureLogin      string `json:"secureLogin"`
		Symbol           string `json:"symbol"`
		Language         string `json:"language"`
		Token            string `json:"token"`
		ExternalPlayerId string `json:"externalPlayerId"`
		Currency         string `json:"currency"`
	}{
		SecureLogin:      "zf723_infigming",
		Symbol:           "vs10bhallbnza2",
		Language:         "en",
		Token:            "1234567890ABCDEFGH",
		ExternalPlayerId: "ABCD_USD",
		Currency:         "USD",
	}
	statusCode, responseBody, err := PostFormEncoded(
		context.Background(),
		"https://httpbin.org/post",
		request,
		WithDebugEnabled(true),
		WithRequestSigner(Md5QueryParametersSigner, Md5SignerKeys{Secret: "1234567890ABCDEFGH"}),
	)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.NotEmpty(t, responseBody)
}
