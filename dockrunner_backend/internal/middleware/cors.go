package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
}

func CORS(cfg CORSConfig) gin.HandlerFunc {
	origins := cfg.AllowedOrigins
	if len(origins) == 0 {
		origins = []string{"*"}
	}
	methods := cfg.AllowedMethods
	if len(methods) == 0 {
		methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	}
	headers := cfg.AllowedHeaders
	if len(headers) == 0 {
		headers = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"}
	}

	return cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     methods,
		AllowHeaders:     headers,
		ExposeHeaders:    []string{"Content-Length", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}
