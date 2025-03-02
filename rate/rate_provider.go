package rate

import (
	"context"
)

type Rate struct {
	Timestamp int64   `json:"timestamp"`
	Currency  string  `json:"currency"`
	Rate      float64 `json:"rate"`
}

type RateProvider interface {
	GetRate(ctx context.Context, currency string, timestamp int64) (*Rate, error)
	GetRates(ctx context.Context, currencies []string, timestamp int64) ([]Rate, error)
}
