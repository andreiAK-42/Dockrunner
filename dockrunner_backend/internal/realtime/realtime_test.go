package realtime

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"dockrunner_backend/internal/middleware"
)

func testToken(t *testing.T, secret, login, role string) string {
	t.Helper()
	claims := middleware.Claims{
		Login: login,
		Role:  role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func dial(t *testing.T, serverURL, token string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(serverURL, "http") + "/ws?token=" + url.QueryEscape(token)
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return conn
}

func TestStreamDeliversOwnerEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "test-secret"

	hub := NewHub(zap.NewNop())
	router := gin.New()
	router.GET("/ws", NewHandler(hub, zap.NewNop(), secret).Stream)
	server := httptest.NewServer(router)
	defer server.Close()

	conn := dial(t, server.URL, testToken(t, secret, "alice", "user"))
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// Accept() completes before register(), so give the handler a moment.
	time.Sleep(50 * time.Millisecond)
	hub.Emit("container.updated", TopicContainers, "alice", map[string]any{"id": "c1", "status": "running"})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if event.Type != "container.updated" || event.Topic != TopicContainers {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestStreamFiltersForeignOwnerEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "test-secret"

	hub := NewHub(zap.NewNop())
	router := gin.New()
	router.GET("/ws", NewHandler(hub, zap.NewNop(), secret).Stream)
	server := httptest.NewServer(router)
	defer server.Close()

	conn := dial(t, server.URL, testToken(t, secret, "alice", "user"))
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	time.Sleep(50 * time.Millisecond)

	hub.Emit("container.updated", TopicContainers, "bob", map[string]any{"id": "c2"})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, _, err := conn.Read(ctx); err == nil {
		t.Fatal("foreign owner's event must not be delivered")
	}
}
