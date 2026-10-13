package users

import (
	"github.com/gin-gonic/gin"
	"dockrunner_backend/internal/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, jwtSecret string) {
	users := rg.Group("/users")
	users.Use(middleware.Auth(jwtSecret))
	{
		users.GET("", middleware.RequireRole("admin"), h.List)
		users.POST("", middleware.RequireRole("admin"), h.AdminCreate)
		users.GET("/:id", h.GetByID)
		users.PATCH("/:id/role", middleware.RequireRole("admin"), h.UpdateRole)
		users.PATCH("/:id/block", middleware.RequireRole("admin"), h.SetBlocked)
		users.PATCH("/:id/password", middleware.RequireRole("admin"), h.SetPassword)
		users.PATCH("/:id/plan", middleware.RequireRole("admin"), h.SetPlan)
		users.DELETE("/:id", middleware.RequireRole("admin"), h.Delete)
	}
}
