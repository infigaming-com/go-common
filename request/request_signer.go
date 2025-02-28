package request

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/infigaming-com/go-common/util"
)

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

func HmacSha256Signer(req *http.Request, apiKeyHeader, apiKey, signatureHeader, apiKeySecret string) error {
	canonicalizedMessage := getCanonicalizedMessage(req)
	req.Header.Add(apiKeyHeader, apiKey)
	signature := util.HmacSha256Hash(canonicalizedMessage, []byte(apiKeySecret))
	req.Header.Add(signatureHeader, hex.EncodeToString(signature))
	return nil
}

func Md5WithSecretSigner(req *http.Request, apiKeyHeader, apiKey, signatureHeader, apiKeySecret string) error {
	canonicalizedMessage := getFormEncodedCanonicalizedMessage(req)
	messageWithSecret := append(canonicalizedMessage, []byte(apiKeySecret)...)
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
