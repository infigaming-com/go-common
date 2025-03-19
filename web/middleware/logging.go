package middleware

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	common_util "github.com/infigaming-com/go-common/util"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type MsgArchiver func(ctx context.Context, correlationId string, url string, msg map[string]any)

type loggingMiddlewareOptions struct {
	lg           *zap.Logger
	debugEnabled bool
	msgArchiver  MsgArchiver
	excludePaths []string
}

type LoggingMiddlewareOption func(*loggingMiddlewareOptions)

func WithLogger(lg *zap.Logger) LoggingMiddlewareOption {
	return func(o *loggingMiddlewareOptions) {
		o.lg = lg
	}
}

func WithDebugEnabled(debugEnabled bool) LoggingMiddlewareOption {
	return func(o *loggingMiddlewareOptions) {
		o.debugEnabled = debugEnabled
	}
}

func WithMsgArchiver(msgArchiver MsgArchiver) LoggingMiddlewareOption {
	return func(o *loggingMiddlewareOptions) {
		o.msgArchiver = msgArchiver
	}
}

func WithExcludePaths(excludePaths []string) LoggingMiddlewareOption {
	return func(o *loggingMiddlewareOptions) {
		o.excludePaths = excludePaths
	}
}

func defaultLoggingMiddlewareOptions() *loggingMiddlewareOptions {
	return &loggingMiddlewareOptions{
		lg:           zap.L(),
		debugEnabled: true,
		msgArchiver:  nil,
	}
}

func LoggingMiddleware(opts ...LoggingMiddlewareOption) gin.HandlerFunc {
	cfg := defaultLoggingMiddlewareOptions()

	for _, opt := range opts {
		opt(cfg)
	}

	return func(c *gin.Context) {
		if lo.Contains(cfg.excludePaths, c.Request.URL.Path) {
			c.Next()
			return
		}

		ctx := c.Request.Context()

		correlationId, err := common_util.CorrelationIdFromCtx(ctx)
		if err != nil {
			cfg.lg.Warn("failed to get correlation id", zap.Error(err))
			correlationId = uuid.New().String()
		}

		startTime := time.Now()
		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		rw := &responseWriter{ResponseWriter: c.Writer, body: bytes.NewBuffer([]byte{})}
		c.Writer = rw

		c.Next()

		responseBody := rw.body.Bytes()
		if len(responseBody) > 1024 {
			responseBody = responseBody[:1024]
		}

		if cfg.debugEnabled {
			cfg.lg.Debug("[Logging]", zap.String("CorrelationID: ", correlationId),
				zap.String("method", c.Request.Method),
				zap.String("url", c.Request.URL.String()),
				zap.Any("queryParams", c.Request.URL.Query()),
				zap.Any("requestHeaders", c.Request.Header),
				zap.ByteString("requestBody", requestBody),
				zap.Int("status", c.Writer.Status()),
				zap.ByteString("responseBody", responseBody),
				zap.Duration("duration", time.Since(startTime)),
			)
		}

		if cfg.msgArchiver != nil {
			cfg.msgArchiver(
				ctx,
				correlationId,
				c.Request.URL.String(),
				map[string]any{
					"method":       c.Request.Method,
					"queryParams":  c.Request.URL.Query(),
					"requestBody":  string(requestBody),
					"status":       c.Writer.Status(),
					"responseBody": string(responseBody),
				},
			)
		}
	}
}
