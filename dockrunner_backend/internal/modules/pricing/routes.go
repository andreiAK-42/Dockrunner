package pricing

import (
	"github.com/gin-gonic/gin"

	"dockrunner_backend/internal/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, jwtSecret string) {
	pricing := rg.Group("/pricing")
	pricing.Use(middleware.Auth(jwtSecret))
	{
		pricing.GET("", h.List)
	}

	admin := rg.Group("/admin")
	admin.Use(middleware.Auth(jwtSecret))
	admin.Use(middleware.RequireRole("admin"))
	{
		admin.PUT("/pricing", h.SetRates)
	}
}
