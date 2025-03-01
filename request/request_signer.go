package request

import (
	"bytes"
	"crypto/md5"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/golang-jwt/jwt/v5"
	"github.com/infigaming-com/go-common/util"
)

type requestSigner func(req *http.Request, keys any) error

type HmacSha256SignerKeys struct {
	ApiKeyHeader    string
	SignatureHeader string
	ApiKey          string
	ApiKeySecret    string
}

type Md5SignerKeys struct {
	Secret string
}

type JwtSignerKeys struct {
	ApiKeyHeader    string
	SignatureHeader string
	ApiKey          string
	PrivateKey      string
}

const (
	ApiKeyHeader    = "X-API-KEY"
	SignatureHeader = "X-SIGNATURE"
)

func getCanonicalizedMessage(req *http.Request) []byte {
	if req.Method == http.MethodGet {
		queryParams := req.URL.Query()
		var formattedParams bytes.Buffer
		for key, value := range queryParams {
			formattedParams.WriteString(key + "=" + value[0] + "&")
		}
		if formattedParams.Len() > 0 {
			formattedParams.Truncate(formattedParams.Len() - 1) // Remove the last &
		}
		return formattedParams.Bytes()
	}

	var requestBody []byte
	if req.Body != nil {
		requestBody, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	}

	return requestBody
}

func getFormEncodedCanonicalizedMessage(req *http.Request) []byte {
	var requestBody []byte
	if req.Body != nil {
		requestBody, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	}

	if len(requestBody) == 0 {
		return []byte{}
	}

	values, err := url.ParseQuery(string(requestBody))
	if err != nil {
		return []byte{}
	}

	return []byte(values.Encode())
}

func HmacSha256Signer(req *http.Request, keys any) error {
	hmacSha256SignerKeys, ok := keys.(HmacSha256SignerKeys)
	if !ok {
		return NewRequestError(ErrCodeInvalidSignerKeys, "invalid signer keys", nil, keys)
	}
	canonicalizedMessage := getCanonicalizedMessage(req)
	req.Header.Add(hmacSha256SignerKeys.ApiKeyHeader, hmacSha256SignerKeys.ApiKey)
	signature := util.HmacSha256Hash(canonicalizedMessage, []byte(hmacSha256SignerKeys.ApiKeySecret))
	req.Header.Add(hmacSha256SignerKeys.SignatureHeader, hex.EncodeToString(signature))
	return nil
}

func Md5WithSecretSigner(req *http.Request, keys any) error {
	md5SignerKeys, ok := keys.(Md5SignerKeys)
	if !ok {
		return NewRequestError(ErrCodeInvalidSignerKeys, "invalid signer keys", nil, keys)
	}
	canonicalizedMessage := getFormEncodedCanonicalizedMessage(req)
	messageWithSecret := append(canonicalizedMessage, []byte(md5SignerKeys.Secret)...)
	hash := md5.Sum(messageWithSecret)
	hashHex := hex.EncodeToString(hash[:])

	var formValues url.Values
	if len(canonicalizedMessage) > 0 {
		var err error
		formValues, err = url.ParseQuery(string(canonicalizedMessage))
		if err != nil {
			return fmt.Errorf("failed to parse form data: %w", err)
		}
	} else {
		formValues = url.Values{}
	}

	formValues.Set("hash", hashHex)

	newBody := formValues.Encode()
	req.Body = io.NopCloser(bytes.NewBufferString(newBody))
	req.ContentLength = int64(len(newBody))

	return nil
}

func JwtSigner(req *http.Request, keys any) error {
	jwtSignerKeys, ok := keys.(JwtSignerKeys)
	if !ok {
		return NewRequestError(ErrCodeInvalidSignerKeys, "invalid signer keys", nil, keys)
	}

	base64DecodedPrivateKey, err := base64.StdEncoding.DecodeString(jwtSignerKeys.PrivateKey)
	if err != nil {
		return NewRequestError(ErrCodeInvalidSignerKeys, "invalid private key", err, jwtSignerKeys)
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(base64DecodedPrivateKey)
	if err != nil {
		return NewRequestError(ErrCodeInvalidSignerKeys, "invalid private key", err, jwtSignerKeys)
	}

	rsaPrivateKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return NewRequestError(ErrCodeInvalidSignerKeys, "invalid private key", nil, jwtSignerKeys)
	}

	var requestBody []byte
	if req.Body != nil {
		requestBody, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	}

	var claims jwt.MapClaims
	if err := json.Unmarshal(requestBody, &claims); err != nil {
		return NewRequestError(ErrCodeInvalidRequestBody, "invalid request body", err, jwtSignerKeys)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(rsaPrivateKey)
	if err != nil {
		return NewRequestError(ErrCodeInvalidRequestBody, "invalid request body", err, jwtSignerKeys)
	}

	req.Header.Add(jwtSignerKeys.ApiKeyHeader, jwtSignerKeys.ApiKey)
	req.Header.Add(jwtSignerKeys.SignatureHeader, tokenString)

	return nil
}
