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
//   - Server keepalive: pings every 30s with 5s timeout to detect dead clients.
//
// This is safe to deploy before clients upgrade — old clients that don't send
// keepalive pings are unaffected by the enforcement policy.
//
// For optional connection age management (e.g. graceful drain during rolling
// deploys), use WithMaxConnectionAge.
func ResilientServerOptions(opts ...ServerOption) []grpc.ServerOption {
	cfg := &serverConfig{}
	for _, o := range opts {
		o(cfg)
	}

	params := keepalive.ServerParameters{
		MaxConnectionIdle: 15 * time.Minute,
		Time:              30 * time.Second,
		Timeout:           5 * time.Second,
	}
	if cfg.maxConnectionAge > 0 {
		params.MaxConnectionAge = cfg.maxConnectionAge
		params.MaxConnectionAgeGrace = 10 * time.Second
	}

	return []grpc.ServerOption{
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.KeepaliveParams(params),
	}
}

// ServerOption configures optional server parameters.
type ServerOption func(*serverConfig)

type serverConfig struct {
	maxConnectionAge time.Duration
}

// WithMaxConnectionAge sets the maximum connection age before the server
// sends GOAWAY. Use this only after all clients have been upgraded with
// ResilientDialOptions (which includes retry), so they can handle the
// transient reconnection gracefully.
func WithMaxConnectionAge(d time.Duration) ServerOption {
	return func(c *serverConfig) {
		c.maxConnectionAge = d
	}
}
