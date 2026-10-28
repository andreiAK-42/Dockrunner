package deploymentrequests

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"dockrunner_backend/internal/modules/balances"
	"dockrunner_backend/internal/oplock"
	"dockrunner_backend/pkg/response"
)

type Handler struct {
	service *Service
	logger  *zap.Logger
	isDev   bool
}

func NewHandler(service *Service, logger *zap.Logger, isDev bool) *Handler {
	return &Handler{service: service, logger: logger, isDev: isDev}
}

// Default container resource limits (shared with containers module)
const (
	DefaultCPUCores  = 0.25
	DefaultMemoryGB  = 0.5  // 512 MB
	DefaultDiskGB    = 1.0
)

type CreateRequest struct {
	AppName       string            `json:"app_name" binding:"required" example:"my-awesome-app"`
	RepoURL       string            `json:"repo_url" example:"https://github.com/org/repo"`
	Image         string            `json:"image" example:""`
	ContainerName string            `json:"container_name" example:"web-01"`
	NodeID        string            `json:"node_id" binding:"required" example:"worker-01"`
	Ports         []PortMapping     `json:"ports"`
	Volumes       []VolumeSpec      `json:"volumes"`
	Env           map[string]string `json:"env"`
	CPUCores      float64           `json:"cpu_cores" example:"0.25"`
	MemoryGB      float64           `json:"memory_gb" example:"0.5"`
	DiskGB        float64           `json:"disk_gb" example:"1"`
	DurationHours float64           `json:"duration_hours" binding:"required,gt=0" example:"10"`
	Services      []string          `json:"services" example:"front,back"`
	BuildMode     string            `json:"build_mode" example:"compose"`
	Branch        string            `json:"branch" example:"main"`
}

func applyDefaults(req *CreateRequest) {
	if req.CPUCores <= 0 {
		req.CPUCores = DefaultCPUCores
	}
	if req.MemoryGB <= 0 {
		req.MemoryGB = DefaultMemoryGB
	}
	if req.DiskGB <= 0 {
		req.DiskGB = DefaultDiskGB
	}
}

type RejectRequest struct {
	Comment string `json:"comment" example:"Недостаточно ресурсов"`
}

type SetPriceRequest struct {
	Price float64 `json:"price" binding:"required,gte=0" example:"150.50"`
}

// Create godoc
// @Summary Создать заявку на запуск контейнера
// @Tags deployment-requests
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body deploymentrequests.CreateRequest true "Параметры заявки"
// @Success 201 {object} response.Success{data=deploymentrequests.Request}
// @Failure 400 {object} response.Error
// @Router /api/v1/deployment-requests [post]
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	applyDefaults(&req)

	owner := c.GetString("login")
	id := uuid.New()

	request := &Request{
		ID:            id,
		OwnerLogin:    owner,
		AppName:       req.AppName,
		RepoURL:       req.RepoURL,
		Image:         req.Image,
		ContainerName: req.ContainerName,
		NodeID:        req.NodeID,
		Ports:         req.Ports,
		Volumes:       req.Volumes,
		Env:           req.Env,
		CPUCores:      req.CPUCores,
		MemoryGB:      req.MemoryGB,
		DiskGB:        req.DiskGB,
		DurationHours: req.DurationHours,
		Services:      req.Services,
		BuildMode:     req.BuildMode,
		Branch:        req.Branch,
		Price:         0,
		Status:        "pending",
		Comment:       "",
		DecidedBy:     "",
		DecidedAt:     nil,
		DeploymentID:  "",
		CreatedAt:     time.Time{},
		UpdatedAt:     time.Time{},
	}

	if err := h.service.Create(c.Request.Context(), request); err != nil {
		h.logger.Error("create request failed", zap.Error(err))
		if h.isDev {
			response.Internal(c, err.Error())
		} else {
			response.Internal(c, "failed to create request")
		}
		return
	}

	// Calculate price
	price, err := h.service.CalculatePrice(c.Request.Context(), request)
	if err != nil {
		h.logger.Warn("price calculation failed", zap.Error(err))
	} else {
		_ = h.service.SetPrice(c.Request.Context(), id, price)
		request.Price = price
	}

	c.JSON(http.StatusCreated, gin.H{"data": request})
}

