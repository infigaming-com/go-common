package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/coocood/freecache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestFreeCache(t *testing.T) Cache {
	// Create a 10MB cache for testing
	cache := freecache.NewCache(10 * 1024 * 1024)
	return NewFreeCache(cache)
}

func TestNewFreeCache(t *testing.T) {
	cache := freecache.NewCache(1024 * 1024) // 1MB
	freeCache := NewFreeCache(cache)
	assert.NotNil(t, freeCache)
}

func TestFreeCache_Set(t *testing.T) {
	cache := createTestFreeCache(t)
	ctx := context.Background()

	t.Run("successful set", func(t *testing.T) {
		err := cache.Set(ctx, "test-key", "test-value", time.Minute)
		assert.NoError(t, err)

		// Verify the value was set
		value, err := cache.Get(ctx, "test-key")
		assert.NoError(t, err)
		assert.Equal(t, "test-value", value)
	})

	t.Run("set with zero expiry", func(t *testing.T) {
		err := cache.Set(ctx, "test-key-zero", "test-value-zero", 0)
		assert.NoError(t, err)

		value, err := cache.Get(ctx, "test-key-zero")
		assert.NoError(t, err)
		assert.Equal(t, "test-value-zero", value)
	})

	t.Run("set with empty key", func(t *testing.T) {
		err := cache.Set(ctx, "", "test-value", time.Minute)
		assert.NoError(t, err)

		value, err := cache.Get(ctx, "")
		assert.NoError(t, err)
		assert.Equal(t, "test-value", value)
	})

	t.Run("set with empty value", func(t *testing.T) {
		err := cache.Set(ctx, "empty-value-key", "", time.Minute)
		assert.NoError(t, err)

		value, err := cache.Get(ctx, "empty-value-key")
		assert.NoError(t, err)
		assert.Equal(t, "", value)
	})

	t.Run("set with negative expiry", func(t *testing.T) {
		err := cache.Set(ctx, "negative-expiry-key", "test-value", -time.Minute)
		assert.NoError(t, err)

		value, err := cache.Get(ctx, "negative-expiry-key")
		assert.NoError(t, err)
		assert.Equal(t, "test-value", value)
	})
}

func TestFreeCache_SetNX(t *testing.T) {
	cache := createTestFreeCache(t)
	ctx := context.Background()

	t.Run("not supported", func(t *testing.T) {
		ok, err := cache.SetNX(ctx, "test-key", "test-value", time.Minute)
		assert.Error(t, err)
		assert.False(t, ok)
		assert.Equal(t, ErrNotSupport, err)
	})
}

func TestFreeCache_Get(t *testing.T) {
	cache := createTestFreeCache(t)
	ctx := context.Background()

	t.Run("get existing key", func(t *testing.T) {
		// Set a value first
		err := cache.Set(ctx, "existing-key", "existing-value", time.Minute)
		require.NoError(t, err)

		value, err := cache.Get(ctx, "existing-key")
		assert.NoError(t, err)
		assert.Equal(t, "existing-value", value)
	})

	t.Run("get non-existing key", func(t *testing.T) {
		value, err := cache.Get(ctx, "non-existing-key")
		assert.Error(t, err)
		assert.Empty(t, value)
		assert.Equal(t, ErrKeyNotFound, err)
	})

	t.Run("get after expiry", func(t *testing.T) {
		// Set a value with very short expiry
		err := cache.Set(ctx, "expiry-key", "expiry-value", 1*time.Second)
		require.NoError(t, err)

		// Value should exist immediately
		value, err := cache.Get(ctx, "expiry-key")
		assert.NoError(t, err)
		assert.Equal(t, "expiry-value", value)

		// Wait for expiry (FreeCache TTL might not be precise for very short durations)
		time.Sleep(2 * time.Second)

		value, err = cache.Get(ctx, "expiry-key")
		assert.Error(t, err)
		assert.Empty(t, value)
		assert.Equal(t, ErrKeyNotFound, err)
	})

	t.Run("get with empty key", func(t *testing.T) {
		value, err := cache.Get(ctx, "")
		assert.Error(t, err)
		assert.Empty(t, value)
		assert.Equal(t, ErrKeyNotFound, err)
	})
}

