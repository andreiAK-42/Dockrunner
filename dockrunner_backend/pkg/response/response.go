package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Error struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

type Success struct {
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Success{Data: data})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Success{Data: data})
}

func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, Error{Error: msg, Code: "BAD_REQUEST"})
}

func Unauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, Error{Error: msg, Code: "UNAUTHORIZED"})
}

func Forbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, Error{Error: msg, Code: "FORBIDDEN"})
}

func NotFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, Error{Error: msg, Code: "NOT_FOUND"})
}

func Conflict(c *gin.Context, msg string) {
	c.JSON(http.StatusConflict, Error{Error: msg, Code: "CONFLICT"})
}

func Internal(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, Error{Error: msg, Code: "INTERNAL"})
}

func Paginated(c *gin.Context, data interface{}, total int64, page, limit int) {
	c.JSON(http.StatusOK, gin.H{
		"data": data,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}