// Get godoc
// @Summary Получить заявку по ID
// @Tags deployment-requests
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID заявки"
// @Success 200 {object} response.Success{data=deploymentrequests.Request}
// @Failure 404 {object} response.Error
// @Router /api/v1/deployment-requests/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	req, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "request not found")
		return
	}

	// Check ownership
	if req.OwnerLogin != c.GetString("login") {
		response.NotFound(c, "request not found")
		return
	}

	response.OK(c, req)
}

// ListMine godoc
// @Summary Мои заявки
// @Tags deployment-requests
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success{data=[]deploymentrequests.Request}
// @Router /api/v1/deployment-requests/mine [get]
func (h *Handler) ListMine(c *gin.Context) {
	requests, err := h.service.ListByOwner(c.Request.Context(), c.GetString("login"))
	if err != nil {
		h.logger.Error("list requests failed", zap.Error(err))
		response.Internal(c, "failed to list requests")
		return
	}
	response.OK(c, requests)
}

// ListPending godoc (admin)
// @Summary Все ожидающие заявки (админ)
// @Tags deployment-requests
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success{data=[]deploymentrequests.Request}
// @Router /api/v1/admin/deployment-requests/pending [get]
func (h *Handler) ListPending(c *gin.Context) {
	requests, err := h.service.ListPending(c.Request.Context())
	if err != nil {
		h.logger.Error("list pending requests failed", zap.Error(err))
		response.Internal(c, "failed to list requests")
		return
	}
	response.OK(c, requests)
}

// ListAll godoc (admin)
// @Summary Все заявки (админ)
// @Tags deployment-requests
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success{data=[]deploymentrequests.Request}
// @Router /api/v1/admin/deployment-requests [get]
func (h *Handler) ListAll(c *gin.Context) {
	requests, err := h.service.ListAll(c.Request.Context())
	if err != nil {
		h.logger.Error("list all requests failed", zap.Error(err))
		response.Internal(c, "failed to list requests")
		return
	}
	response.OK(c, requests)
}

// Approve godoc (admin)
// @Summary Одобрить заявку (админ)
// @Tags deployment-requests
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID заявки"
// @Success 200 {object} response.Success
// @Failure 400 {object} response.Error
// @Failure 404 {object} response.Error
// @Failure 409 {object} response.Error
// @Router /api/v1/admin/deployment-requests/{id}/approve [post]
func (h *Handler) Approve(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	deploymentID, err := h.service.Approve(c.Request.Context(), id, c.GetString("login"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "request not found")
			return
		}
		if errors.Is(err, ErrInvalidState) {
			response.BadRequest(c, "request cannot be approved")
			return
		}
		if errors.Is(err, balances.ErrInsufficientFunds) {
			response.BadRequest(c, "у пользователя недостаточно средств")
			return
		}
		if errors.Is(err, oplock.ErrBusy) {
			response.Conflict(c, err.Error())
			return
		}
		h.logger.Warn("approve failed", zap.Error(err))
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "request approved", "deployment_id": deploymentID})
}

// Reject godoc (admin)
// @Summary Отклонить заявку (админ)
// @Tags deployment-requests
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID заявки"
// @Param request body deploymentrequests.RejectRequest true "Причина отказа"
// @Success 200 {object} response.Success
// @Failure 400 {object} response.Error
// @Failure 404 {object} response.Error
// @Router /api/v1/admin/deployment-requests/{id}/reject [post]
func (h *Handler) Reject(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	var req RejectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	err = h.service.Reject(c.Request.Context(), id, c.GetString("login"), req.Comment)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "request not found")
			return
		}
		if errors.Is(err, ErrInvalidState) {
			response.BadRequest(c, "request cannot be rejected")
			return
		}
		h.logger.Error("reject failed", zap.Error(err))
		response.Internal(c, "failed to reject")
		return
	}
	response.OK(c, gin.H{"message": "request rejected"})
}

// SetPrice godoc (admin)
// @Summary Установить цену заявки (админ)
// @Tags deployment-requests
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID заявки"
// @Param request body deploymentrequests.SetPriceRequest true "Цена"
// @Success 200 {object} response.Success
// @Failure 400 {object} response.Error
// @Failure 404 {object} response.Error
// @Router /api/v1/admin/deployment-requests/{id}/price [patch]
func (h *Handler) SetPrice(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	var req SetPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	err = h.service.SetPrice(c.Request.Context(), id, req.Price)
	if err != nil {
		h.logger.Error("set price failed", zap.Error(err))
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "price updated"})
}
