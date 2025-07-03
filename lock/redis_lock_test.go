package lock

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func setupTestRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Using DB 15 for testing to avoid conflicts with other tests
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		t.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Cleanup function
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client.FlushDB(ctx)
		client.Close()
	})

	return client
}

func TestRedisLock_Lock(t *testing.T) {
	// Setup Redis client (connection is verified in setupTestRedis)
	client := setupTestRedis(t)

	// Create lock with verified client
	lock := NewRedisLock(client)
	assert.NotNil(t, lock)

	ctx := context.Background()
	key := "test-lock"

	// Test successful lock acquisition with default options (8s expiry)
	start := time.Now()
	unlock, err := lock.Lock(ctx, key)
	t.Logf("First lock acquired in %v", time.Since(start))
	assert.NoError(t, err)
	assert.NotNil(t, unlock)

	// Test lock is actually held using TryLock
	start = time.Now()
	_, err = lock.TryLock(ctx, key)
	t.Logf("TryLock attempt took %v", time.Since(start))
	t.Logf("err type: %T", err)
	t.Logf("err: %v", err)
	assert.ErrorIs(t, err, ErrLockNotAcquired)

	// Test successful unlock
	start = time.Now()
	err = unlock(ctx)
	t.Logf("Unlock took %v", time.Since(start))
	assert.NoError(t, err)

	// Test can acquire lock again after unlock
	start = time.Now()
	unlock, err = lock.Lock(ctx, key)
	t.Logf("Third lock acquired in %v", time.Since(start))
	assert.NoError(t, err)
	assert.NotNil(t, unlock)
	err = unlock(ctx)
	assert.NoError(t, err)
}

func TestRedisLock_LockWithTimeout(t *testing.T) {
	client := setupTestRedis(t)
	lock := NewRedisLock(client)

	ctx := context.Background()
	key := "test-lock-timeout"

	// Test lock with custom timeout
	unlock, err := lock.Lock(ctx, key, WithExpiry(300*time.Millisecond))
	assert.NoError(t, err)
	assert.NotNil(t, unlock)

	// Wait for lock to expire
	time.Sleep(400 * time.Millisecond)

	// Should be able to acquire lock after timeout
	unlock2, err := lock.Lock(ctx, key, WithExpiry(300*time.Millisecond))
	assert.NoError(t, err)
	assert.NotNil(t, unlock2)
	err = unlock2(ctx)
	assert.NoError(t, err)
}

func TestRedisLock_TryLock(t *testing.T) {
	client := setupTestRedis(t)
	lock := NewRedisLock(client)

	ctx := context.Background()
	key := "test-try-lock"

	// Test successful try lock
	unlock, err := lock.TryLock(ctx, key)
	assert.NoError(t, err)
	assert.NotNil(t, unlock)

	// Test try lock when already locked
	_, err = lock.TryLock(ctx, key)
	assert.ErrorIs(t, err, ErrLockNotAcquired)

	// Test successful unlock
	err = unlock(ctx)
	assert.NoError(t, err)

	// Test can try lock again after unlock
	unlock, err = lock.TryLock(ctx, key)
	assert.NoError(t, err)
	assert.NotNil(t, unlock)
	err = unlock(ctx)
	assert.NoError(t, err)
}

func TestRedisLock_InvalidKey(t *testing.T) {
	client := setupTestRedis(t)
	lock := NewRedisLock(client)

	ctx := context.Background()

	// Test empty key
	_, err := lock.Lock(ctx, "")
	assert.ErrorIs(t, err, ErrInvalidLockKey)

	// Test empty key with TryLock
	_, err = lock.TryLock(ctx, "")
	assert.ErrorIs(t, err, ErrInvalidLockKey)
}

func TestRedisLock_ContextCancellation(t *testing.T) {
	client := setupTestRedis(t)
	lock := NewRedisLock(client)

	ctx, cancel := context.WithCancel(context.Background())
	key := "test-context-cancel"

	// Cancel the context
	cancel()

	// Test lock with cancelled context
	_, err := lock.Lock(ctx, key)
	assert.ErrorIs(t, err, context.Canceled)

	// Test try lock with cancelled context
	_, err = lock.TryLock(ctx, key)
	assert.ErrorIs(t, err, context.Canceled)
}
