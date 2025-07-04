package dns

import (
	"context"
	"errors"
)

type DNSRecord struct {
	Type    string
	Name    string
	Content string
	TTL     int
}

type DNSProvider interface {
	CreateRecord(ctx context.Context, record DNSRecord) error
}

var (
	ErrRecordAlreadyExists = errors.New("record already exists")
)
