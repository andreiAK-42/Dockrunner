package admin

import (
	"dockrunner_backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, jwtSecret string) {
	admin := rg.Group("/admin")
	admin.Use(middleware.Auth(jwtSecret))
	admin.Use(middleware.RequireRole("admin"))
	{
		admin.GET("/plans", h.GetPlans)
		admin.GET("/containers", h.ListContainers)
		admin.POST("/containers/:id/:action", h.ContainerAction)
		admin.PUT("/containers/:id/params", h.UpdateContainerParams)
		admin.POST("/servers/:id/reboot", h.RebootServer)
	}
}
