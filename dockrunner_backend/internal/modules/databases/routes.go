package databases

import (
	"github.com/gin-gonic/gin"

	"dockrunner_backend/internal/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, jwtSecret string) {
	databases := rg.Group("/databases")
	databases.Use(middleware.Auth(jwtSecret))
	{
		databases.GET("", h.Engines)
		databases.GET("/users", h.Mine)
		databases.POST("/users", h.Create)
		databases.DELETE("/users/:id", h.Delete)
	}
}
