package util

import (
	"time"

	"github.com/google/uuid"
)

func NewUUID() string {
	maxRetry := 10
	for i := 0; i < maxRetry; i++ {
		id, err := uuid.NewV7()
		if err == nil {
			return id.String()
		}

		if i < maxRetry-1 { // Don't sleep on last attempt
			// Sleep for 200 nanoseconds
			// Just over UUID v7's 100ns precision
			time.Sleep(200 * time.Nanosecond)
		}
	}

	// Fallback to UUID v4 after all retries fail
	return uuid.New().String()
}
