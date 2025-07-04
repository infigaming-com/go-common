package dns

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	cfApiToken = "KWHCefM-W17-di0WN1KUYvYcrBJnsXrtBddYD5bA"
)

func TestCfCreateRecord(t *testing.T) {
	cf, err := NewCfDNSProvider(cfApiToken)
	if err != nil {
		t.Fatalf("fail to create dns provider: %v", err)
	}
	err = cf.CreateRecord(
		context.Background(),
		DNSRecord{
			Type:    "A",
			Name:    "aaa.a9game.site",
			Content: "130.211.73.222",
			TTL:     1,
		},
	)
	assert.NoError(t, err)
}