func TestFreeCache_Sets(t *testing.T) {
	cache := createTestFreeCache(t)
	ctx := context.Background()

	t.Run("successful sets", func(t *testing.T) {
		kvs := map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		}

		err := cache.Sets(ctx, kvs, time.Minute)
		assert.NoError(t, err)

		// Verify all values were set
		for key, expectedValue := range kvs {
			value, err := cache.Get(ctx, key)
			assert.NoError(t, err)
			assert.Equal(t, expectedValue, value)
		}
	})

	t.Run("sets with empty map", func(t *testing.T) {
		err := cache.Sets(ctx, map[string]string{}, time.Minute)
		assert.NoError(t, err)
	})

	t.Run("sets with empty values", func(t *testing.T) {
		kvs := map[string]string{
			"empty1": "",
			"empty2": "",
		}

		err := cache.Sets(ctx, kvs, time.Minute)
		assert.NoError(t, err)

		for key := range kvs {
			value, err := cache.Get(ctx, key)
			assert.NoError(t, err)
			assert.Equal(t, "", value)
		}
	})

	t.Run("sets with zero expiry", func(t *testing.T) {
		kvs := map[string]string{
			"zero-expiry1": "value1",
			"zero-expiry2": "value2",
		}

		err := cache.Sets(ctx, kvs, 0)
		assert.NoError(t, err)

		for key, expectedValue := range kvs {
			value, err := cache.Get(ctx, key)
			assert.NoError(t, err)
			assert.Equal(t, expectedValue, value)
		}
	})
}

func TestFreeCache_SetsNX(t *testing.T) {
	cache := createTestFreeCache(t)
	ctx := context.Background()

	t.Run("not supported", func(t *testing.T) {
		kvs := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}

		result, err := cache.SetsNX(ctx, kvs, time.Minute)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, ErrNotSupport, err)
	})
}

