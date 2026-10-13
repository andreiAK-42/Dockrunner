package databases

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

// Engines godoc
// @Summary Доступные СУБД (postgres, mongo, redis)
// @Tags databases
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success{data=[]databases.Engine}
// @Router /api/v1/databases [get]
func (h *Handler) Engines(c *gin.Context) {
	response.OK(c, h.service.Engines())
}

// Mine godoc
// @Summary Мои пользователи СУБД + занятое место и лимит
// @Tags databases
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success
// @Router /api/v1/databases/users [get]
func (h *Handler) Mine(c *gin.Context) {
	owner := c.GetString("login")
	users, err := h.service.Mine(c.Request.Context(), owner)
	if err != nil {
		h.logger.Error("list db users failed", zap.Error(err))
		response.Internal(c, "failed to list database users")
		return
	}
	usedGB, limitGB := h.service.Usage(c.Request.Context(), owner)
	response.OK(c, gin.H{"users": users, "usage_gb": usedGB, "limit_gb": limitGB})
}

type createRequest struct {
	DbType string `json:"db_type" binding:"required,oneof=postgres mongo redis" example:"postgres"`
}

// Create godoc
// @Summary Создать пользователя СУБД (пароль — только в ответе)
// @Tags databases
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body databases.createRequest true "Тип СУБД"
// @Success 201 {object} response.Success
// @Failure 400 {object} response.Error
// @Router /api/v1/databases/users [post]
func (h *Handler) Create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	created, err := h.service.Create(c.Request.Context(), c.GetString("login"), req.DbType)
	if err != nil {
		if errors.Is(err, ErrUnknownEngine) || errors.Is(err, ErrQuotaExceeded) {
			response.BadRequest(c, err.Error())
			return
		}
		h.logger.Warn("create db user failed", zap.Error(err))
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(201, gin.H{"data": gin.H{
		"user":              created.DbUser,
		"rows":              created.Rows,
		"connection_string": created.ConnectionString,
		"size_gb":           created.DbUser.SizeGB,
	}})
}

// Delete godoc
// @Summary Удалить пользователя СУБД
// @Tags databases
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID пользователя СУБД"
// @Success 200 {object} response.Success
// @Failure 404 {object} response.Error
// @Router /api/v1/databases/users/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	if err := h.service.Delete(c.Request.Context(), c.GetString("login"), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "database user not found")
			return
		}
		h.logger.Warn("delete db user failed", zap.Error(err))
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}
