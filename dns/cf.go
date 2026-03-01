package dns

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudflare/cloudflare-go"
)

type cfDNSProvider struct {
	api         *cloudflare.API
	zoneIDCache sync.Map // rootDomain -> zoneID cache
}

func NewCfDNSProvider(apiToken string) (DNSProvider, error) {
	api, err := cloudflare.NewWithAPIToken(apiToken)
	if err != nil {
		return nil, fmt.Errorf("fail to create cloudflare api: %w", err)
	}

	return &cfDNSProvider{api: api}, nil
}

func (p *cfDNSProvider) CreateRecord(ctx context.Context, record DNSRecord) error {
	rootDomain, err := extractRootDomain(record.Name)
	if err != nil {
		return fmt.Errorf("fail to extract root domain: %w", err)
	}

	zoneId, err := p.getZoneID(rootDomain)
	if err != nil {
		return fmt.Errorf("fail to get zone id: %w", err)
	}

	_, err = p.api.CreateDNSRecord(
		ctx,
		cloudflare.ZoneIdentifier(zoneId), cloudflare.CreateDNSRecordParams{
			Type:    record.Type,
			Name:    record.Name,
			Content: record.Content,
			TTL:     record.TTL,
			Proxied: cloudflare.BoolPtr(true),
		},
	)
	if err != nil {
		if strings.Contains(err.Error(), "record already exists") {
			return ErrRecordAlreadyExists
		}
		return fmt.Errorf("fail to create record: %w", err)
	}

	return nil
}

// getZoneID returns cached zone ID or fetches and caches it.
// Zone IDs are immutable (domain-to-zone mapping never changes), so permanent caching is safe.
func (p *cfDNSProvider) getZoneID(rootDomain string) (string, error) {
	if cached, ok := p.zoneIDCache.Load(rootDomain); ok {
		return cached.(string), nil
	}
	zoneId, err := p.api.ZoneIDByName(rootDomain)
	if err != nil {
		return "", err
	}
	p.zoneIDCache.Store(rootDomain, zoneId)
	return zoneId, nil
}

func extractRootDomain(subdomain string) (string, error) {
	if subdomain == "" {
		return "", fmt.Errorf("subdomain cannot be empty")
	}

	// Split the domain by dots
	parts := strings.Split(subdomain, ".")

	// We need at least 2 parts for a valid domain (domain.tld)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid domain format: %s", subdomain)
	}

	// For most cases, take the last 2 parts (domain.tld)
	rootDomain := strings.Join(parts[len(parts)-2:], ".")

	// Handle special cases for domains with country code TLDs (like .co.uk, .com.au)
	if len(parts) >= 3 {
		lastPart := parts[len(parts)-1]
		secondLastPart := parts[len(parts)-2]

		// Common country code TLD patterns
		countryCodeTLDs := map[string][]string{
			"uk": {"co", "org", "net", "ac", "gov"},
			"au": {"com", "net", "org", "edu", "gov"},
			"nz": {"co", "net", "org", "ac", "govt"},
			"za": {"co", "net", "org", "ac", "gov"},
			"br": {"com", "net", "org", "edu", "gov"},
			"in": {"co", "net", "org", "edu", "gov"},
		}

		if validSecondLevel, exists := countryCodeTLDs[lastPart]; exists {
			for _, valid := range validSecondLevel {
				if secondLastPart == valid {
					// Take last 3 parts for domains like example.co.uk
					if len(parts) >= 3 {
						rootDomain = strings.Join(parts[len(parts)-3:], ".")
					}
					break
				}
			}
		}
	}

	return rootDomain, nil
}
