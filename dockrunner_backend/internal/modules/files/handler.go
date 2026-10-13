package files

import (
	"io"
	"net/http"
	"path"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"dockrunner_backend/pkg/response"
)

const maxUploadBytes = 20 * 1024 * 1024

type Handler struct {
	service *Service
	logger  *zap.Logger
}

func NewHandler(service *Service, logger *zap.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// resolveOwner возвращает владельца папки: админ может указать чужой логин
// через owner (query/body/form), обычный пользователь — только себя.
func resolveOwner(c *gin.Context, override string) string {
	if override != "" && c.GetString("role") == "admin" {
		return override
	}
	return c.GetString("login")
}

// List godoc
// @Summary Список файлов в папке пользователя на воркере
// @Tags files
// @Produce json
// @Security BearerAuth
// @Param node query string true "ID воркера"
// @Param path query string false "Путь внутри папки пользователя" default(/)
// @Param owner query string false "Логин владельца (только админ)" default()
// @Success 200 {object} response.Success{data=[]files.Entry}
// @Router /api/v1/files [get]
func (h *Handler) List(c *gin.Context) {
	entries, err := h.service.List(c.Request.Context(), resolveOwner(c, c.Query("owner")), c.Query("node"), c.Query("path"))
	if err != nil {
		h.logger.Warn("files list failed", zap.Error(err))
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, entries)
}

type mkdirRequest struct {
	NodeID string `json:"node_id" binding:"required" example:"worker-01"`
	Path   string `json:"path" binding:"required" example:"data/db"`
	Owner  string `json:"owner" example:"ivan"`
}

// Mkdir godoc
// @Summary Создать папку в директории пользователя
// @Tags files
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body files.mkdirRequest true "Воркер и путь"
// @Success 200 {object} response.Success
// @Router /api/v1/files/mkdir [post]
func (h *Handler) Mkdir(c *gin.Context) {
	var req mkdirRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.service.Mkdir(c.Request.Context(), resolveOwner(c, req.Owner), req.NodeID, req.Path); err != nil {
		h.logger.Warn("files mkdir failed", zap.Error(err))
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "created"})
}

// Upload godoc
// @Summary Загрузить файл (заменяет существующий)
// @Tags files
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param node_id formData string true "ID воркера"
// @Param path formData string true "Путь внутри папки пользователя"
// @Param file formData file true "Файл (до 20 МБ)"
// @Success 200 {object} response.Success
// @Router /api/v1/files/upload [post]
func (h *Handler) Upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)
	node := c.PostForm("node_id")
	uploadPath := c.PostForm("path")
	uploadOwner := resolveOwner(c, c.PostForm("owner"))
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file is required")
		return
	}
	src, err := file.Open()
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	defer func() { _ = src.Close() }()
	data := make([]byte, file.Size)
	if _, err := io.ReadFull(src, data); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.service.Upload(c.Request.Context(), uploadOwner, node, uploadPath, data); err != nil {
		h.logger.Warn("files upload failed", zap.Error(err))
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "uploaded"})
}

// Download godoc
// @Summary Скачать файл из папки пользователя
// @Tags files
// @Produce octet-stream
// @Security BearerAuth
// @Param node query string true "ID воркера"
// @Param path query string true "Путь внутри папки пользователя"
// @Success 200 {file} binary
// @Router /api/v1/files/download [get]
func (h *Handler) Download(c *gin.Context) {
	uploadPath := c.Query("path")
	data, err := h.service.Download(c.Request.Context(), resolveOwner(c, c.Query("owner")), c.Query("node"), uploadPath)
	if err != nil {
		h.logger.Warn("files download failed", zap.Error(err))
		response.BadRequest(c, err.Error())
		return
	}
	c.Header("Content-Disposition", "attachment; filename="+path.Base(uploadPath))
	c.Data(http.StatusOK, "application/octet-stream", data)
}

// Delete godoc
// @Summary Удалить файл или папку из директории пользователя
// @Tags files
// @Produce json
// @Security BearerAuth
// @Param node query string true "ID воркера"
// @Param path query string true "Путь внутри папки пользователя"
// @Success 200 {object} response.Success
// @Router /api/v1/files [delete]
func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), resolveOwner(c, c.Query("owner")), c.Query("node"), c.Query("path")); err != nil {
		h.logger.Warn("files delete failed", zap.Error(err))
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}
