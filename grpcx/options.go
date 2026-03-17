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

// ResilientDialOptions returns gRPC client dial options that improve connection
// resilience during transient failures such as pod restarts or node rotations.
//
// It configures:
//   - Keepalive: pings every 10s, 3s timeout, permits pings without active streams.
//   - Retry: up to 3 attempts for UNAVAILABLE with exponential backoff (100ms-1s).
//
// Pair with ResilientServerOptions on the server side to allow 10s ping interval.
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

// ResilientServerOptions returns gRPC server options that pair with
// ResilientDialOptions on the client side.
//
// It configures:
//   - EnforcementPolicy: allows client pings every 5s and pings without active streams.
//   - ServerParameters: max connection age 5m with 10s grace for graceful drain during
//     rolling deployments; idle timeout 15m.
func ResilientServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionAge:      5 * time.Minute,
			MaxConnectionAgeGrace: 10 * time.Second,
			MaxConnectionIdle:     15 * time.Minute,
			Time:                  30 * time.Second,
			Timeout:               5 * time.Second,
		}),
	}
}
