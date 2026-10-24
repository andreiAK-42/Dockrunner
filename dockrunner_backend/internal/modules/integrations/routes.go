package integrations

import (
	"github.com/gin-gonic/gin"

	"dockrunner_backend/internal/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, jwtSecret string) {
	mount(rg.Group("/integrations"), h, jwtSecret)
	mount(rg.Group("/databases"), h, jwtSecret)
}

func mount(g *gin.RouterGroup, h *Handler, jwtSecret string) {
	g.Use(middleware.Auth(jwtSecret))
	g.GET("", h.Catalog)
	g.GET("/users", h.Mine)
	g.POST("/users", h.Create)
	g.POST("/users/:id/reset-password", h.ResetPassword)
	g.DELETE("/users/:id", h.Delete)
}
