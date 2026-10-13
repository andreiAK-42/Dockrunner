package balances

import (
	"errors"
	"strconv"

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

type AddFundsRequest struct {
	Amount float64 `json:"amount" binding:"required,gt=0" example:"1000"`
	Reason string  `json:"reason" example:"Пополнение баланса"`
}

type SetBalanceRequest struct {
	Balance float64 `json:"balance" binding:"required,gte=0" example:"5000"`
}

// GetMyBalance godoc
// @Summary Мой баланс
// @Tags balances
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success{data=float64}
// @Router /api/v1/balances/me [get]
func (h *Handler) GetMyBalance(c *gin.Context) {
	balance, err := h.service.GetBalance(c.Request.Context(), c.GetString("login"))
	if err != nil {
		h.logger.Error("get balance failed", zap.Error(err))
		response.Internal(c, "failed to get balance")
		return
	}
	response.OK(c, balance)
}

// GetMyTransactions godoc
// @Summary Мои транзакции
// @Tags balances
// @Produce json
// @Security BearerAuth
// @Param page query int false "Страница" default(1)
// @Param limit query int false "Размер страницы" default(20)
// @Success 200 {object} response.Success{data=[]balances.Transaction}
// @Router /api/v1/balances/me/transactions [get]
func (h *Handler) GetMyTransactions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	transactions, total, err := h.service.ListTransactions(c.Request.Context(), c.GetString("login"), page, limit)
	if err != nil {
		h.logger.Error("list transactions failed", zap.Error(err))
		response.Internal(c, "failed to list transactions")
		return
	}
	response.Paginated(c, transactions, total, page, limit)
}

// AdminGetUserBalance godoc
// @Summary Баланс пользователя (админ)
// @Tags balances
// @Produce json
// @Security BearerAuth
// @Param login path string true "Логин пользователя"
// @Success 200 {object} response.Success{data=float64}
// @Router /api/v1/admin/balances/{login} [get]
func (h *Handler) AdminGetUserBalance(c *gin.Context) {
	login := c.Param("login")
	balance, err := h.service.GetBalance(c.Request.Context(), login)
	if err != nil {
		h.logger.Error("get user balance failed", zap.Error(err))
		response.NotFound(c, "user not found")
		return
	}
	response.OK(c, balance)
}

// AdminAddFunds godoc
// @Summary Пополнить баланс пользователя (админ)
// @Tags balances
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param login path string true "Логин пользователя"
// @Param request body balances.AddFundsRequest true "Сумма и причина"
// @Success 200 {object} response.Success{data=float64}
// @Router /api/v1/admin/balances/{login}/add [post]
func (h *Handler) AdminAddFunds(c *gin.Context) {
	login := c.Param("login")
	var req AddFundsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	balance, err := h.service.AddFunds(c.Request.Context(), login, req.Amount, req.Reason)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			response.NotFound(c, "user not found")
			return
		}
		h.logger.Error("add funds failed", zap.Error(err))
		response.Internal(c, "failed to add funds")
		return
	}
	response.OK(c, balance)
}

// AdminSetBalance godoc
// @Summary Установить баланс пользователя (админ)
// @Tags balances
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param login path string true "Логин пользователя"
// @Param request body balances.SetBalanceRequest true "Новый баланс"
// @Success 200 {object} response.Success{data=float64}
// @Router /api/v1/admin/balances/{login} [patch]
func (h *Handler) AdminSetBalance(c *gin.Context) {
	login := c.Param("login")
	var req SetBalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := h.service.SetBalance(c.Request.Context(), login, req.Balance); err != nil {
		h.logger.Error("set balance failed", zap.Error(err))
		response.Internal(c, "failed to set balance")
		return
	}
	response.OK(c, gin.H{"balance": req.Balance})
}

// AdminListTransactions godoc
// @Summary Все транзакции (админ)
// @Tags balances
// @Produce json
// @Security BearerAuth
// @Param page query int false "Страница" default(1)
// @Param limit query int false "Размер страницы" default(20)
// @Success 200 {object} response.Success{data=[]balances.Transaction}
// @Router /api/v1/admin/balances/transactions [get]
func (h *Handler) AdminListTransactions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	transactions, total, err := h.service.ListAllTransactions(c.Request.Context(), page, limit)
	if err != nil {
		h.logger.Error("list all transactions failed", zap.Error(err))
		response.Internal(c, "failed to list transactions")
		return
	}
	response.Paginated(c, transactions, total, page, limit)
}