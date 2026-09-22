package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	TicketStatusOpen        = "open"
	TicketStatusAnswered    = "answered"
	TicketStatusWaitingUser = "waiting_user"
	TicketStatusClosed      = "closed"

	TicketPriorityLow    = "low"
	TicketPriorityNormal = "normal"
	TicketPriorityHigh   = "high"
	TicketPriorityUrgent = "urgent"

	TicketCategoryBilling        = "billing"
	TicketCategoryTechnical      = "technical"
	TicketCategoryAccount        = "account"
	TicketCategoryFeatureRequest = "feature_request"
	TicketCategoryOther          = "other"
)

type Ticket struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	UserId      int    `json:"user_id" gorm:"index"`
	Title       string `json:"title" gorm:"type:varchar(255);not null"`
	Category    string `json:"category" gorm:"type:varchar(64);default:'technical'"`
	Priority    string `json:"priority" gorm:"type:varchar(32);default:'normal'"`
	Status      string `json:"status" gorm:"type:varchar(32);index;default:'open'"`
	LastReplyAt int64  `json:"last_reply_at" gorm:"bigint;index"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint"`

	// Virtual fields for display
	UserName  string `json:"user_name,omitempty" gorm:"-"`
	UserEmail string `json:"user_email,omitempty" gorm:"-"`
	UserRole  int    `json:"user_role,omitempty" gorm:"-"`
}

type TicketMessage struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	TicketId    int    `json:"ticket_id" gorm:"index;not null"`
	UserId      int    `json:"user_id" gorm:"index;not null"`
	IsAdmin     bool   `json:"is_admin" gorm:"default:false"`
	Content     string `json:"content" gorm:"type:text;not null"`
	Attachments string `json:"attachments" gorm:"type:text"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`

	// Virtual fields for display
	UserName  string `json:"user_name,omitempty" gorm:"-"`
	UserEmail string `json:"user_email,omitempty" gorm:"-"`
	UserRole  int    `json:"user_role,omitempty" gorm:"-"`
}

func CreateTicket(ticket *Ticket, initialMessage string, attachments string) error {
	now := time.Now().Unix()
	ticket.CreatedAt = now
	ticket.UpdatedAt = now
	ticket.LastReplyAt = now
	if ticket.Status == "" {
		ticket.Status = TicketStatusOpen
	}
	if ticket.Priority == "" {
		ticket.Priority = TicketPriorityNormal
	}
	if ticket.Category == "" {
		ticket.Category = TicketCategoryTechnical
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ticket).Error; err != nil {
			return err
		}

		msg := &TicketMessage{
			TicketId:    ticket.Id,
			UserId:      ticket.UserId,
			IsAdmin:     false,
			Content:     initialMessage,
			Attachments: attachments,
			CreatedAt:   now,
		}
		return tx.Create(msg).Error
	})
}

func GetTicketById(id int) (*Ticket, error) {
	var ticket Ticket
	if err := DB.First(&ticket, "id = ?", id).Error; err != nil {
		return nil, err
	}
	var user User
	if err := DB.Select("username, email, role").First(&user, "id = ?", ticket.UserId).Error; err == nil {
		ticket.UserName = user.Username
		ticket.UserEmail = user.Email
		ticket.UserRole = user.Role
	}
	return &ticket, nil
}

func GetTicketMessages(ticketId int) ([]*TicketMessage, error) {
	var messages []*TicketMessage
	if err := DB.Where("ticket_id = ?", ticketId).Order("created_at ASC").Find(&messages).Error; err != nil {
		return nil, err
	}

	userIds := make([]int, 0, len(messages))
	userMap := make(map[int]*User)
	for _, msg := range messages {
		if _, exists := userMap[msg.UserId]; !exists {
			userIds = append(userIds, msg.UserId)
			userMap[msg.UserId] = nil
		}
	}

	if len(userIds) > 0 {
		var users []*User
		if err := DB.Select("id, username, email, role").Where("id IN ?", userIds).Find(&users).Error; err == nil {
			for _, u := range users {
				userMap[u.Id] = u
			}
		}
	}

	for _, msg := range messages {
		if u := userMap[msg.UserId]; u != nil {
			msg.UserName = u.Username
			msg.UserEmail = u.Email
			msg.UserRole = u.Role
		}
	}

	return messages, nil
}