func TestFreeCache_Gets(t *testing.T) {
	cache := createTestFreeCache(t)
	ctx := context.Background()

	t.Run("gets existing keys", func(t *testing.T) {
		// Set multiple values
		kvs := map[string]string{
			"get-key1": "get-value1",
			"get-key2": "get-value2",
			"get-key3": "get-value3",
		}

		err := cache.Sets(ctx, kvs, time.Minute)
		require.NoError(t, err)

		keys := []string{"get-key1", "get-key2", "get-key3"}
		results, err := cache.Gets(ctx, keys)
		assert.NoError(t, err)
		assert.Len(t, results, 3)

		for key, expectedValue := range kvs {
			assert.Equal(t, expectedValue, results[key])
		}
	})

	t.Run("gets with non-existing key", func(t *testing.T) {
		// Set one value
		err := cache.Set(ctx, "existing-get-key", "existing-get-value", time.Minute)
		require.NoError(t, err)

		keys := []string{"existing-get-key", "non-existing-get-key"}
		results, err := cache.Gets(ctx, keys)
		assert.Error(t, err)
		assert.Nil(t, results)
		assert.Equal(t, ErrKeyNotFound, err)
	})

	t.Run("gets with empty keys slice", func(t *testing.T) {
		results, err := cache.Gets(ctx, []string{})
		assert.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("gets with single key", func(t *testing.T) {
		err := cache.Set(ctx, "single-key", "single-value", time.Minute)
		require.NoError(t, err)

		results, err := cache.Gets(ctx, []string{"single-key"})
		assert.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "single-value", results["single-key"])
	})
}

func TestFreeCache_Delete(t *testing.T) {
	cache := createTestFreeCache(t)
	ctx := context.Background()

	t.Run("delete existing key", func(t *testing.T) {
		// Set a value first
		err := cache.Set(ctx, "delete-key", "delete-value", time.Minute)
		require.NoError(t, err)

		// Verify it exists
		value, err := cache.Get(ctx, "delete-key")
		assert.NoError(t, err)
		assert.Equal(t, "delete-value", value)

		// Delete it
		err = cache.Delete(ctx, "delete-key")
		assert.NoError(t, err)

		// Verify it's gone
		value, err = cache.Get(ctx, "delete-key")
		assert.Error(t, err)
		assert.Empty(t, value)
		assert.Equal(t, ErrKeyNotFound, err)
	})

	t.Run("delete non-existing key", func(t *testing.T) {
		err := cache.Delete(ctx, "non-existing-delete-key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("delete with empty key", func(t *testing.T) {
		err := cache.Delete(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestFreeCache_Clear(t *testing.T) {
	cache := createTestFreeCache(t)
	ctx := context.Background()

	t.Run("clear all keys", func(t *testing.T) {
		// Set multiple values
		kvs := map[string]string{
			"clear-key1": "clear-value1",
			"clear-key2": "clear-value2",
			"clear-key3": "clear-value3",
		}

		err := cache.Sets(ctx, kvs, time.Minute)
		require.NoError(t, err)

		// Verify they exist
		for key := range kvs {
			value, err := cache.Get(ctx, key)
			assert.NoError(t, err)
			assert.NotEmpty(t, value)
		}

		// Clear all
		err = cache.Clear(ctx)
		assert.NoError(t, err)

		// Verify they're all gone
		for key := range kvs {
			value, err := cache.Get(ctx, key)
			assert.Error(t, err)
			assert.Empty(t, value)
			assert.Equal(t, ErrKeyNotFound, err)
		}
	})

	t.Run("clear empty cache", func(t *testing.T) {
		err := cache.Clear(ctx)
		assert.NoError(t, err)
	})
}

func TestFreeCache_Expiry(t *testing.T) {
	cache := createTestFreeCache(t)
	ctx := context.Background()

	t.Run("value expires", func(t *testing.T) {
		err := cache.Set(ctx, "expiry-test", "expiry-value", 1*time.Second)
		require.NoError(t, err)

		// Value should exist immediately
		value, err := cache.Get(ctx, "expiry-test")
		assert.NoError(t, err)
		assert.Equal(t, "expiry-value", value)

		// Wait for expiry (FreeCache TTL might not be precise for very short durations)
		time.Sleep(2 * time.Second)

		// Value should be gone
		value, err = cache.Get(ctx, "expiry-test")
		assert.Error(t, err)
		assert.Empty(t, value)
		assert.Equal(t, ErrKeyNotFound, err)
	})

	t.Run("value with zero expiry never expires", func(t *testing.T) {
		err := cache.Set(ctx, "no-expiry-test", "no-expiry-value", 0)
		require.NoError(t, err)

		// Value should exist immediately
		value, err := cache.Get(ctx, "no-expiry-test")
		assert.NoError(t, err)
		assert.Equal(t, "no-expiry-value", value)

		// Wait a bit
		time.Sleep(10 * time.Millisecond)

		// Value should still exist
		value, err = cache.Get(ctx, "no-expiry-test")
		assert.NoError(t, err)
		assert.Equal(t, "no-expiry-value", value)
	})
}

func TestFreeCache_Concurrent(t *testing.T) {
	cache := createTestFreeCache(t)
	ctx := context.Background()

	t.Run("concurrent sets and gets", func(t *testing.T) {
		const numGoroutines = 10
		const numOperations = 100

		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer func() { done <- true }()

				for j := 0; j < numOperations; j++ {
					key := fmt.Sprintf("concurrent-key-%d-%d", id, j)
					value := fmt.Sprintf("concurrent-value-%d-%d", id, j)

					// Set value
					err := cache.Set(ctx, key, value, time.Minute)
					if err != nil {
						t.Errorf("Failed to set %s: %v", key, err)
						return
					}

					// Get value
					retrieved, err := cache.Get(ctx, key)
					if err != nil {
						t.Errorf("Failed to get %s: %v", key, err)
						return
					}

					if retrieved != value {
						t.Errorf("Value mismatch for %s: expected %s, got %s", key, value, retrieved)
					}
				}
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}
	})
}

func TestFreeCache_Context(t *testing.T) {
	cache := createTestFreeCache(t)

	t.Run("with cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := cache.Set(ctx, "cancelled-key", "cancelled-value", time.Minute)
		// FreeCache doesn't check context, so this should succeed
		assert.NoError(t, err)
	})

	t.Run("with timeout context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()

		err := cache.Set(ctx, "timeout-key", "timeout-value", time.Minute)
		// FreeCache doesn't check context, so this should succeed
		assert.NoError(t, err)
	})
}

func TestFreeCache_EdgeCases(t *testing.T) {
	cache := createTestFreeCache(t)
	ctx := context.Background()

	t.Run("very large value", func(t *testing.T) {
		largeValue := string(make([]byte, 1000)) // 1KB string
		err := cache.Set(ctx, "large-key", largeValue, time.Minute)
		assert.NoError(t, err)

		value, err := cache.Get(ctx, "large-key")
		assert.NoError(t, err)
		assert.Equal(t, largeValue, value)
	})

	t.Run("special characters in key and value", func(t *testing.T) {
		specialKey := "key-with-special-chars:!@#$%^&*()"
		specialValue := "value-with-special-chars:!@#$%^&*()"

		err := cache.Set(ctx, specialKey, specialValue, time.Minute)
		assert.NoError(t, err)

		value, err := cache.Get(ctx, specialKey)
		assert.NoError(t, err)
		assert.Equal(t, specialValue, value)
	})

	t.Run("unicode characters", func(t *testing.T) {
		unicodeKey := "key-with-unicode-🚀-🎉"
		unicodeValue := "value-with-unicode-🚀-🎉"

		err := cache.Set(ctx, unicodeKey, unicodeValue, time.Minute)
		assert.NoError(t, err)

		value, err := cache.Get(ctx, unicodeKey)
		assert.NoError(t, err)
		assert.Equal(t, unicodeValue, value)
	})

	t.Run("very long key", func(t *testing.T) {
		longKey := string(make([]byte, 1000)) // 1KB key
		err := cache.Set(ctx, longKey, "test-value", time.Minute)
		assert.NoError(t, err)

		value, err := cache.Get(ctx, longKey)
		assert.NoError(t, err)
		assert.Equal(t, "test-value", value)
	})
}
