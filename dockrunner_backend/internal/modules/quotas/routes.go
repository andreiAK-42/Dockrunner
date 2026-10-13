package quotas

import (
	"dockrunner_backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, jwtSecret string) {
	quotas := rg.Group("/quotas")
	quotas.Use(middleware.Auth(jwtSecret))
	{
		quotas.GET("", h.Get)
	}
}
