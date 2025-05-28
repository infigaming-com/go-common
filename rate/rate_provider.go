package rate

import (
	"context"

	"github.com/shopspring/decimal"
)

type Rate struct {
	Currency  string          `json:"currency"`
	Rate      decimal.Decimal `json:"rate"`
	Timestamp int64           `json:"timestamp"`
}

type RateProvider interface {
	GetRate(ctx context.Context, currency string, timestamp int64) (*Rate, error)
	GetRates(ctx context.Context, currencies []string, timestamp int64) ([]Rate, error)
	GetRatesMap(ctx context.Context, currencies []string, timestamp int64) (map[string]Rate, error)
}
