package users

import (
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"dockrunner_backend/pkg/response"
)

func forbiddenOrInternal(c *gin.Context, err error, what string, logger *zap.Logger) {
	if errors.Is(err, ErrForbidden) {
		response.Forbidden(c, err.Error())
		return
	}
	logger.Error(what, zap.Error(err))
	response.Internal(c, "failed to update user")
}

var errInvalidDate = errors.New("invalid date")

type Handler struct {
	service    *Service
	logger     *zap.Logger
	bcryptCost int
}

func NewHandler(service *Service, logger *zap.Logger, bcryptCost int) *Handler {
	return &Handler{service: service, logger: logger, bcryptCost: bcryptCost}
}

// List godoc
// @Summary Список пользователей (админ)
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param page query int false "Страница" default(1)
// @Param limit query int false "Размер страницы" default(20)
// @Success 200 {object} response.Success{data=[]users.User}
// @Failure 403 {object} response.Error
// @Router /api/v1/users [get]
func (h *Handler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	users, total, err := h.service.List(c.Request.Context(), page, limit)
	if err != nil {
		h.logger.Error("list users failed", zap.Error(err))
		response.Internal(c, "failed to list users")
		return
	}
	response.Paginated(c, users, total, page, limit)
}

// GetByID godoc
// @Summary Пользователь по id
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID пользователя"
// @Success 200 {object} response.Success{data=users.User}
// @Failure 404 {object} response.Error
// @Router /api/v1/users/{id} [get]
func (h *Handler) GetByID(c *gin.Context) {
	id := c.Param("id")
	user, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	if user == nil {
		response.NotFound(c, "user not found")
		return
	}
	response.OK(c, user)
}

type UpdateRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=admin user viewer" example:"user"`
}

// UpdateRole godoc
// @Summary Сменить роль (админ)
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID пользователя"
// @Param request body users.UpdateRoleRequest true "Новая роль"
// @Success 200 {object} response.Success
// @Router /api/v1/users/{id}/role [patch]
func (h *Handler) UpdateRole(c *gin.Context) {
	id := c.Param("id")
	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.service.UpdateRole(c.Request.Context(), c.GetString("login"), id, req.Role); err != nil {
		forbiddenOrInternal(c, err, "update role failed", h.logger)
		return
	}
	response.OK(c, gin.H{"message": "role updated"})
}

// Delete godoc
// @Summary Удалить пользователя (админ)
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID пользователя"
// @Success 204 "Удалён"
// @Router /api/v1/users/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.Delete(c.Request.Context(), c.GetString("login"), id); err != nil {
		forbiddenOrInternal(c, err, "delete user failed", h.logger)
		return
	}
	c.JSON(204, nil)
}

type BlockRequest struct {
	Blocked bool `json:"blocked" example:"true"`
}

// SetBlocked godoc
// @Summary Заблокировать / разблокировать (админ)
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID пользователя"
// @Param request body users.BlockRequest true "Флаг блокировки"
// @Success 200 {object} response.Success
// @Router /api/v1/users/{id}/block [patch]
func (h *Handler) SetBlocked(c *gin.Context) {
	id := c.Param("id")
	var req BlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.service.SetBlocked(c.Request.Context(), c.GetString("login"), id, req.Blocked); err != nil {
		forbiddenOrInternal(c, err, "block user failed", h.logger)
		return
	}
	response.OK(c, gin.H{"message": "user updated"})
}

type PasswordRequest struct {
	Password string `json:"password" binding:"required,min=6,max=72" example:"new-secret123"`
}

// SetPassword godoc
// @Summary Сменить пароль пользователя (админ; админам — только главный)
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID пользователя"
// @Param request body users.PasswordRequest true "Новый пароль (6..72)"
// @Success 200 {object} response.Success
// @Failure 403 {object} response.Error
// @Router /api/v1/users/{id}/password [patch]
func (h *Handler) SetPassword(c *gin.Context) {
	id := c.Param("id")
	var req PasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.service.SetPassword(c.Request.Context(), c.GetString("login"), id, req.Password, h.bcryptCost); err != nil {
		forbiddenOrInternal(c, err, "set password failed", h.logger)
		return
	}
	response.OK(c, gin.H{"message": "password updated"})
}

type PlanRequest struct {
	PlanID  string  `json:"plan_id" binding:"required,oneof=student basic pro" example:"basic"`
	Expires *string `json:"expires" example:"2026-12-31"`
}

func parseExpiry(raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, *raw); err == nil {
			utc := t.UTC()
			return &utc, nil
		}
	}
	return nil, errInvalidDate
}

// SetPlan godoc
// @Summary Сменить тариф и срок (админ)
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID пользователя"
// @Param request body users.PlanRequest true "Тариф и дата окончания (пусто — бессрочно)"
// @Success 200 {object} response.Success
// @Router /api/v1/users/{id}/plan [patch]
func (h *Handler) SetPlan(c *gin.Context) {
	id := c.Param("id")
	var req PlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	expires, err := parseExpiry(req.Expires)
	if err != nil {
		response.BadRequest(c, "invalid expires date")
		return
	}
	if err := h.service.SetPlan(c.Request.Context(), id, req.PlanID, expires); err != nil {
		h.logger.Error("set plan failed", zap.Error(err))
		response.Internal(c, "failed to update plan")
		return
	}
	response.OK(c, gin.H{"message": "plan updated"})
}

type AdminCreateRequest struct {
	Login    string `json:"login" binding:"required,min=3,max=64" example:"ivan"`
	Password string `json:"password" binding:"required,min=6,max=72" example:"secret123"`
	Role     string `json:"role" binding:"required,oneof=admin user viewer" example:"user"`
	PlanID   string `json:"plan_id" binding:"required,oneof=student basic pro" example:"student"`
}

// AdminCreate godoc
// @Summary Создать пользователя (админ)
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body users.AdminCreateRequest true "Данные пользователя"
// @Success 201 {object} response.Success{data=users.User}
// @Failure 400 {object} response.Error
// @Router /api/v1/users [post]
func (h *Handler) AdminCreate(c *gin.Context) {
	var req AdminCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	user, err := h.service.AdminCreate(c.Request.Context(), req.Login, req.Password, req.Role, req.PlanID, h.bcryptCost)
	if err != nil {
		h.logger.Error("admin create failed", zap.Error(err))
		response.Internal(c, "failed to create user")
		return
	}
	response.Created(c, user)
}
