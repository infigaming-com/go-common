package uid

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type redisUID struct {
	client *redis.Client
	name   string
}

type Options struct {
	addr           string
	db             int64
	connectTimeout time.Duration
	name           string
}

type Option interface {
	apply(*Options) error
}

type optionFunc func(*Options) error

func (f optionFunc) apply(o *Options) error {
	return f(o)
}

func defaultOptions() *Options {
	return &Options{
		addr:           "localhost:6379",
		db:             0,
		connectTimeout: 5 * time.Second,
	}
}

func WithAddr(addr string) Option {
	return optionFunc(func(o *Options) error {
		o.addr = addr
		return nil
	})
}

func WithDB(db int64) Option {
	return optionFunc(func(o *Options) error {
		o.db = db
		return nil
	})
}

func WithConnectTimeout(timeout time.Duration) Option {
	return optionFunc(func(o *Options) error {
		o.connectTimeout = timeout
		return nil
	})
}

func WithName(name string) Option {
	return optionFunc(func(o *Options) error {
		o.name = name
		return nil
	})
}

func NewRedisUID(opts ...Option) (UID, func(), error) {
	cfg := defaultOptions()
	for _, opt := range opts {
		if err := opt.apply(cfg); err != nil {
			return nil, nil, fmt.Errorf("failed to apply option to redis uid: %w", err)
		}
	}

	redisOptions := &redis.Options{
		Addr: cfg.addr,
		DB:   int(cfg.db),
	}
	client := redis.NewClient(redisOptions)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.connectTimeout)
	defer cancel()
	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, nil, fmt.Errorf("failed to connect to redis for uid: %w", err)
	}

	return &redisUID{
			client: client,
			name:   cfg.name,
		}, func() {
			client.Close()
		}, nil
}

func (r *redisUID) New() (string, error) {
	counter, err := r.client.Incr(context.Background(), getCounterKey(r.name)).Result()
	if err != nil {
		return "", fmt.Errorf("failed to get counter for uid: %w", err)
	}

	uuid, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("failed to generate uuidv7 for uid: %w", err)
	}

	var sb strings.Builder
	// 16 (timestamp) + 1 (-) + 16 (counter) + 1 (-) + 32 (UUID without hyphens)
	sb.Grow(16 + 1 + 16 + 1 + 32)
	sb.WriteString(strconv.FormatInt(time.Now().UnixNano(), 16))
	sb.WriteString("-")
	sb.WriteString(strconv.FormatInt(counter, 16))
	sb.WriteString("-")
	sb.WriteString(strings.ReplaceAll(uuid.String(), "-", ""))

	return sb.String(), nil
}

func getCounterKey(name string) string {
	var sb strings.Builder
	if len(name) > 0 {
		sb.Grow(len("counter:uid:") + len(name))
		sb.WriteString("counter:uid:")
		sb.WriteString(name)
	} else {
		sb.Grow(len("counter:uid"))
		sb.WriteString("counter:uid")
	}
	return sb.String()
}
