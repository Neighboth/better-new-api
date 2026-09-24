package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	TicketStatusOpen        = "open"
	TicketStatusAnswered    = "answered"
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
	Id              int    `json:"id" gorm:"primaryKey"`
	UserId          int    `json:"user_id" gorm:"index"`
	IsGuest         bool   `json:"is_guest" gorm:"default:false;index"`
	GuestContact    string `json:"guest_contact" gorm:"type:varchar(128);index"` // email or phone
	GuestSessionKey string `json:"guest_session_key,omitempty" gorm:"type:varchar(64);index"` // secret token to retrieve guest ticket
	Title           string `json:"title" gorm:"type:varchar(255);not null"`
	Category        string `json:"category" gorm:"type:varchar(64);default:'technical'"`
	Priority        string `json:"priority" gorm:"type:varchar(32);default:'normal'"`
	Status          string `json:"status" gorm:"type:varchar(32);index;default:'open'"`
	LastReplyAt     int64  `json:"last_reply_at" gorm:"bigint;index"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint"`

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
	if ticket.IsGuest {
		ticket.UserName = "Misafir (" + ticket.GuestContact + ")"
		ticket.UserEmail = ticket.GuestContact
		ticket.UserRole = 0
		return &ticket, nil
	}
	var user User
	if err := DB.Select("username, email, role").First(&user, "id = ?", ticket.UserId).Error; err == nil {
		ticket.UserName = user.Username
		ticket.UserEmail = user.Email
		ticket.UserRole = user.Role
	}
	return &ticket, nil
}

func GetTicketBySessionKey(sessionKey string) (*Ticket, error) {
	if strings.TrimSpace(sessionKey) == "" {
		return nil, errors.New("empty session key")
	}
	var ticket Ticket
	if err := DB.First(&ticket, "guest_session_key = ?", sessionKey).Error; err != nil {
		return nil, err
	}
	ticket.UserName = "Misafir (" + ticket.GuestContact + ")"
	ticket.UserEmail = ticket.GuestContact
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
		if msg.UserId > 0 {
			if _, exists := userMap[msg.UserId]; !exists {
				userIds = append(userIds, msg.UserId)
				userMap[msg.UserId] = nil
			}
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
		if msg.UserId == 0 {
			if msg.IsAdmin {
				msg.UserName = "Destek Yetkilisi"
			} else {
				msg.UserName = "Misafir"
			}
		} else if u := userMap[msg.UserId]; u != nil {
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
		if t.IsGuest {
			t.UserName = "Misafir (" + t.GuestContact + ")"
			t.UserEmail = t.GuestContact
		} else if u := userMap[t.UserId]; u != nil {
			t.UserName = u.Username
			t.UserEmail = u.Email
			t.UserRole = u.Role
		}
	}

	return tickets, total, nil
}

func CreateGuestTicket(contact string, initialMessage string) (*Ticket, error) {
	now := time.Now().Unix()
	ticket := &Ticket{
		Title:           "Canlı Destek (" + contact + ")",
		Category:        TicketCategoryTechnical,
		Priority:        TicketPriorityNormal,
		Status:          TicketStatusOpen,
		UserId:          0,
		IsGuest:         true,
		GuestContact:    contact,
		GuestSessionKey: common.GetUUID(),
		CreatedAt:       now,
		UpdatedAt:       now,
		LastReplyAt:     now,
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ticket).Error; err != nil {
			return err
		}

		msg := &TicketMessage{
			TicketId:  ticket.Id,
			UserId:    0,
			IsAdmin:   false,
			Content:   initialMessage,
			CreatedAt: now,
		}
		return tx.Create(msg).Error
	})
	if err != nil {
		return nil, err
	}
	return ticket, nil
}

func AddGuestTicketMessage(sessionKey string, content string, attachments string) (*TicketMessage, error) {
	ticket, err := GetTicketBySessionKey(sessionKey)
	if err != nil {
		return nil, err
	}
	if ticket.Status == TicketStatusClosed {
		return nil, errors.New("cannot reply to a closed ticket")
	}

	now := time.Now().Unix()
	msg := &TicketMessage{
		TicketId:    ticket.Id,
		UserId:      0,
		IsAdmin:     false,
		Content:     content,
		Attachments: attachments,
		CreatedAt:   now,
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(msg).Error; err != nil {
			return err
		}

		return tx.Model(&Ticket{}).Where("id = ?", ticket.Id).Updates(map[string]interface{}{
			"last_reply_at": now,
			"updated_at":    now,
			"status":        TicketStatusOpen,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	msg.UserName = "Misafir"
	msg.UserEmail = ticket.GuestContact
	return msg, nil
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
	if status == "waiting_user" {
		status = TicketStatusAnswered
	}
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

func DeleteTicketPermanently(ticketId int) error {
	return DeleteTicket(ticketId)
}

func GenerateTicketTranscript(ticket *Ticket, messages []*TicketMessage) string {
	var sb strings.Builder
	sb.WriteString("# Destek Talebi Görüşme Geçmişi (Transcript)\n\n")
	sb.WriteString(fmt.Sprintf("- **Talep No:** #%d\n", ticket.Id))
	sb.WriteString(fmt.Sprintf("- **Başlık:** %s\n", ticket.Title))
	sb.WriteString(fmt.Sprintf("- **Kategori:** %s\n", ticket.Category))
	sb.WriteString(fmt.Sprintf("- **Öncelik:** %s\n", ticket.Priority))
	sb.WriteString(fmt.Sprintf("- **Durum:** %s\n", ticket.Status))
	if ticket.IsGuest {
		sb.WriteString(fmt.Sprintf("- **Kullanıcı:** Misafir (%s)\n", ticket.GuestContact))
	} else {
		sb.WriteString(fmt.Sprintf("- **Kullanıcı:** %s (%s)\n", ticket.UserName, ticket.UserEmail))
	}
	sb.WriteString(fmt.Sprintf("- **Oluşturulma Tarihi:** %s\n\n", time.Unix(ticket.CreatedAt, 0).Format("2006-01-02 15:04:05")))
	sb.WriteString("---\n\n## Mesajlar\n\n")

	for _, msg := range messages {
		sender := msg.UserName
		if sender == "" {
			if msg.IsAdmin {
				sender = "Destek Yetkilisi"
			} else {
				sender = "Kullanıcı"
			}
		}
		roleLabel := "Kullanıcı"
		if msg.IsAdmin {
			roleLabel = "Destek Ekibi"
		}
		tStr := time.Unix(msg.CreatedAt, 0).Format("2006-01-02 15:04:05")
		sb.WriteString(fmt.Sprintf("### [%s] %s (%s)\n\n", tStr, sender, roleLabel))
		sb.WriteString(msg.Content + "\n\n")
		if msg.Attachments != "" {
			sb.WriteString(fmt.Sprintf("*Ekler:* %s\n\n", msg.Attachments))
		}
	}
	return sb.String()
}

func MigrateLegacyTicketStatus() {
	if DB != nil && DB.Migrator().HasTable("tickets") {
		_ = DB.Model(&Ticket{}).Where("status = ?", "waiting_user").Update("status", TicketStatusAnswered).Error
	}
}
