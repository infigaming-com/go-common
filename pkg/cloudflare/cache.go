package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

var (
	apiBaseURL = "https://api.cloudflare.com"
	httpClient = &http.Client{Timeout: 5 * time.Second}
)

type purgeRequest struct {
	Files []string `json:"files"`
}

type purgeResponse struct {
	Success  bool             `json:"success"`
	Errors   []responseDetail `json:"errors"`
	Messages []responseDetail `json:"messages"`
}

type responseDetail struct {
	Message string `json:"message"`
}

// PurgeCloudflareCache clears Cloudflare cached copies of the provided file URLs using the supplied API token and zone ID.
// Example:
//
//	err := cloudflare.PurgeCloudflareCache(ctx, "<API_TOKEN>", "<ZONE_ID>", []string{
//	    "https://example.com/script.js",
//	    "https://example.com/style.css",
//	})
//
//	if err != nil {
//	    log.Fatal(err)
//	}
func PurgeCloudflareCache(ctx context.Context, apiToken, zoneID string, files []string) error {
	logger := zap.L()

	if ctx == nil {
		return errors.New("context must not be nil")
	}

	if strings.TrimSpace(apiToken) == "" {
		return errors.New("cloudflare API token must not be empty")
	}

	if strings.TrimSpace(zoneID) == "" {
		return errors.New("cloudflare zone ID must not be empty")
	}

	if len(files) == 0 {
		return errors.New("files must not be empty")
	}

	payload := purgeRequest{Files: files}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("cloudflare purge marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/client/v4/zones/%s/purge_cache", strings.TrimRight(apiBaseURL, "/"), zoneID)

	attempts := 2
	for attempt := 1; attempt <= attempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("cloudflare purge create request: %w", err)
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiToken))
		req.Header.Set("Content-Type", "application/json")

		logger.Info("purging cloudflare cache",
			zap.String("zone_id", zoneID),
			zap.Int("file_count", len(files)),
			zap.Int("attempt", attempt),
		)

		resp, err := httpClient.Do(req)
		if err != nil {
			if attempt < attempts && shouldRetry(err) {
				logger.Warn("retrying cloudflare cache purge after transient error",
					zap.Error(err),
					zap.Int("attempt", attempt),
				)
				continue
			}
			return fmt.Errorf("cloudflare purge execute request: %w", err)
		}

		responseBody, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if closeErr != nil {
			logger.Warn("failed to close cloudflare response body", zap.Error(closeErr))
		}
		if readErr != nil {
			return fmt.Errorf("cloudflare purge read response: %w", readErr)
		}

		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			apiErr := extractAPIError(responseBody)
			return fmt.Errorf("cloudflare purge unexpected status %d: %s", resp.StatusCode, apiErr)
		}

		var parsed purgeResponse
		if err := json.Unmarshal(responseBody, &parsed); err != nil {
			return fmt.Errorf("cloudflare purge decode response: %w", err)
		}

		if !parsed.Success {
			apiErr := extractFailureMessage(parsed)
			return fmt.Errorf("cloudflare purge unsuccessful: %s", apiErr)
		}

		logger.Info("cloudflare cache purge succeeded",
			zap.String("zone_id", zoneID),
			zap.Int("file_count", len(files)),
		)
		return nil
	}

	return errors.New("cloudflare purge exhausted retries")
}

func shouldRetry(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}
		if temporary, ok := interface{}(netErr).(interface{ Temporary() bool }); ok {
			return temporary.Temporary()
		}
	}
	return false
}

func extractAPIError(body []byte) string {
	var parsed purgeResponse
	if err := json.Unmarshal(body, &parsed); err == nil {
		message := extractFailureMessage(parsed)
		if message != "" {
			return message
		}
	}
	return string(body)
}

func extractFailureMessage(resp purgeResponse) string {
	for _, detail := range resp.Errors {
		if detail.Message != "" {
			return detail.Message
		}
	}
	for _, detail := range resp.Messages {
		if detail.Message != "" {
			return detail.Message
		}
	}
	return ""
}
