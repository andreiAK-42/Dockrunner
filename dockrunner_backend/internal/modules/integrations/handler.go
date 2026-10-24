package integrations

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

// Catalog godoc
// @Summary Каталог интеграций (postgres, mongo, redis, ollama)
// @Tags integrations
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success{data=[]integrations.Kind}
// @Router /api/v1/integrations [get]
func (h *Handler) Catalog(c *gin.Context) {
	response.OK(c, h.service.Catalog())
}

// Mine godoc
// @Summary Мои учётки интеграций + занятое место и лимит БД
// @Tags integrations
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success
// @Router /api/v1/integrations/users [get]
func (h *Handler) Mine(c *gin.Context) {
	owner := c.GetString("login")
	users, err := h.service.Mine(c.Request.Context(), owner)
	if err != nil {
		h.logger.Error("list integration accounts failed", zap.Error(err))
		response.Internal(c, "failed to list integration accounts")
		return
	}
	usedGB, limitGB := h.service.Usage(c.Request.Context(), owner)
	response.OK(c, gin.H{"users": users, "usage_gb": usedGB, "limit_gb": limitGB})
}

type createRequest struct {
	Kind   string `json:"kind" example:"postgres"`
	DbType string `json:"db_type" example:"postgres"`
}

// Create godoc
// @Summary Создать учётку интеграции (пароль — только в ответе)
// @Tags integrations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body integrations.createRequest true "Тип интеграции"
// @Success 201 {object} response.Success
// @Failure 400 {object} response.Error
// @Failure 409 {object} response.Error
// @Router /api/v1/integrations/users [post]
func (h *Handler) Create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	kind := req.Kind
	if kind == "" {
		kind = req.DbType
	}
	if kind == "" {
		response.BadRequest(c, "укажите kind")
		return
	}
	created, err := h.service.Create(c.Request.Context(), c.GetString("login"), kind)
	if err != nil {
		h.writeErr(c, err)
		return
	}
	response.Created(c, gin.H{
		"user":              created.Account,
		"rows":              created.Rows,
		"connection_string": created.ConnectionString,
		"size_gb":           created.Account.SizeGB,
	})
}

// ResetPassword godoc
// @Summary Сбросить пароль учётки интеграции
// @Tags integrations
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID учётки"
// @Success 200 {object} response.Success
// @Router /api/v1/integrations/users/{id}/reset-password [post]
func (h *Handler) ResetPassword(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	created, err := h.service.ResetPassword(c.Request.Context(), c.GetString("login"), id)
	if err != nil {
		h.writeErr(c, err)
		return
	}
	response.OK(c, gin.H{
		"user":              created.Account,
		"rows":              created.Rows,
		"connection_string": created.ConnectionString,
	})
}

// Delete godoc
// @Summary Удалить учётку интеграции
// @Tags integrations
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID учётки"
// @Success 200 {object} response.Success
// @Failure 404 {object} response.Error
// @Router /api/v1/integrations/users/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	if err := h.service.Delete(c.Request.Context(), c.GetString("login"), id); err != nil {
		h.writeErr(c, err)
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

func (h *Handler) writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrUnknownKind), errors.Is(err, ErrQuotaExceeded):
		response.BadRequest(c, err.Error())
	case errors.Is(err, ErrAlreadyExists):
		response.Conflict(c, err.Error())
	case errors.Is(err, ErrNotFound):
		response.NotFound(c, "учётка не найдена")
	default:
		h.logger.Warn("integration operation failed", zap.Error(err))
		response.BadRequest(c, err.Error())
	}
}
