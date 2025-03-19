package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type CurrencyKey struct {
	LineName string `json:"line_name"`
	Currency string `json:"currency"`
}

type CurrencyValue struct {
	DecimalPlaces int64 `json:"decimal_places"`
}

type Currency struct {
	LineName      string `json:"line_name"`
	Currency      string `json:"currency"`
	DecimalPlaces int64  `json:"decimal_places"`
}

func (c *Currency) Key() string {
	return fmt.Sprintf("currency:%s:%s", c.LineName, c.Currency)
}

func (c *Currency) Value() CurrencyValue {
	return CurrencyValue{
		DecimalPlaces: c.DecimalPlaces,
	}
}

func TestRedisCacheCurrencySetAndGet(t *testing.T) {
	lg, err := zap.NewDevelopment()
	if err != nil {
		t.Fatal(err)
	}

	cache, close := NewRedisCache(lg, &RedisCacheConfig{
		Addr:           "localhost:6379",
		DB:             15,
		ConnectTimeout: 5,
	})
	defer close()

	currency := &Currency{
		LineName:      "test_line_name",
		Currency:      "USD",
		DecimalPlaces: 2,
	}

	err = SetTyped(context.Background(), cache, currency.Key(), currency.Value(), 60*time.Second)
	assert.NoError(t, err)

	time.Sleep(1 * time.Second)
	value, err := GetTyped[CurrencyValue](context.Background(), cache, currency.Key())
	assert.NoError(t, err)
	assert.Equal(t, currency.DecimalPlaces, value.DecimalPlaces)

	err = Delete(context.Background(), cache, currency.Key())
	assert.NoError(t, err)

	value, err = GetTyped[CurrencyValue](context.Background(), cache, currency.Key())
	assert.ErrorIs(t, err, ErrKeyNotFound)
}
