package rate

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestFurionRateProvider_TestGetRates(t *testing.T) {
	provider := NewFurionRateProvider(
		zap.NewNop(),
		"http://localhost:8080/rates",
		"",
		"",
	)
	rates, err := provider.GetRates(context.Background(), []string{"USD", "USDT", "JPY"}, time.Now().Unix())
	assert.NoError(t, err)
	assert.NotNil(t, rates)
}

func TestFurionRateProvider_TestGetRate(t *testing.T) {
	provider := NewFurionRateProvider(
		zap.NewNop(),
		"http://localhost:8080/rates",
		"",
		"",
	)
	rate, err := provider.GetRate(context.Background(), "USDT", time.Now().Unix())
	assert.NoError(t, err)
	assert.NotNil(t, rate)
}
