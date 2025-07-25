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
	"net/http"
	"sort"

	"github.com/golang-jwt/jwt/v5"
	"github.com/infigaming-com/go-common/util"
)

type RequestSigner func(requestSigningData RequestSigningData, keys any) error

type RequestSigningData struct {
	Method         string
	Url            string
	QueryParams    map[string]string
	RequestHeaders map[string]string
	RequestBody    *[]byte
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

func getHmacSha256SignerCanonicalizedMessage(requestSigningData RequestSigningData) []byte {
	if requestSigningData.Method == http.MethodGet {
		queryParams := requestSigningData.QueryParams

		// Create a slice to store and sort the parameters
		paramPairs := make([]string, 0, len(queryParams))
		for key, value := range queryParams {
			paramPairs = append(paramPairs, key+"="+value)
		}

		// Sort the parameters alphabetically
		sort.Strings(paramPairs)

		// Write the sorted parameters to the buffer
		var formattedParams bytes.Buffer
		for i, pair := range paramPairs {
			if i > 0 {
				formattedParams.WriteString("&")
			}
			formattedParams.WriteString(pair)
		}

		return formattedParams.Bytes()
	}

	return *requestSigningData.RequestBody
}

func HmacSha256Signer(requestSigningData RequestSigningData, keys any) error {
	hmacSha256SignerKeys, ok := keys.(HmacSha256SignerKeys)
	if !ok {
		return fmt.Errorf("invalid signer keys for hmac sha256 signer: %v", keys)
	}
	canonicalizedMessage := getHmacSha256SignerCanonicalizedMessage(requestSigningData)
	requestSigningData.RequestHeaders[hmacSha256SignerKeys.ApiKeyHeader] = hmacSha256SignerKeys.ApiKey
	signature := util.HmacSha256Hash(canonicalizedMessage, []byte(hmacSha256SignerKeys.ApiKeySecret))
	requestSigningData.RequestHeaders[hmacSha256SignerKeys.SignatureHeader] = hex.EncodeToString(signature)
	return nil
}

func getMd5QueryParametersSignerCanonicalizedMessage(requestSigningData RequestSigningData) []byte {
	queryParams := requestSigningData.QueryParams

	// Create a slice to store and sort the parameters
	paramPairs := make([]string, 0, len(queryParams))
	for key, value := range queryParams {
		paramPairs = append(paramPairs, key+"="+value)
	}

	// Sort the parameters alphabetically
	sort.Strings(paramPairs)

	// Write the sorted parameters to the buffer
	var formattedParams bytes.Buffer
	for i, pair := range paramPairs {
		if i > 0 {
			formattedParams.WriteString("&")
		}
		formattedParams.WriteString(pair)
	}

	return formattedParams.Bytes()
}

func Md5QueryParametersSigner(requestSigningData RequestSigningData, keys any) error {
	md5SignerKeys, ok := keys.(Md5SignerKeys)
	if !ok {
		return fmt.Errorf("invalid signer keys for md5 query parameters signer: %v", keys)
	}
	canonicalizedMessage := getMd5QueryParametersSignerCanonicalizedMessage(requestSigningData)
	messageWithSecret := append(canonicalizedMessage, []byte(md5SignerKeys.Secret)...)
	hash := md5.Sum(messageWithSecret)
	hashHex := hex.EncodeToString(hash[:])

	// Add hash to query parameters instead of body
	if requestSigningData.QueryParams == nil {
		requestSigningData.QueryParams = make(map[string]string)
	}
	requestSigningData.QueryParams["hash"] = hashHex

	return nil
}

func JwtSigner(requestSigningData RequestSigningData, keys any) error {
	jwtSignerKeys, ok := keys.(JwtSignerKeys)
	if !ok {
		return fmt.Errorf("invalid signer keys for jwt signer: %v", keys)
	}

	base64DecodedPrivateKey, err := base64.StdEncoding.DecodeString(jwtSignerKeys.PrivateKey)
	if err != nil {
		return fmt.Errorf("invalid private key for jwt signer: %v", err)
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(base64DecodedPrivateKey)
	if err != nil {
		return fmt.Errorf("invalid private key for jwt signer: %v", err)
	}

	rsaPrivateKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return fmt.Errorf("invalid private key for jwt signer: %v", jwtSignerKeys)
	}

	var claims jwt.MapClaims
	if err := json.Unmarshal(*requestSigningData.RequestBody, &claims); err != nil {
		return fmt.Errorf("invalid request body for jwt signer: %v", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(rsaPrivateKey)
	if err != nil {
		return fmt.Errorf("invalid request body for jwt signer: %v", err)
	}

	requestSigningData.RequestHeaders[jwtSignerKeys.ApiKeyHeader] = jwtSignerKeys.ApiKey
	requestSigningData.RequestHeaders[jwtSignerKeys.SignatureHeader] = tokenString

	return nil
}
