package balances

import (
	"github.com/gin-gonic/gin"
	"dockrunner_backend/internal/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, jwtSecret string) {
	// User routes
	user := rg.Group("/balances")
	user.Use(middleware.Auth(jwtSecret))
	{
		user.GET("/me", h.GetMyBalance)
		user.GET("/me/transactions", h.GetMyTransactions)
	}

	// Admin routes
	admin := rg.Group("/admin/balances")
	admin.Use(middleware.Auth(jwtSecret), middleware.RequireRole("admin"))
	{
		admin.GET("/transactions", h.AdminListTransactions)
		admin.GET("/:login", h.AdminGetUserBalance)
		admin.POST("/:login/add", h.AdminAddFunds)
		admin.PATCH("/:login", h.AdminSetBalance)
	}
}