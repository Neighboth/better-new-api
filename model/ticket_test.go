package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTicketLifecycle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	originalDB := DB
	DB = db
	t.Cleanup(func() { DB = originalDB })

	require.NoError(t, db.AutoMigrate(&User{}, &Ticket{}, &TicketMessage{}))

	user := &User{
		Username: "ticketuser",
		Email:    "ticketuser@example.com",
		AffCode:  "ticketuser1",
		Role:     1,
	}
	require.NoError(t, db.Create(user).Error)

	admin := &User{
		Username: "ticketadmin",
		Email:    "admin@example.com",
		AffCode:  "ticketadmin1",
		Role:     10,
	}
	require.NoError(t, db.Create(admin).Error)

	// 1. Create ticket
	ticket := &Ticket{
		UserId:   user.Id,
		Title:    "Issue with API balance",
		Category: TicketCategoryBilling,
		Priority: TicketPriorityHigh,
	}
	err = CreateTicket(ticket, "My balance was not updated after payment", "")
	require.NoError(t, err)
	assert.NotEmpty(t, ticket.Id)
	assert.Equal(t, TicketStatusOpen, ticket.Status)

	// 2. Fetch ticket by id
	loaded, err := GetTicketById(ticket.Id)
	require.NoError(t, err)
	assert.Equal(t, ticket.Title, loaded.Title)
	assert.Equal(t, user.Username, loaded.UserName)
	assert.Equal(t, user.Email, loaded.UserEmail)

	// 3. Messages check
	msgs, err := GetTicketMessages(ticket.Id)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, "My balance was not updated after payment", msgs[0].Content)
	assert.False(t, msgs[0].IsAdmin)
	assert.Equal(t, user.Username, msgs[0].UserName)

	// 4. Admin replies
	adminReply, err := AddTicketMessage(ticket.Id, admin.Id, true, "We checked and resolved the payment.", "")
	require.NoError(t, err)
	assert.True(t, adminReply.IsAdmin)

	loaded, err = GetTicketById(ticket.Id)
	require.NoError(t, err)
	assert.Equal(t, TicketStatusAnswered, loaded.Status)

	// 5. User replies back
	userReply, err := AddTicketMessage(ticket.Id, user.Id, false, "Thank you, confirmed!", "")
	require.NoError(t, err)
	assert.False(t, userReply.IsAdmin)

	loaded, err = GetTicketById(ticket.Id)
	require.NoError(t, err)
	assert.Equal(t, TicketStatusOpen, loaded.Status)

	// 6. User tickets listing
	userTickets, total, err := GetUserTickets(user.Id, 1, 10, "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, userTickets, 1)

	// 7. Admin tickets listing
	allTickets, totalAdmin, err := GetAllTickets(1, 10, "", "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), totalAdmin)
	assert.Len(t, allTickets, 1)

	// 8. Close ticket
	err = UpdateTicketStatus(ticket.Id, TicketStatusClosed)
	require.NoError(t, err)
	loaded, err = GetTicketById(ticket.Id)
	require.NoError(t, err)
	assert.Equal(t, TicketStatusClosed, loaded.Status)

	// 9. Delete ticket
	err = DeleteTicket(ticket.Id)
	require.NoError(t, err)
	_, err = GetTicketById(ticket.Id)
	assert.Error(t, err)
}
