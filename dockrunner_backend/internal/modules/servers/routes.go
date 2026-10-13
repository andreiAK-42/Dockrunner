package servers

import (
	"github.com/gin-gonic/gin"
	"dockrunner_backend/internal/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, jwtSecret string) {
	servers := rg.Group("/servers")
	servers.Use(middleware.Auth(jwtSecret))
	{
		servers.GET("", h.List)
		servers.GET("/:id", h.GetByID)
		servers.POST("", h.Create)
		servers.DELETE("/:id", h.Delete)
	}
}
