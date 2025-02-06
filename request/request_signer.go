package request

import (
	"bytes"
	"encoding/hex"
	"io"
	"net/http"

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

func HmacSha256Signer(req *http.Request, apiKeyHeader, apiKey, signatureHeader, apiKeySecret string) error {
	canonicalizedMessage := getCanonicalizedMessage(req)
	req.Header.Add(apiKeyHeader, apiKey)
	signature := util.HmacSha256Hash(canonicalizedMessage, []byte(apiKeySecret))
	req.Header.Add(signatureHeader, hex.EncodeToString(signature))
	return nil
}
