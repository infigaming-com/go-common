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

func NewRedisUID(client *redis.Client, name string) (UID, error) {
	return &redisUID{
		client: client,
		name:   name,
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
