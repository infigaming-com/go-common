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
		"hwostZ8GyYYBbHfa10MTmaGpUfQLfwLMb8hYX0jn9hH086yEgD7LdpkfR8MFbaZjQ2WFM05XXJrJPLDvu8870oT39jkw5yEWn9jOcT59LepQ9S9C9X8fXU4Fd9V3BQtA",
		"ytUqDfrwJAFGwNdlz1b9ksVDF9xAk6Z7b6XZ80sEF7RZYPsQ9USAc5243L4r94EnqsJdBVU2bJTLzYuRYiYn8d5FtRNLuFA5WZugEgfWUb6ftPhjhgVsi3RLWn5gMcCv",
	)
	rates, err := provider.GetRates(context.Background(), []string{"USD", "USDT", "JPY"}, time.Now().Unix())
	assert.NoError(t, err)
	assert.NotNil(t, rates)
}

func TestFurionRateProvider_TestGetRate(t *testing.T) {
	provider := NewFurionRateProvider(
		zap.NewNop(),
		"http://localhost:8080/rates",
		"hwostZ8GyYYBbHfa10MTmaGpUfQLfwLMb8hYX0jn9hH086yEgD7LdpkfR8MFbaZjQ2WFM05XXJrJPLDvu8870oT39jkw5yEWn9jOcT59LepQ9S9C9X8fXU4Fd9V3BQtA",
		"ytUqDfrwJAFGwNdlz1b9ksVDF9xAk6Z7b6XZ80sEF7RZYPsQ9USAc5243L4r94EnqsJdBVU2bJTLzYuRYiYn8d5FtRNLuFA5WZugEgfWUb6ftPhjhgVsi3RLWn5gMcCv",
	)
	rate, err := provider.GetRate(context.Background(), "USDT", time.Now().Unix())
	assert.NoError(t, err)
	assert.NotNil(t, rate)
}
