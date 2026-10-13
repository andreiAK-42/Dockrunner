package auth

import (
	"github.com/gin-gonic/gin"
	"dockrunner_backend/internal/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, jwtSecret string) {
	auth := rg.Group("/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.Refresh)

		// protected
		protected := auth.Group("")
		protected.Use(middleware.Auth(jwtSecret))
		{
			protected.GET("/me", h.Me)
			protected.POST("/logout", h.Logout)
		}
	}
}
