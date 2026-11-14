package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/authz"

	"github.com/gin-gonic/gin"
)

// GetUserDetail aggregates everything an administrator may need to know about
// one account in a single request: the profile (without secrets), the recent
// usage logs, the live dashboard sessions, the 2FA state, the OAuth bindings,
// the subscriptions and the ticket count. The route is admin-only and sits
// behind the user:read permission; it exists so the user detail page does not
// fan out into a dozen separate requests.
func GetUserDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid user id")
		return
	}
	myRole := c.GetInt("role")
	user, err := model.GetUserById(id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !canManageTargetRole(myRole, user.Role) {
		common.ApiErrorI18n(c, i18n.MsgUserNoPermissionSameLevel)
		return
	}
	user.AdminPermissions = authz.Capabilities(user.Id, user.Role)

	now := common.GetTimestamp()

	// Recent usage logs (latest 12). A failure here must not hide the rest
	// of the profile.
	recentLogs, logsTotal, logsErr := model.GetUserLogs(id, 0, 0, 0, "", "", 0, 12, "", "", "")
	if logsErr != nil {
		recentLogs = []*model.Log{}
		logsTotal = 0
	}

	sessions, sessionsErr := model.ListActiveUserSessions(id, "", now)
	if sessionsErr != nil {
		sessions = []model.UserSession{}
	}
	sessionsCount := int64(len(sessions))

	twoFAEnabled, twoFAErr := model.IsTwoFAEnabled(id)
	if twoFAErr != nil {
		twoFAEnabled = false
	}

	bindings, bindingsErr := model.GetUserOAuthBindingsByUserId(id)
	if bindingsErr != nil {
		bindings = nil
	}

	subscriptions, subsErr := model.GetAllUserSubscriptions(id)
	if subsErr != nil {
		subscriptions = []model.SubscriptionSummary{}
	}

	_, ticketsTotal, ticketsErr := model.GetUserTickets(id, 1, 1, "", "")
	if ticketsErr != nil {
		ticketsTotal = 0
	}

	common.ApiSuccess(c, gin.H{
		"user":                  user,
		"recent_logs":           recentLogs,
		"recent_logs_total":     logsTotal,
		"active_sessions":       sessions,
		"active_sessions_count": sessionsCount,
		"two_fa_enabled":        twoFAEnabled,
		"oauth_bindings":        bindings,
		"subscriptions":         subscriptions,
		"tickets_count":         ticketsTotal,
		"server_time":           now,
	})
}
