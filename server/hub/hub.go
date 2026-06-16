// Package hub is the central WebSocket coordinator. It owns connected clients,
// presence, and the message-posting services shared with the REST handlers.
package hub

import (
	"encoding/json"
	"net/http"
	"sync"

	"murmur/ai"
	"murmur/models"
	"murmur/ratelimit"
	"murmur/settings"

	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

// Err is a typed error carrying an API/WS error code, HTTP status and optional
// retry-after hint (used by rate limiting).
type Err struct {
	Status     int
	Code       string
	Message    string
	RetryAfter int
}

func (e *Err) Error() string { return e.Message }

func newErr(status int, code, msg string) *Err {
	return &Err{Status: status, Code: code, Message: msg}
}

type Hub struct {
	db *gorm.DB
	st *settings.Service
	rl *ratelimit.Limiter
	ai *ai.Service

	upgrader websocket.Upgrader

	mu      sync.RWMutex
	clients map[uint]map[*Client]bool

	botID       uint
	botUsername string
}

func New(db *gorm.DB, st *settings.Service, rl *ratelimit.Limiter, aiSvc *ai.Service, botID uint, botUsername string) *Hub {
	return &Hub{
		db:          db,
		st:          st,
		rl:          rl,
		ai:          aiSvc,
		clients:     map[uint]map[*Client]bool{},
		botID:       botID,
		botUsername: botUsername,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			// Auth is enforced via JWT before upgrade; allow any origin so the
			// dev proxy and same-origin production both work.
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// Upgrade promotes an HTTP request to a websocket and registers the client.
func (h *Hub) Upgrade(w http.ResponseWriter, r *http.Request, user *models.User) error {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	h.addClient(user, conn)
	return nil
}

func (h *Hub) addClient(user *models.User, conn *websocket.Conn) {
	client := &Client{hub: h, conn: conn, user: user, send: make(chan []byte, sendBuffer)}

	h.mu.Lock()
	if h.clients[user.ID] == nil {
		h.clients[user.ID] = map[*Client]bool{}
	}
	wasOffline := len(h.clients[user.ID]) == 0
	h.clients[user.ID][client] = true
	h.mu.Unlock()

	go client.writePump()
	go client.readPump()

	client.trySend(envelope("ready", map[string]any{
		"user_id":         user.ID,
		"online_user_ids": h.onlineIDs(),
	}))
	if wasOffline {
		h.broadcastPresence()
	}
}

func (h *Hub) removeClient(c *Client) {
	h.mu.Lock()
	nowOffline := false
	if conns := h.clients[c.user.ID]; conns != nil {
		if conns[c] {
			delete(conns, c)
			close(c.send)
		}
		if len(conns) == 0 {
			delete(h.clients, c.user.ID)
			nowOffline = true
		}
	}
	h.mu.Unlock()
	if nowOffline {
		h.broadcastPresence()
	}
}

func (h *Hub) onlineIDs() []uint {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]uint, 0, len(h.clients))
	for id := range h.clients {
		ids = append(ids, id)
	}
	return ids
}

// IsOnline reports whether the user has at least one live connection.
func (h *Hub) IsOnline(uid uint) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[uid]) > 0
}

func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) broadcastAll(b []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, conns := range h.clients {
		for c := range conns {
			c.trySend(b)
		}
	}
}

func (h *Hub) sendToUser(uid uint, b []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[uid] {
		c.trySend(b)
	}
}

func (h *Hub) broadcastPresence() {
	h.broadcastAll(envelope("presence", map[string]any{"online_user_ids": h.onlineIDs()}))
}

func envelope(t string, fields map[string]any) []byte {
	if fields == nil {
		fields = map[string]any{}
	}
	fields["type"] = t
	b, _ := json.Marshal(fields)
	return b
}

func (c *Client) sendErr(e *Err) {
	c.trySend(envelope("error", map[string]any{
		"code":        e.Code,
		"message":     e.Message,
		"retry_after": e.RetryAfter,
	}))
}

type incoming struct {
	Type      string `json:"type"`
	ChannelID uint   `json:"channel_id"`
	Content   string `json:"content"`
	To        uint   `json:"to"`
	From      uint   `json:"from"`
	TempID    string `json:"temp_id"`
}

func (h *Hub) handleIncoming(c *Client, data []byte) {
	var in incoming
	if err := json.Unmarshal(data, &in); err != nil {
		return
	}
	switch in.Type {
	case "chat_message":
		if _, err := h.PostChannelMessage(c.user, in.ChannelID, in.Content, in.TempID); err != nil {
			c.sendErr(err)
		}
	case "dm_message":
		if _, err := h.PostDirectMessage(c.user, in.To, in.Content, in.TempID); err != nil {
			c.sendErr(err)
		}
	case "typing":
		h.handleTyping(c.user, in)
	case "read_dm":
		h.markDMRead(c.user.ID, in.From)
	case "ping":
		c.trySend(envelope("pong", nil))
	}
}

func (h *Hub) handleTyping(user *models.User, in incoming) {
	if in.To != 0 {
		h.sendToUser(in.To, envelope("typing", map[string]any{
			"user_id": user.ID,
			"from":    user.ID,
		}))
		return
	}
	if in.ChannelID != 0 {
		// Relay to everyone; clients filter by their active channel.
		h.broadcastAll(envelope("typing", map[string]any{
			"user_id":    user.ID,
			"channel_id": in.ChannelID,
		}))
	}
}

func (h *Hub) markDMRead(viewerID, fromID uint) {
	if fromID == 0 {
		return
	}
	h.db.Model(&models.DirectMessage{}).
		Where("sender_id = ? AND receiver_id = ? AND read_at IS NULL", fromID, viewerID).
		Update("read_at", gorm.Expr("CURRENT_TIMESTAMP"))
}
