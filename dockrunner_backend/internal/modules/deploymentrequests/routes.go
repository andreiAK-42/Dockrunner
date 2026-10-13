package deploymentrequests

import (
	"github.com/gin-gonic/gin"
	"dockrunner_backend/internal/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, jwtSecret string) {
	// User routes
	user := rg.Group("/deployment-requests")
	user.Use(middleware.Auth(jwtSecret))
	{
		user.POST("", h.Create)
		user.GET("/mine", h.ListMine)
		user.GET("/:id", h.Get)
	}

	// Admin routes
	admin := rg.Group("/admin/deployment-requests")
	admin.Use(middleware.Auth(jwtSecret), middleware.RequireRole("admin"))
	{
		admin.GET("/pending", h.ListPending)
		admin.GET("", h.ListAll)
		admin.POST("/:id/approve", h.Approve)
		admin.POST("/:id/reject", h.Reject)
		admin.PATCH("/:id/price", h.SetPrice)
	}
}