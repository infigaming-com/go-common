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
	DeleteRecord(ctx context.Context, domain string) error
}

var (
	ErrRecordAlreadyExists = errors.New("record already exists")
	ErrRecordNotFound      = errors.New("record not found")
)
