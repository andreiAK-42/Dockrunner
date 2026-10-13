package volumes

import (
	"dockrunner_backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, jwtSecret string) {
	volumes := rg.Group("/volumes")
	volumes.Use(middleware.Auth(jwtSecret))
	{
		volumes.GET("", h.List)
		volumes.POST("", h.Create)
		volumes.DELETE("/:name", h.Delete)
	}
}
