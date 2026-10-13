package overview

import (
	"github.com/gin-gonic/gin"
	"dockrunner_backend/internal/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, jwtSecret string) {
	overview := rg.Group("/overview")
	overview.Use(middleware.Auth(jwtSecret))
	{
		overview.GET("/resources", h.Resources)
	}
}
