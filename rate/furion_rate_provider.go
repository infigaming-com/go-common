package rate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/infigaming-com/go-common/request"
)

type furionRate struct {
	Timestamp int64  `json:"timestamp"`
	Currency  string `json:"currency"`
	Rate      string `json:"rate"`
}

type furionRateProvider struct {
	endpoint     string
	apiKey       string
	apiKeySecret string
}

type getRatesRequest struct {
	Timestamp  int64    `json:"timestamp" binding:"required"`
	Currencies []string `json:"currencies"`
}

type getRatesResponse struct {
	Rates []Rate `json:"rates"`
}

type getRatesMapResponse struct {
	Rates map[string]Rate `json:"rates"`
}

func NewFurionRateProvider(endpoint, apiKey, apiKeySecret string) (RateProvider, error) {
	// Parse the URL to extract the base URL
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint URL: %w", err)
	}

	// Reconstruct the base URL (scheme + host)
	endpoint = fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	return &furionRateProvider{
		endpoint:     endpoint,
		apiKey:       apiKey,
		apiKeySecret: apiKeySecret,
	}, nil
}

func (p *furionRateProvider) GetRates(ctx context.Context, currencies []string, timestamp int64) ([]Rate, error) {
	if timestamp == 0 {
		timestamp = time.Now().Unix()
	}
	rateRequest := &getRatesRequest{
		Timestamp:  timestamp,
		Currencies: currencies,
	}
	statusCode, responseBody, err := request.PostJson(
		ctx,
		p.endpoint+"/rates",
		rateRequest,
		request.WithRequestSigner(
			request.HmacSha256Signer,
			request.HmacSha256SignerKeys{
				ApiKeyHeader:    "X-API-KEY",
				SignatureHeader: "X-SIGNATURE",
				ApiKey:          p.apiKey,
				ApiKeySecret:    p.apiKeySecret,
			},
		),
	)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d, response: %s", statusCode, string(responseBody))
	}
	var resp getRatesResponse
	if err := json.Unmarshal(responseBody, &resp); err != nil {
		return nil, err
	}
	return resp.Rates, nil
}

func (p *furionRateProvider) GetRatesMap(ctx context.Context, currencies []string, timestamp int64) (map[string]Rate, error) {
	if timestamp == 0 {
		timestamp = time.Now().Unix()
	}
	rateRequest := &getRatesRequest{
		Timestamp:  timestamp,
		Currencies: currencies,
	}
	statusCode, responseBody, err := request.PostJson(
		ctx,
		p.endpoint+"/rates/map",
		rateRequest,
		request.WithRequestSigner(
			request.HmacSha256Signer,
			request.HmacSha256SignerKeys{
				ApiKeyHeader:    "X-API-KEY",
				SignatureHeader: "X-SIGNATURE",
				ApiKey:          p.apiKey,
				ApiKeySecret:    p.apiKeySecret,
			},
		),
	)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d, response: %s", statusCode, string(responseBody))
	}
	var resp getRatesMapResponse
	if err := json.Unmarshal(responseBody, &resp); err != nil {
		return nil, err
	}
	return resp.Rates, nil
}

func (p *furionRateProvider) GetRate(ctx context.Context, currency string, timestamp int64) (*Rate, error) {
	rates, err := p.GetRates(ctx, []string{currency}, timestamp)
	if err != nil {
		return nil, err
	}
	if len(rates) == 0 {
		return nil, fmt.Errorf("no rates found for currency: %s", currency)
	}
	return &rates[0], nil
}
