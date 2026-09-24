package service

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type TicketClient struct {
	Hub      *TicketHub
	Conn     *websocket.Conn
	UserId   int
	Role     int
	TicketId int
	Send     chan []byte
}

type TicketHub struct {
	clients    map[*TicketClient]bool
	register   chan *TicketClient
	unregister chan *TicketClient
	mu         sync.RWMutex
}

var GlobalTicketHub = NewTicketHub()

func NewTicketHub() *TicketHub {
	hub := &TicketHub{
		clients:    make(map[*TicketClient]bool),
		register:   make(chan *TicketClient),
		unregister: make(chan *TicketClient),
	}
	go hub.run()
	return hub
}

func (h *TicketHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			h.mu.Unlock()
		}
	}
}

func (h *TicketHub) BroadcastToTicket(ticketId int, messageType string, data any) {
	payload := map[string]any{
		"type":      messageType,
		"ticket_id": ticketId,
		"data":      data,
		"timestamp": time.Now().Unix(),
	}

	bytes, err := common.Marshal(payload)
	if err != nil {
		common.SysError("failed to marshal ticket broadcast payload: " + err.Error())
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		// Send if client is listening to this ticket, or if client is an admin listening to all tickets
		if client.TicketId == ticketId || (client.Role >= common.RoleAdminUser && client.TicketId == 0) {
			select {
			case client.Send <- bytes:
			default:
				// Buffer full
			}
		}
	}
}

func (h *TicketHub) BroadcastToAdmins(messageType string, data any) {
	payload := map[string]any{
		"type":      messageType,
		"data":      data,
		"timestamp": time.Now().Unix(),
	}

	bytes, err := common.Marshal(payload)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.Role >= common.RoleAdminUser {
			select {
			case client.Send <- bytes:
			default:
			}
		}
	}
}

func (c *TicketClient) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(65536)
	_ = c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var incoming map[string]any
		if err := common.Unmarshal(message, &incoming); err != nil {
			continue
		}

		msgType, _ := incoming["type"].(string)
		switch msgType {
		case "subscribe":
			if tid, ok := incoming["ticket_id"].(float64); ok {
				c.TicketId = int(tid)
			}
		case "ping":
			pongBytes, _ := common.Marshal(map[string]string{"type": "pong"})
			select {
			case c.Send <- pongBytes:
			default:
			}
		case "send_message":
			// Live chat or ticket message sent over WebSocket
			content, _ := incoming["content"].(string)
			attachments, _ := incoming["attachments"].(string)
			ticketId := c.TicketId
			if tid, ok := incoming["ticket_id"].(float64); ok && int(tid) > 0 {
				ticketId = int(tid)
			}

			if ticketId > 0 && content != "" {
				isAdmin := c.Role >= common.RoleAdminUser
				msg, err := model.AddTicketMessage(ticketId, c.UserId, isAdmin, content, attachments)
				if err == nil {
					c.Hub.BroadcastToTicket(ticketId, "new_message", msg)
					ticket, _ := model.GetTicketById(ticketId)
					if ticket != nil {
						if isAdmin {
							NotifyUserTicketReplyEmail(ticket, content)
						} else {
							NotifyAdminNewTicketEmail(ticket, content)
						}
					}
				}
			}
		}
	}
}

func (c *TicketClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			n := len(c.Send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func HandleTicketWebSocket(c *gin.Context, userId int, role int) {
	ticketIdStr := c.Query("ticket_id")
	ticketId := 0
	if ticketIdStr != "" {
		ticketId, _ = strconv.Atoi(ticketIdStr)
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		common.SysError("failed to upgrade ticket websocket: " + err.Error())
		return
	}

	client := &TicketClient{
		Hub:      GlobalTicketHub,
		Conn:     conn,
		UserId:   userId,
		Role:     role,
		TicketId: ticketId,
		Send:     make(chan []byte, 256),
	}

	client.Hub.register <- client

	go client.writePump()
	go client.readPump()
}

func NotifyAdminNewTicketEmail(ticket *model.Ticket, initialMessage string) {
	setting := operation_setting.GetTicketSetting()
	if !setting.NotifyAdminOnNewTicket {
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				common.SysError(fmt.Sprintf("panic in NotifyAdminNewTicketEmail: %v", r))
			}
		}()

		rootUser := model.GetRootUser()
		if rootUser == nil || rootUser.Email == "" {
			return
		}

		subject, content := RenderEmail("ticket_created", "tr", map[string]string{
			"ticket_id":    strconv.Itoa(ticket.Id),
			"ticket_title": ticket.Title,
			"user_name":    ticket.UserName,
			"user_email":   ticket.UserEmail,
			"category":     ticket.Category,
			"priority":     ticket.Priority,
			"message":      initialMessage,
		})
		_ = common.SendEmail(subject, rootUser.Email, content)
	}()
}

func NotifyUserTicketReplyEmail(ticket *model.Ticket, replyMessage string) {
	setting := operation_setting.GetTicketSetting()
	if !setting.NotifyUserOnReply {
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				common.SysError(fmt.Sprintf("panic in NotifyUserTicketReplyEmail: %v", r))
			}
		}()

		if ticket.UserEmail == "" {
			user, err := model.GetUserById(ticket.UserId, false)
			if err != nil || user.Email == "" {
				return
			}
			ticket.UserEmail = user.Email
		}

		subject, content := RenderEmail("ticket_replied", "tr", map[string]string{
			"ticket_id":      strconv.Itoa(ticket.Id),
			"ticket_title":   ticket.Title,
			"reply_message":  replyMessage,
			"dashboard_link": PaymentReturnURL("/tickets"),
		})
		_ = common.SendEmail(subject, ticket.UserEmail, content)
	}()
}
