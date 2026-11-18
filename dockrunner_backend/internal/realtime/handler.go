package realtime

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"dockrunner_backend/internal/middleware"
)

const (
	writeTimeout  = 10 * time.Second
	pingInterval  = 25 * time.Second
	readLimit     = 32 << 10
)

// Handler exposes GET /api/v1/ws?token=<JWT>.
type Handler struct {
	hub       *Hub
	logger    *zap.Logger
	jwtSecret string
}

func NewHandler(hub *Hub, logger *zap.Logger, jwtSecret string) *Handler {
	return &Handler{hub: hub, logger: logger, jwtSecret: jwtSecret}
}

// Stream godoc
// @Summary Канал событий панели по WebSocket
// @Description Один канал на сессию: контейнеры, проверки репозитория и заявки.
// @Description Токен — в ?token=, браузер не ставит заголовки. Если токена в
// @Description query нет, первым сообщением нужно отправить {"action":"auth","token":"..."}
// @Description в течение 5 секунд. Клиент может подписаться/отписаться сообщениями
// @Description {"action":"subscribe","topics":["containers"]}.
// @Tags realtime
// @Produce json
// @Param token query string false "JWT пользователя"
// @Success 101 "Переключение на WebSocket"
// @Failure 401 {object} gin.H
// @Router /api/v1/ws [get]
func (h *Handler) Stream(c *gin.Context) {
	claims, ok := h.queryClaims(c)
	if c.Query("token") != "" && !ok {
		// Токен передан, но невалиден — 401 до апгрейда, как раньше.
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}

	// WebSockets are long-lived, while the HTTP server sets a write deadline
	// for regular responses. Clear it on the hijacked connection.
	hijacked := &hijackWriter{ResponseWriter: c.Writer}
	conn, err := websocket.Accept(hijacked, c.Request, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		h.logger.Warn("realtime accept failed", zap.Error(err))
		return
	}

	if !ok {
		// Токена в query нет — ждём {"action":"auth","token":"..."} первым сообщением.
		claims, ok = h.readAuthMessage(conn)
		if !ok {
			_ = conn.Close(websocket.StatusPolicyViolation, "auth required")
			return
		}
	}

	client := newClient(h.hub, conn, claims.Login, claims.Role)
	h.hub.register(client)
	defer func() {
		h.hub.unregister(client)
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()

	h.serve(c.Request.Context(), client)
}

func (h *Handler) serve(ctx context.Context, client *Client) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	client.conn.SetReadLimit(readLimit)

	go client.readLoop(ctx, cancel)
	client.writeLoop(ctx)
}

func (c *Client) readLoop(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()
	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			return
		}

		var msg struct {
			Action string   `json:"action"`
			Topics []string `json:"topics"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		switch msg.Action {
		case "subscribe":
			c.setTopics(msg.Topics, true)
		case "unsubscribe":
			c.setTopics(msg.Topics, false)
		}
	}
}

func (c *Client) writeLoop(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case payload, ok := <-c.send:
			if !ok {
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Write(writeCtx, websocket.MessageText, payload)
			cancel()
			if err != nil {
				return
			}
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (h *Handler) queryClaims(c *gin.Context) (*middleware.Claims, bool) {
	raw := c.Query("token")
	if raw == "" {
		return nil, false
	}
	return h.parseClaims(raw)
}

func (h *Handler) parseClaims(raw string) (*middleware.Claims, bool) {
	claims := &middleware.Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(h.jwtSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid {
		return nil, false
	}
	return claims, true
}

// readAuthMessage ждёт первым сообщением {"action":"auth","token":"..."}.
// Нужно, чтобы не светить JWT в URL WebSocket (логи прокси, history).
func (h *Handler) readAuthMessage(conn *websocket.Conn) (*middleware.Claims, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn.SetReadLimit(4 << 10)
	_, data, err := conn.Read(ctx)
	if err != nil {
		return nil, false
	}
	var msg struct {
		Action string `json:"action"`
		Token  string `json:"token"`
	}
	if err := json.Unmarshal(data, &msg); err != nil || msg.Token == "" {
		return nil, false
	}
	return h.parseClaims(msg.Token)
}

// hijackWriter clears the HTTP server deadlines before handing the connection
// to the WebSocket library, otherwise long-lived sockets die after WriteTimeout.
type hijackWriter struct {
	http.ResponseWriter
}

func (w *hijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	conn, brw, err := hijacker.Hijack()
	if err == nil {
		_ = conn.SetDeadline(time.Time{})
	}
	return conn, brw, err
}

func (w *hijackWriter) WriteHeaderNow() {
	if now, ok := w.ResponseWriter.(interface{ WriteHeaderNow() }); ok {
		now.WriteHeaderNow()
	}
}
