package files

import (
	"dockrunner_backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, jwtSecret string) {
	files := rg.Group("/files")
	files.Use(middleware.Auth(jwtSecret))
	{
		files.GET("", h.List)
		files.POST("/mkdir", h.Mkdir)
		files.POST("/upload", h.Upload)
		files.GET("/download", h.Download)
		files.DELETE("", h.Delete)
	}
}
