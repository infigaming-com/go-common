package grpcx

import (
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// retryServiceConfig is the default gRPC service config that enables
// automatic retry for UNAVAILABLE errors with exponential backoff.
const retryServiceConfig = `{
	"methodConfig": [{
		"name": [{"service": ""}],
		"retryPolicy": {
			"maxAttempts": 3,
			"initialBackoff": "0.1s",
			"maxBackoff": "1s",
			"backoffMultiplier": 2,
			"retryableStatusCodes": ["UNAVAILABLE"]
		}
	}]
}`

// ResilientDialOptions returns gRPC dial options that improve connection
// resilience during transient failures such as pod restarts or node rotations.
//
// It configures:
//   - Keepalive: pings every 10s, 3s timeout, permits pings without active streams.
//   - Retry: up to 3 attempts for UNAVAILABLE with exponential backoff (100ms-1s).
func ResilientDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultServiceConfig(retryServiceConfig),
	}
}
