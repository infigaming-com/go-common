package sessiontracker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// TrackRequest carries all session info from the middleware.
type TrackRequest struct {
	UserID             int64
	RealOperatorID     int64  // computed "active" operator
	OperatorID         int64  // individual operator level (0 if not operator type)
	CompanyOperatorID  int64  // company level (0 if below company)
	RetailerOperatorID int64  // retailer level (0 if below retailer)
	SystemOperatorID   int64  // system level
	OperatorType       string // "operator" | "company" | "retailer" | "system"
	IP                 string
	UserAgent          string
	Country            string
	LoginMethod        string
}

// ChangeEvent contains information about detected session activity changes.
type ChangeEvent struct {
	UserID             int64
	OperatorID         int64    // = RealOperatorID for backward compat
	RealOperatorID     int64
	CompanyOperatorID  int64
	RetailerOperatorID int64
	SystemOperatorID   int64
	OperatorType       string
	LoginMethod        string
	Triggers           []string // e.g. ["daily_visit", "ip_change", "device_change"]
	IP                 string
	PrevIP             string
	UserAgent          string
	UAHash             string
	PrevUAHash         string
	Country            string
	PrevCountry        string
	Timestamp          int64
}

// OnChangeFunc is called asynchronously when a change is detected.
type OnChangeFunc func(event *ChangeEvent)

type l1Entry struct {
	ip      string
	uaHash  string
	country string
	date    string
	expiry  time.Time
}

// Tracker provides two-level caching (L1 in-process, L2 Redis) for session
// activity tracking. When a change is detected (new day, IP change, device
// change), it invokes the registered callback asynchronously.
type Tracker struct {
	redisClient *redis.Client
	onChange    OnChangeFunc

	l1    sync.Map // map[int64]*l1Entry
	l1TTL time.Duration

	redisKeyPrefix  string
	l2TTL           time.Duration
	cleanupInterval time.Duration

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// New creates a new Tracker. The onChange callback is invoked in a separate
// goroutine whenever a trackable change is detected.
func New(redisClient *redis.Client, onChange OnChangeFunc, opts ...Option) *Tracker {
	t := &Tracker{
		redisClient:     redisClient,
		onChange:        onChange,
		l1TTL:           5 * time.Minute,
		redisKeyPrefix:  "session_ctx",
		l2TTL:           30 * 24 * time.Hour,
		cleanupInterval: 10 * time.Minute,
		stopCh:          make(chan struct{}),
	}
	for _, o := range opts {
		o(t)
	}

	// Start L1 cleanup goroutine.
	t.wg.Add(1)
	go t.cleanupLoop(t.cleanupInterval)

	return t
}

// Track records a session activity for the given user. It is safe to call
// concurrently from multiple goroutines.
func (t *Tracker) Track(ctx context.Context, req *TrackRequest) {
	uaHash := hashUA(req.UserAgent)
	date := time.Now().UTC().Format("2006-01-02")

	// L1 lookup
	if v, ok := t.l1.Load(req.UserID); ok {
		entry := v.(*l1Entry)
		if time.Now().Before(entry.expiry) &&
			entry.date == date &&
			entry.ip == req.IP &&
			entry.uaHash == uaHash {
			return // no change
		}
	}

	// L2 lookup
	redisKey := fmt.Sprintf("%s:%d", t.redisKeyPrefix, req.UserID)
	cached, err := t.redisClient.HGetAll(ctx, redisKey).Result()

	var triggers []string
	var prevIP, prevUAHash, prevCountry string

	if err != nil || len(cached) == 0 {
		// No L2 entry — first time or expired
		triggers = append(triggers, "daily_visit")
	} else {
		prevIP = cached["ip"]
		prevUAHash = cached["ua_hash"]
		prevCountry = cached["country"]
		cachedDate := cached["date"]

		if cachedDate != date {
			triggers = append(triggers, "daily_visit")
		}
		if prevIP != "" && prevIP != req.IP {
			triggers = append(triggers, "ip_change")
		}
		if prevUAHash != "" && prevUAHash != uaHash {
			triggers = append(triggers, "device_change")
		}

		// If L2 exists but nothing changed, just refresh L1 and return.
		if len(triggers) == 0 {
			t.l1.Store(req.UserID, &l1Entry{
				ip:      req.IP,
				uaHash:  uaHash,
				country: req.Country,
				date:    date,
				expiry:  time.Now().Add(t.l1TTL),
			})
			return
		}
	}

	// Update L1
	t.l1.Store(req.UserID, &l1Entry{
		ip:      req.IP,
		uaHash:  uaHash,
		country: req.Country,
		date:    date,
		expiry:  time.Now().Add(t.l1TTL),
	})

	// Update L2
	t.redisClient.HSet(ctx, redisKey, map[string]interface{}{
		"ip":      req.IP,
		"ua_hash": uaHash,
		"country": req.Country,
		"date":    date,
	})
	t.redisClient.Expire(ctx, redisKey, t.l2TTL)

	// Fire callback asynchronously
	if t.onChange != nil && len(triggers) > 0 {
		event := &ChangeEvent{
			UserID:             req.UserID,
			OperatorID:         req.RealOperatorID,
			RealOperatorID:     req.RealOperatorID,
			CompanyOperatorID:  req.CompanyOperatorID,
			RetailerOperatorID: req.RetailerOperatorID,
			SystemOperatorID:   req.SystemOperatorID,
			OperatorType:       req.OperatorType,
			LoginMethod:        req.LoginMethod,
			Triggers:           triggers,
			IP:                 req.IP,
			PrevIP:             prevIP,
			UserAgent:          req.UserAgent,
			UAHash:             uaHash,
			PrevUAHash:         prevUAHash,
			Country:            req.Country,
			PrevCountry:        prevCountry,
			Timestamp:          time.Now().UnixMilli(),
		}
		go t.onChange(event)
	}
}

// Stop shuts down the background cleanup goroutine.
func (t *Tracker) Stop() {
	close(t.stopCh)
	t.wg.Wait()
}

func (t *Tracker) cleanupLoop(interval time.Duration) {
	defer t.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			t.l1.Range(func(key, value any) bool {
				entry := value.(*l1Entry)
				if now.After(entry.expiry) {
					t.l1.Delete(key)
				}
				return true
			})
		case <-t.stopCh:
			return
		}
	}
}

func hashUA(ua string) string {
	h := sha256.Sum256([]byte(ua))
	return fmt.Sprintf("%x", h[:8]) // 16 hex chars
}
