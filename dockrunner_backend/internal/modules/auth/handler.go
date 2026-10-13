package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"dockrunner_backend/pkg/response"
)

type Handler struct {
	service *Service
	logger  *zap.Logger
}

func NewHandler(service *Service, logger *zap.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

type RegisterRequest struct {
	Login    string `json:"login" binding:"required,min=3,max=64" example:"admin"`
	Password string `json:"password" binding:"required,min=6,max=72" example:"admin123"`
}

type LoginRequest struct {
	Login    string `json:"login" binding:"required" example:"admin"`
	Password string `json:"password" binding:"required" example:"admin123"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Register godoc
// @Summary Регистрация пользователя
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.RegisterRequest true "Логин и пароль"
// @Success 201 {object} response.Success{data=auth.User}
// @Failure 400 {object} response.Error
// @Failure 409 {object} response.Error
// @Router /api/v1/auth/register [post]
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	user, err := h.service.Register(c.Request.Context(), req.Login, req.Password)
	if err != nil {
		if err == ErrUserExists {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error("register failed", zap.Error(err))
		response.Internal(c, "failed to register")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": user})
}

// Login godoc
// @Summary Вход, выдача пары токенов
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.LoginRequest true "Логин и пароль"
// @Success 200 {object} response.Success{data=auth.LoginResult}
// @Failure 400 {object} response.Error
// @Failure 401 {object} response.Error
// @Failure 403 {object} response.Error "Пользователь заблокирован"
// @Router /api/v1/auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	res, err := h.service.Login(c.Request.Context(), req.Login, req.Password)
	if err != nil {
		if err == ErrInvalidCredentials {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		if err == ErrBlocked {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error("login failed", zap.Error(err))
		response.Internal(c, "login failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": res})
}

// Me godoc
// @Summary Текущий пользователь по токену
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success{data=auth.User}
// @Failure 401 {object} response.Error
// @Failure 404 {object} response.Error
// @Router /api/v1/auth/me [get]
func (h *Handler) Me(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		response.Unauthorized(c, "unauthorized")
		return
	}
	user, err := h.service.Me(c.Request.Context(), userID)
	if err != nil || user == nil {
		response.NotFound(c, "user not found")
		return
	}
	response.OK(c, user)
}

// Refresh godoc
// @Summary Обновление пары токенов (ротация)
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.RefreshRequest true "Refresh-токен"
// @Success 200 {object} auth.RefreshResponse
// @Failure 400 {object} response.Error
// @Failure 401 {object} response.Error
// @Router /api/v1/auth/refresh [post]
func (h *Handler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	access, refresh, err := h.service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Unauthorized(c, "invalid refresh token")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":  access,
		"refresh_token": refresh,
	})
}

// Logout godoc
// @Summary Выход, отзыв refresh-токена
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success
// @Router /api/v1/auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	if err := h.service.Logout(c.Request.Context(), c.GetString("user_id")); err != nil {
		h.logger.Warn("logout revoke failed", zap.Error(err))
	}
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}
