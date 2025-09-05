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

func TestRequestWithHmacSha256RequestBodySigner(t *testing.T) {
	type SoftswissRequestUrls struct {
		DepositUrl string `json:"deposit_url"`
		ReturnUrl  string `json:"return_url"`
	}
	type SoftswissRequestUser struct {
		Id           string `json:"id"`
		Email        string `json:"email"`
		Firstname    string `json:"firstname"`
		Lastname     string `json:"lastname"`
		Nickname     string `json:"nickname"`
		City         string `json:"city"`
		Country      string `json:"country"`
		DateOfBirth  string `json:"date_of_birth"`
		Gender       string `json:"gender"`
		RegisteredAt string `json:"registered_at"`
	}
	type SoftswissRequest struct {
		CasinoId     string               `json:"casino_id"`
		Game         string               `json:"game"`
		Currency     string               `json:"currency"`
		Locale       string               `json:"locale"`
		Ip           string               `json:"ip"`
		ClientType   string               `json:"client_type"`
		Urls         SoftswissRequestUrls `json:"urls"`
		Jurisdiction string               `json:"jurisdiction"`
		User         SoftswissRequestUser `json:"user"`
	}
	request := SoftswissRequest{
		CasinoId:   "minigame",
		Game:       "softswiss:ElvisFroginVegas",
		Currency:   "EUR",
		Locale:     "en",
		Ip:         "46.53.162.55",
		ClientType: "desktop",
		Urls: SoftswissRequestUrls{
			DepositUrl: "https://www.google.com",
			ReturnUrl:  "https://www.google.com",
		},
		Jurisdiction: "DE",
		User: SoftswissRequestUser{
			Id:           "550e8400-e29b-41d4-a716-446655440000",
			Email:        "user@example.com",
			Firstname:    "John",
			Lastname:     "Doe",
			Nickname:     "spinmaster",
			City:         "Berlin",
			Country:      "DE",
			DateOfBirth:  "1980-12-26",
			Gender:       "m",
			RegisteredAt: "2018-10-11",
		},
	}
	statusCode, responseBody, err := PostJson(
		context.Background(),
		"https://casino.int.a8r.games/sessions",
		request,
		WithRequestSigner(HmacSha256RequestBodySigner, HmacSha256RequestBodySignerKeys{
			RequestSignHeader: "X-REQUEST-SIGN",
			Secret:            "",
		}),
	)
	assert.NoError(t, err)
	assert.Equal(t, 2, statusCode/100)
	assert.NotEmpty(t, responseBody)
}
