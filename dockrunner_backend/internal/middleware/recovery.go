package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Recovery with zap logging, returns 500 on panic
func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered interface{}) {
		logger.Error("panic recovered", zap.Any("error", recovered), zap.String("path", c.Request.URL.Path))
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
	})
}
