// Package realtime implements the server -> browser event channel used by the
// dashboard. A single WebSocket per session multiplexes container, check and
// deployment-request events; the client subscribes to topics and the hub
// delivers only events the user is allowed to see.
package realtime

import (
	"encoding/json"
	"sync"

	"github.com/coder/websocket"
	"go.uber.org/zap"
)

const (
	TopicContainers          = "containers"
	TopicChecks              = "checks"
	TopicDeploymentRequests  = "deployment_requests"
)

// Event is one message pushed to browsers. Owner/AdminOnly are routing fields
// and never leave the server; the client sees type/topic/data.
type Event struct {
	Type      string `json:"type"`
	Topic     string `json:"topic"`
	Data      any    `json:"data,omitempty"`
	Owner     string `json:"-"`
	AdminOnly bool   `json:"-"`
}

// Hub keeps the open browser connections and fans events out to them.
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}
	logger  *zap.Logger
}

func NewHub(logger *zap.Logger) *Hub {
	return &Hub{clients: map[*Client]struct{}{}, logger: logger}
}

// Emit routes an event to the owner and to administrators. A nil hub is a
// valid no-op so services stay usable in dev mode without a WebSocket layer.
func (h *Hub) Emit(eventType, topic, owner string, data any) {
	h.publish(Event{Type: eventType, Topic: topic, Data: data, Owner: owner})
}

// EmitAdminOnly routes an event to administrators only.
func (h *Hub) EmitAdminOnly(eventType, topic string, data any) {
	h.publish(Event{Type: eventType, Topic: topic, Data: data, AdminOnly: true})
}

func (h *Hub) publish(event Event) {
	if h == nil {
		return
	}

	payload, err := json.Marshal(event)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("realtime event marshal failed", zap.String("type", event.Type), zap.Error(err))
		}
		return
	}

	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		if c.accepts(event) {
			clients = append(clients, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range clients {
		c.trySend(payload)
	}
}

func (h *Hub) register(c *Client) {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) unregister(c *Client) {
	if h == nil {
		return
	}
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

// Client is one browser WebSocket connection.
type Client struct {
	hub   *Hub
	conn  *websocket.Conn
	login string
	role  string

	mu     sync.RWMutex
	topics map[string]bool

	send      chan []byte
	closeOnce sync.Once
}

func newClient(hub *Hub, conn *websocket.Conn, login, role string) *Client {
	return &Client{
		hub:   hub,
		conn:  conn,
		login: login,
		role:  role,
		topics: map[string]bool{
			TopicContainers:         true,
			TopicChecks:             true,
			TopicDeploymentRequests: true,
		},
		send: make(chan []byte, 128),
	}
}

func (c *Client) accepts(event Event) bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	subscribed := c.topics[event.Topic]
	c.mu.RUnlock()
	if !subscribed {
		return false
	}
	if event.AdminOnly {
		return c.role == "admin"
	}
	if event.Owner == "" {
		return true
	}
	return c.login == event.Owner || c.role == "admin"
}

func (c *Client) trySend(payload []byte) {
	select {
	case c.send <- payload:
	default:
		// A client that cannot keep up is disconnected: it will reconnect and
		// pull a fresh snapshot via the regular HTTP endpoints.
		c.closeNow()
	}
}

func (c *Client) closeNow() {
	c.closeOnce.Do(func() {
		_ = c.conn.CloseNow()
	})
}

func (c *Client) setTopics(topics []string, enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, topic := range topics {
		if topic == "" {
			continue
		}
		if enabled {
			c.topics[topic] = true
		} else {
			delete(c.topics, topic)
		}
	}
}
