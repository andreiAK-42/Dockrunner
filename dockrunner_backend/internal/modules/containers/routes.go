package containers

import (
	"dockrunner_backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, jwtSecret string) {
	// WebSocket stream authenticates via ?token= (browsers set no headers),
	// so it lives outside the Bearer middleware group.
	rg.GET("/containers/:id/build-log/stream", h.BuildLogStream)
	containers := rg.Group("/containers")
	containers.Use(middleware.Auth(jwtSecret))
	{
		containers.POST("/check", h.StartCheck)
		containers.GET("/check/:id", h.GetCheck)
		containers.POST("/run", h.Run)
		containers.POST("/refresh", h.Refresh)
		containers.GET("/mine", h.Mine)
		containers.GET("/:id/logs", h.ContainerLogs)
		containers.GET("/:id/spec", h.ContainerSpec)
		containers.POST("/:id/recreate", h.RecreateContainer)        // admin direct recreate
		containers.POST("/:id/request-update", h.RequestUpdate)       // user request update (needs admin approval)
		containers.GET("/:id/stats", h.ContainerStats)
		containers.GET("/:id/build-log", h.BuildLog)
		containers.POST("/:id/start", h.Start)
		containers.POST("/:id/stop", h.Stop)
		containers.DELETE("/:id", h.Remove)
	}
}
