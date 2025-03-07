package request

import (
	"bytes"
	"crypto/md5"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/golang-jwt/jwt/v5"
	"github.com/infigaming-com/go-common/util"
)

type requestSigner func(requestSigningData requestSigningData, keys any) error

type requestSigningData struct {
	method         string
	url            string
	queryParams    map[string]string
	requestHeaders map[string]string
	requestBody    []byte
}

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

func getHmacSha256SignerCanonicalizedMessage(requestSigningData requestSigningData) []byte {
	if requestSigningData.method == http.MethodGet {
		queryParams := requestSigningData.queryParams
		var formattedParams bytes.Buffer
		for key, value := range queryParams {
			formattedParams.WriteString(key + "=" + value + "&")
		}
		if formattedParams.Len() > 0 {
			formattedParams.Truncate(formattedParams.Len() - 1) // Remove the last &
		}
		return formattedParams.Bytes()
	}

	return requestSigningData.requestBody
}

func HmacSha256Signer(requestSigningData requestSigningData, keys any) error {
	hmacSha256SignerKeys, ok := keys.(HmacSha256SignerKeys)
	if !ok {
		return NewRequestError(ErrCodeInvalidSignerKeys, "invalid signer keys", nil, keys)
	}
	canonicalizedMessage := getHmacSha256SignerCanonicalizedMessage(requestSigningData)
	requestSigningData.requestHeaders[hmacSha256SignerKeys.ApiKeyHeader] = hmacSha256SignerKeys.ApiKey
	signature := util.HmacSha256Hash(canonicalizedMessage, []byte(hmacSha256SignerKeys.ApiKeySecret))
	requestSigningData.requestHeaders[hmacSha256SignerKeys.SignatureHeader] = hex.EncodeToString(signature)
	return nil
}

func getMd5QueryParametersSignerCanonicalizedMessage(requestSigningData requestSigningData) []byte {
	// Create url.Values from queryParams map
	values := url.Values{}
	for key, value := range requestSigningData.queryParams {
		values.Set(key, value)
	}

	return []byte(values.Encode())
}

func Md5QueryParametersSigner(requestSigningData requestSigningData, keys any) error {
	md5SignerKeys, ok := keys.(Md5SignerKeys)
	if !ok {
		return NewRequestError(ErrCodeInvalidSignerKeys, "invalid signer keys", nil, keys)
	}
	canonicalizedMessage := getMd5QueryParametersSignerCanonicalizedMessage(requestSigningData)
	messageWithSecret := append(canonicalizedMessage, []byte(md5SignerKeys.Secret)...)
	hash := md5.Sum(messageWithSecret)
	hashHex := hex.EncodeToString(hash[:])

	// Add hash to query parameters instead of body
	if requestSigningData.queryParams == nil {
		requestSigningData.queryParams = make(map[string]string)
	}
	requestSigningData.queryParams["hash"] = hashHex

	return nil
}

func JwtSigner(requestSigningData requestSigningData, keys any) error {
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

	var claims jwt.MapClaims
	if err := json.Unmarshal(requestSigningData.requestBody, &claims); err != nil {
		return NewRequestError(ErrCodeInvalidRequestBody, "invalid request body", err, jwtSignerKeys)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(rsaPrivateKey)
	if err != nil {
		return NewRequestError(ErrCodeInvalidRequestBody, "invalid request body", err, jwtSignerKeys)
	}

	requestSigningData.requestHeaders[jwtSignerKeys.ApiKeyHeader] = jwtSignerKeys.ApiKey
	requestSigningData.requestHeaders[jwtSignerKeys.SignatureHeader] = tokenString

	return nil
}
