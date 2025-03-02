package rate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/infigaming-com/go-common/request"
	"go.uber.org/zap"
)

type furionRateProvider struct {
	lg           *zap.Logger
	url          string
	apiKey       string
	apiKeySecret string
}

type getRatesRequest struct {
	Timestamp  int64    `json:"timestamp" binding:"required"`
	Currencies []string `json:"currencies"`
}

func NewFurionRateProvider(lg *zap.Logger, url, apiKey, apiKeySecret string) RateProvider {
	return &furionRateProvider{
		lg:           lg,
		url:          url,
		apiKey:       apiKey,
		apiKeySecret: apiKeySecret,
	}
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
		p.url,
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
	var rates []Rate
	if err := json.Unmarshal(responseBody, &rates); err != nil {
		return nil, err
	}
	return rates, nil
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
