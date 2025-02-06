package request

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
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

func HmacSha256Signer(req *http.Request, apiKeyHeader, apiKey, signatureHeader, apiKeySecret string) error {
	canonicalizedMessage := getCanonicalizedMessage(req)
	signature := CalculateHmacSha256Hash(canonicalizedMessage, apiKeySecret)
	req.Header.Add(apiKeyHeader, apiKey)
	req.Header.Add(signatureHeader, signature)
	return nil
}

func CalculateHmacSha256Hash(message []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(message)
	signatureBytes := h.Sum(nil)
	return hex.EncodeToString(signatureBytes)
}
