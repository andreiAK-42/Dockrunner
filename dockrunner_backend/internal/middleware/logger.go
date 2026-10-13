package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ZapLogger logs requests in JSON with zap
func ZapLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		if strings.HasSuffix(path, "/build-log/stream") {
			// Never log the ?token= JWT from WebSocket URLs.
			query = "[redacted]"
		}

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		ip := c.ClientIP()
		method := c.Request.Method
		requestID := c.GetString("request_id")
		if requestID == "" {
			requestID = c.Writer.Header().Get("X-Request-ID")
		}

		fields := []zap.Field{
			zap.Int("status", status),
			zap.String("method", method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", ip),
			zap.Duration("latency", latency),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.String("request_id", requestID),
		}

		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errors", c.Errors.String()))
		}

		switch {
		case status >= 500:
			logger.Error("request", fields...)
		case status >= 400:
			logger.Warn("request", fields...)
		default:
			logger.Info("request", fields...)
		}
	}
}
