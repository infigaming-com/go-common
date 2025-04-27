package rate

import (
	"context"

	"github.com/shopspring/decimal"
)

type Rate struct {
	Timestamp int64           `json:"timestamp"`
	Currency  string          `json:"currency"`
	Rate      decimal.Decimal `json:"rate"`
}

type RateProvider interface {
	GetRate(ctx context.Context, currency string, timestamp int64) (*Rate, error)
	GetRates(ctx context.Context, currencies []string, timestamp int64) ([]Rate, error)
}