func GetUserTickets(userId int, page, pageSize int, status, search string) ([]*Ticket, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var tickets []*Ticket
	var total int64

	query := DB.Model(&Ticket{}).Where("user_id = ?", userId)
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	if search != "" {
		query = query.Where("title LIKE ?", "%"+strings.TrimSpace(search)+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("last_reply_at DESC").Offset(offset).Limit(pageSize).Find(&tickets).Error; err != nil {
		return nil, 0, err
	}

	return tickets, total, nil
}

func GetAllTickets(page, pageSize int, status, category, priority, search string) ([]*Ticket, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var tickets []*Ticket
	var total int64

	query := DB.Model(&Ticket{})
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	if category != "" && category != "all" {
		query = query.Where("category = ?", category)
	}
	if priority != "" && priority != "all" {
		query = query.Where("priority = ?", priority)
	}
	if search != "" {
		search = strings.TrimSpace(search)
		query = query.Where("title LIKE ?", "%"+search+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("last_reply_at DESC").Offset(offset).Limit(pageSize).Find(&tickets).Error; err != nil {
		return nil, 0, err
	}

	userIds := make([]int, 0, len(tickets))
	userMap := make(map[int]*User)
	for _, t := range tickets {
		if _, exists := userMap[t.UserId]; !exists {
			userIds = append(userIds, t.UserId)
			userMap[t.UserId] = nil
		}
	}

	if len(userIds) > 0 {
		var users []*User
		if err := DB.Select("id, username, email, role").Where("id IN ?", userIds).Find(&users).Error; err == nil {
			for _, u := range users {
				userMap[u.Id] = u
			}
		}
	}

	for _, t := range tickets {
		if u := userMap[t.UserId]; u != nil {
			t.UserName = u.Username
			t.UserEmail = u.Email
			t.UserRole = u.Role
		}
	}

	return tickets, total, nil
}

func AddTicketMessage(ticketId int, userId int, isAdmin bool, content string, attachments string) (*TicketMessage, error) {
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("content cannot be empty")
	}

	now := time.Now().Unix()
	msg := &TicketMessage{
		TicketId:    ticketId,
		UserId:      userId,
		IsAdmin:     isAdmin,
		Content:     content,
		Attachments: attachments,
		CreatedAt:   now,
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(msg).Error; err != nil {
			return err
		}

		updates := map[string]interface{}{
			"last_reply_at": now,
			"updated_at":    now,
		}
		if isAdmin {
			updates["status"] = TicketStatusAnswered
		} else {
			updates["status"] = TicketStatusOpen
		}

		return tx.Model(&Ticket{}).Where("id = ?", ticketId).Updates(updates).Error
	})
	if err != nil {
		return nil, err
	}

	var user User
	if err := DB.Select("username, email, role").First(&user, "id = ?", userId).Error; err == nil {
		msg.UserName = user.Username
		msg.UserEmail = user.Email
		msg.UserRole = user.Role
	}

	return msg, nil
}

func UpdateTicketStatus(ticketId int, status string) error {
	now := time.Now().Unix()
	return DB.Model(&Ticket{}).Where("id = ?", ticketId).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": now,
	}).Error
}

func UpdateTicketPriority(ticketId int, priority string) error {
	now := time.Now().Unix()
	return DB.Model(&Ticket{}).Where("id = ?", ticketId).Updates(map[string]interface{}{
		"priority":   priority,
		"updated_at": now,
	}).Error
}

func DeleteTicket(ticketId int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("ticket_id = ?", ticketId).Delete(&TicketMessage{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", ticketId).Delete(&Ticket{}).Error
	})
}
