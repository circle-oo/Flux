package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// Event types for WebSocket broadcasting.
const (
	EventTaskUpdated = "TASK_UPDATED"
	EventGoalChanged = "GOAL_CHANGED"
	EventPRStatus    = "PR_STATUS"
	EventPodStatus   = "POD_STATUS"
)

// Event represents a WebSocket event.
type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// maxClients is the maximum number of concurrent WebSocket connections.
const maxClients = 10

// WebSocketHub manages WebSocket client connections and broadcasting.
type WebSocketHub struct {
	clients   map[*wsClient]bool
	mu        sync.RWMutex
	broadcast chan Event
	stop      chan struct{}
}

type wsClient struct {
	conn   *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc
}

// NewWebSocketHub creates a new WebSocketHub.
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:   make(map[*wsClient]bool),
		broadcast: make(chan Event, 256),
		stop:      make(chan struct{}),
	}
}

// Run starts the WebSocket hub's broadcast loop.
func (h *WebSocketHub) Run() {
	for {
		select {
		case <-h.stop:
			return
		case event := <-h.broadcast:
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}

			h.mu.RLock()
			clients := make([]*wsClient, 0, len(h.clients))
			for client := range h.clients {
				clients = append(clients, client)
			}
			h.mu.RUnlock()

			for _, client := range clients {
				ctx, cancel := context.WithTimeout(client.ctx, 5*time.Second)
				err = client.conn.Write(ctx, websocket.MessageText, data)
				cancel()
				if err != nil {
					slog.Debug("websocket write error, removing client", "error", err)
					go h.removeClient(client)
				}
			}
		}
	}
}

// Stop stops the WebSocket hub.
func (h *WebSocketHub) Stop() {
	close(h.stop)
	h.mu.Lock()
	for client := range h.clients {
		client.cancel()
		client.conn.Close(websocket.StatusGoingAway, "server shutting down")
	}
	h.clients = make(map[*wsClient]bool)
	h.mu.Unlock()
}

// Broadcast sends an event to all connected clients.
func (h *WebSocketHub) Broadcast(event Event) {
	select {
	case h.broadcast <- event:
	default:
		slog.Warn("websocket broadcast channel full, dropping event", "type", event.Type)
	}
}

func (h *WebSocketHub) addClient(client *wsClient) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.clients) >= maxClients {
		return false
	}
	h.clients[client] = true
	return true
}

func (h *WebSocketHub) removeClient(client *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		client.cancel()
		client.conn.Close(websocket.StatusNormalClosure, "")
	}
}

// handleWebSocket handles GET /ws/events
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		slog.Error("websocket accept error", "error", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	client := &wsClient{
		conn:   conn,
		ctx:    ctx,
		cancel: cancel,
	}

	if !s.ws.addClient(client) {
		conn.Close(websocket.StatusTryAgainLater, "too many connections")
		cancel()
		return
	}

	slog.Info("websocket client connected", "clients", len(s.ws.clients))

	// Keepalive ping/pong every 30 seconds
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
				err := conn.Ping(pingCtx)
				pingCancel()
				if err != nil {
					s.ws.removeClient(client)
					return
				}
			}
		}
	}()

	// Read loop (consume messages to detect disconnection)
	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			s.ws.removeClient(client)
			slog.Info("websocket client disconnected")
			return
		}
	}
}
