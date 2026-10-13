package envconfigs

import (
	"dockrunner_backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, jwtSecret string) {
	configs := rg.Group("/env-configs")
	configs.Use(middleware.Auth(jwtSecret))
	{
		configs.GET("", h.List)
		configs.POST("", h.Create)
		configs.PATCH("/:id", h.Update)
		configs.DELETE("/:id", h.Delete)
	}
}
