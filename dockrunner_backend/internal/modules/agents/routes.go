package agents

import (
	"github.com/gin-gonic/gin"
	"dockrunner_backend/internal/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, jwtSecret, agentToken string) {
	agents := rg.Group("/agents")
	{
		agents.POST("/heartbeat", middleware.AgentAuth(agentToken), h.Heartbeat)

		protected := agents.Group("")
		protected.Use(middleware.Auth(jwtSecret))
		{
			protected.GET("", h.List)
		}
	}
}
