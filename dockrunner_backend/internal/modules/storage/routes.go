package storage

import (
	"github.com/gin-gonic/gin"
	"dockrunner_backend/internal/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, jwtSecret string) {
	storage := rg.Group("/storage")
	storage.Use(middleware.Auth(jwtSecret))
	{
		storage.GET("/usage", h.Usage)
	}
}
