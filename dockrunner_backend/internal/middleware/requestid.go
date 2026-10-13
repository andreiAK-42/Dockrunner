package middleware

import (
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestID ensures X-Request-ID header exists
func RequestID() gin.HandlerFunc {
	return requestid.New(
		requestid.WithGenerator(func() string {
			return uuid.NewString()
		}),
		requestid.WithCustomHeaderStrKey("X-Request-ID"),
	)
}

// GetRequestID helper
func GetRequestID(c *gin.Context) string {
	return requestid.Get(c)
}
