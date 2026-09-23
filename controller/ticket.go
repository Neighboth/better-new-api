package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

func GetTicketConfig(c *gin.Context) {
	setting := operation_setting.GetTicketSetting()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":                   setting.Enabled,
			"live_support_enabled":      setting.LiveSupportEnabled,
			"notify_admin_on_new_ticket": setting.NotifyAdminOnNewTicket,
			"notify_user_on_reply":       setting.NotifyUserOnReply,
		},
	})
}

func GetUserTickets(c *gin.Context) {
	if !operation_setting.IsTicketEnabled() {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Ticket system is disabled",
		})
		return
	}

	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"items": []*model.Ticket{},
				"total": int64(0),
			},
		})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")
	search := c.Query("search")

	tickets, total, err := model.GetUserTickets(userId, page, pageSize, status, search)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items": tickets,
			"total": total,
		},
	})
}

type CreateTicketRequest struct {
	Title       string `json:"title"`
	Category    string `json:"category"`
	Priority    string `json:"priority"`
	Content     string `json:"content"`
	Attachments string `json:"attachments"`
}

func CreateTicket(c *gin.Context) {
	if !operation_setting.IsTicketEnabled() {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Ticket system is disabled",
		})
		return
	}

	userId := c.GetInt("id")
	var req CreateTicketRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid request payload",
		})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)

	if req.Title == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Title cannot be empty",
		})
		return
	}
	if req.Content == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Message content cannot be empty",
		})
		return
	}

	ticket := &model.Ticket{
		UserId:   userId,
		Title:    req.Title,
		Category: req.Category,
		Priority: req.Priority,
	}

	if err := model.CreateTicket(ticket, req.Content, req.Attachments); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	loadedTicket, err := model.GetTicketById(ticket.Id)
	if err == nil {
		ticket = loadedTicket
	}

	service.GlobalTicketHub.BroadcastToAdmins("new_ticket", ticket)
	service.NotifyAdminNewTicketEmail(ticket, req.Content)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    ticket,
	})
}

func GetTicketDetail(c *gin.Context) {
	if !operation_setting.IsTicketEnabled() {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Ticket system is disabled",
		})
		return
	}

	userId := c.GetInt("id")
	role := c.GetInt("role")
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid ticket ID",
		})
		return
	}

	ticket, err := model.GetTicketById(ticketId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Ticket not found",
		})
		return
	}

	if role < common.RoleAdminUser && ticket.UserId != userId {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Permission denied",
		})
		return
	}

	messages, err := model.GetTicketMessages(ticketId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"ticket":   ticket,
			"messages": messages,
		},
	})
}

type AddTicketMessageRequest struct {
	Content     string `json:"content"`
	Attachments string `json:"attachments"`
}

func AddTicketMessage(c *gin.Context) {
	if !operation_setting.IsTicketEnabled() {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Ticket system is disabled",
		})
		return
	}

	userId := c.GetInt("id")
	role := c.GetInt("role")
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid ticket ID",
		})
		return
	}

	ticket, err := model.GetTicketById(ticketId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Ticket not found",
		})
		return
	}

	isAdmin := role >= common.RoleAdminUser
	if !isAdmin && ticket.UserId != userId {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Permission denied",
		})
		return
	}

	if ticket.Status == model.TicketStatusClosed && !isAdmin {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Ticket is closed and cannot receive further replies",
		})
		return
	}

	var req AddTicketMessageRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid request payload",
		})
		return
	}

	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Content cannot be empty",
		})
		return
	}

	msg, err := model.AddTicketMessage(ticketId, userId, isAdmin, req.Content, req.Attachments)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	service.GlobalTicketHub.BroadcastToTicket(ticketId, "new_message", msg)

	if isAdmin {
		service.NotifyUserTicketReplyEmail(ticket, req.Content)
	} else {
		service.NotifyAdminNewTicketEmail(ticket, req.Content)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    msg,
	})
}

func CloseTicket(c *gin.Context) {
	userId := c.GetInt("id")
	role := c.GetInt("role")
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid ticket ID",
		})
		return
	}

	ticket, err := model.GetTicketById(ticketId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Ticket not found",
		})
		return
	}

	if role < common.RoleAdminUser && ticket.UserId != userId {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Permission denied",
		})
		return
	}

	if err := model.UpdateTicketStatus(ticketId, model.TicketStatusClosed); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	service.GlobalTicketHub.BroadcastToTicket(ticketId, "status_changed", map[string]any{
		"status": model.TicketStatusClosed,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func TicketWebSocket(c *gin.Context) {
	userId := c.GetInt("id")
	role := c.GetInt("role")

	if userId == 0 {
		token := c.Query("token")
		if token != "" {
			identity, internal, err := service.ParseDashboardAccessToken(token)
			if internal && err == nil {
				_, user, err := service.ValidateLoginSession(identity)
				if err == nil && user != nil && user.Status == common.UserStatusEnabled {
					userId = user.Id
					role = user.Role
				}
			}
		}
	}

	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Unauthorized",
		})
		return
	}

	service.HandleTicketWebSocket(c, userId, role)
}

// Admin handlers

func AdminGetAllTickets(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")
	category := c.Query("category")
	priority := c.Query("priority")
	search := c.Query("search")

	tickets, total, err := model.GetAllTickets(page, pageSize, status, category, priority, search)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items": tickets,
			"total": total,
		},
	})
}

type AdminUpdateTicketRequest struct {
	Status   string `json:"status"`
	Priority string `json:"priority"`
}

func AdminUpdateTicket(c *gin.Context) {
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid ticket ID",
		})
		return
	}

	var req AdminUpdateTicketRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid request payload",
		})
		return
	}

	if req.Status != "" {
		if err := model.UpdateTicketStatus(ticketId, req.Status); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		service.GlobalTicketHub.BroadcastToTicket(ticketId, "status_changed", map[string]any{
			"status": req.Status,
		})
	}

	if req.Priority != "" {
		if err := model.UpdateTicketPriority(ticketId, req.Priority); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func AdminDeleteTicket(c *gin.Context) {
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Invalid ticket ID",
		})
		return
	}

	if err := model.DeleteTicket(ticketId); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}
