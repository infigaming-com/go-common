package rate

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFurionRateProvider_TestGetRates(t *testing.T) {
	provider, err := NewFurionRateProvider(
		"http://localhost:8082/rates",
		"",
		"",
	)
	rates, err := provider.GetRates(context.Background(), []string{"USD", "USDT", "JPY"}, time.Now().Unix())
	assert.NoError(t, err)
	assert.NotNil(t, rates)
}

func TestFurionRateProvider_TestGetRate(t *testing.T) {
	provider, err := NewFurionRateProvider(
		"http://localhost:8082/rates",
		"",
		"",
	)
	rate, err := provider.GetRate(context.Background(), "USDT", time.Now().Unix())
	assert.NoError(t, err)
	assert.NotNil(t, rate)
}

func TestFurionRateProvider_TestGetRatesMap(t *testing.T) {
	provider, err := NewFurionRateProvider(
		"http://localhost:8082",
		"",
		"",
	)
	ratesMap, err := provider.GetRatesMap(context.Background(), []string{"USD", "USDT", "JPY", "VND", "VND(K)"}, time.Now().Unix())
	assert.NoError(t, err)
	assert.NotNil(t, ratesMap)
}
