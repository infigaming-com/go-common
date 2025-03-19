package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/infigaming-com/go-common/util"
)

const CorrelationIdKey string = "X-CORRELATION-ID"

func CorrelationIdMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		correlationId := uuid.New().String()
		c.Header(CorrelationIdKey, correlationId)
		ctx := util.CorrelationIdToCtx(c.Request.Context(), correlationId)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
