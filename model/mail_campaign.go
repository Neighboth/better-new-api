package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"gorm.io/gorm"
)

// The states of a campaign of the mail of the installation.
const (
	MailCampaignQueued = "queued"
	MailCampaignActive = "running"
	MailCampaignDone   = "done"
	MailCampaignFailed = "failed"
)

// MailCampaign is one message that the administrator writes for the users.
//
// The record keeps the text of the message and the progress of the sending:
// the administrator sees how many messages left the installation and a restart
// of the service does not lose the report of a campaign.
type MailCampaign struct {
	Id            int    `json:"id" gorm:"primaryKey"`
	Subject       string `json:"subject" gorm:"type:varchar(255)"`
	Content       string `json:"content" gorm:"type:text"`
	IsHtml        bool   `json:"is_html"`
	IsPromotional bool   `json:"is_promotional"`
	// Audience is `all` or `selected`, the ids of the selected users are
	// written next to it.
	Audience  string `json:"audience" gorm:"type:varchar(32);default:'all'"`
	UserIds   string `json:"-" gorm:"type:text"`
	Status    string `json:"status" gorm:"type:varchar(32);default:'queued';index"`
	Total     int    `json:"total" gorm:"default:0"`
	Sent      int    `json:"sent" gorm:"default:0"`
	Failed    int    `json:"failed" gorm:"default:0"`
	Skipped   int    `json:"skipped" gorm:"default:0"`
	CreatedBy int    `json:"created_by" gorm:"index"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;index"`
	StartedAt int64  `json:"started_at" gorm:"bigint;default:0"`
	EndedAt   int64  `json:"ended_at" gorm:"bigint;default:0"`
	LastError string `json:"last_error" gorm:"type:varchar(512)"`
	// Failures keeps the first addresses that the server refused, a campaign
	// of ten thousand users does not fill the record of the campaign with the
	// whole list.
	Failures string `json:"failures" gorm:"type:text"`
}

// TableName is the name of the table of the campaigns.
func (MailCampaign) TableName() string {
	return "mail_campaigns"
}

// MailRecipient is one reader of a message.
type MailRecipient struct {
	Id          int
	Username    string
	DisplayName string
	Email       string
	Language    string
	// MarketingEnabled is false when the user removed the promotional mail of
	// the installation in the settings of the profile.
	MarketingEnabled bool
}

// CreateMailCampaign writes the campaign into the store.
func CreateMailCampaign(campaign *MailCampaign) error {
	if campaign.CreatedAt == 0 {
		campaign.CreatedAt = time.Now().Unix()
	}
	if campaign.Status == "" {
		campaign.Status = MailCampaignQueued
	}
	return DB.Create(campaign).Error
}

// GetMailCampaign reads one campaign.
func GetMailCampaign(id int) (*MailCampaign, error) {
	if id <= 0 {
		return nil, errors.New("the campaign is not known")
	}
	var campaign MailCampaign
	if err := DB.First(&campaign, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &campaign, nil
}

// UpdateMailCampaign writes the fields of a campaign.
func UpdateMailCampaign(id int, fields map[string]any) error {
	if id <= 0 {
		return errors.New("the campaign is not known")
	}
	return DB.Model(&MailCampaign{}).Where("id = ?", id).Updates(fields).Error
}

// ListMailCampaigns returns the last campaigns of the installation.
func ListMailCampaigns(limit int) ([]MailCampaign, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var campaigns []MailCampaign
	err := DB.Order("id DESC").Limit(limit).Find(&campaigns).Error
	return campaigns, err
}

// MarkInterruptedMailCampaigns closes the campaigns that a restart of the
// service stopped in the middle. The messages of a campaign that did not
// finish are not sent again by the next start: the administrator sees what
// left the installation and decides.
func MarkInterruptedMailCampaigns() error {
	now := time.Now().Unix()
	return DB.Model(&MailCampaign{}).
		Where("status = ?", MailCampaignActive).
		Updates(map[string]any{
			"status":     MailCampaignFailed,
			"ended_at":   now,
			"last_error": "the service was restarted while the campaign was running",
		}).Error
}

// mailRecipientQuery selects the users that can receive a message.
//
// A user without an address cannot receive a message, a disabled user is not
// written to.
func mailRecipientQuery() *gorm.DB {
	return DB.Model(&User{}).
		Select("id, username, display_name, email, setting, status").
		Where("status = ?", common.UserStatusEnabled).
		Where("email <> ''")
}

// marketingOptOut reports whether the settings of a user refuse the
// promotional messages of the installation.
//
// The flag is written into the settings of the user, the message of the
// installation is accepted by the settings of the profile. A user that never
// changed the settings of the mail accepts the messages: the default of the
// installation is `on`.
func marketingOptOut(setting string) bool {
	if strings.TrimSpace(setting) == "" {
		return false
	}
	parsed := dto.UserSetting{}
	if err := common.UnmarshalJsonStr(setting, &parsed); err != nil {
		return false
	}
	return parsed.MarketingEmailOptOut
}

// languageOf reports the language of the settings of a user.
func languageOf(setting string) string {
	if strings.TrimSpace(setting) == "" {
		return ""
	}
	parsed := dto.UserSetting{}
	if err := common.UnmarshalJsonStr(setting, &parsed); err != nil {
		return ""
	}
	return parsed.Language
}

// rowToRecipient converts a row of the query above.
func rowToRecipient(row *User) MailRecipient {
	optedOut := marketingOptOut(row.Setting)
	return MailRecipient{
		Id:               row.Id,
		Username:         row.Username,
		DisplayName:      row.DisplayName,
		Email:            row.Email,
		Language:         languageOf(row.Setting),
		MarketingEnabled: !optedOut,
	}
}

// MailAudience is the number of the readers of a message.
type MailAudience struct {
	// Addresses is the number of the enabled users with an address.
	Addresses int64 `json:"addresses"`
	// Marketing is the number of those users that accept a promotional
	// message of the installation.
	Marketing int64 `json:"marketing"`
}

// CountMailAudience counts the readers of a message.
//
// The promotional flag of the settings of a user is read from the settings, the
// database does not filter the documents of the users: the settings are a JSON
// document and the filter of a document is not the same on SQLite, MySQL and
// PostgreSQL.
func CountMailAudience() (MailAudience, error) {
	var rows []User
	if err := mailRecipientQuery().Find(&rows).Error; err != nil {
		return MailAudience{}, err
	}
	audience := MailAudience{Addresses: int64(len(rows))}
	for index := range rows {
		if !marketingOptOut(rows[index].Setting) {
			audience.Marketing++
		}
	}
	return audience, nil
}

// IterateMailRecipients walks the readers of a message in batches.
//
// The ids of the selected users limit the walk to those users; an empty list
// walks every user of the installation. The promotional flag keeps only the
// users that accept the promotional messages.
func IterateMailRecipients(ids []int, promotional bool, batchSize int, handle func([]MailRecipient) error) (int, error) {
	if batchSize <= 0 || batchSize > 1000 {
		batchSize = 200
	}
	total := 0
	offset := 0
	for {
		rows := make([]User, 0, batchSize)
		query := mailRecipientQuery()
		if len(ids) > 0 {
			query = query.Where("id IN ?", ids)
		}
		err := query.Order("id ASC").Limit(batchSize).Offset(offset).Find(&rows).Error
		if err != nil {
			return total, err
		}
		if len(rows) == 0 {
			return total, nil
		}
		offset += len(rows)

		batch := make([]MailRecipient, 0, len(rows))
		for index := range rows {
			recipient := rowToRecipient(&rows[index])
			if promotional && !recipient.MarketingEnabled {
				continue
			}
			batch = append(batch, recipient)
		}
		total += len(batch)
		if err := handle(batch); err != nil {
			return total, err
		}
	}
}

// CountSelectedMailRecipients counts the readers of a message of the selected
// users.
func CountSelectedMailRecipients(ids []int, promotional bool) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var rows []User
	if err := mailRecipientQuery().Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return 0, err
	}
	count := 0
	for index := range rows {
		if promotional && marketingOptOut(rows[index].Setting) {
			continue
		}
		count++
	}
	return count, nil
}

// SearchMailRecipients searches the readers of a message for the selector of
// the page of the users.
//
// The search runs on the server: an installation with a hundred thousand users
// does not send its users to the browser of the administrator.
func SearchMailRecipients(keyword string, limit int) ([]MailRecipient, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := mailRecipientQuery()
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("username LIKE ? OR display_name LIKE ? OR email LIKE ?", like, like, like)
	}
	var rows []User
	if err := query.Order("id ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	recipients := make([]MailRecipient, 0, len(rows))
	for index := range rows {
		recipients = append(recipients, rowToRecipient(&rows[index]))
	}
	return recipients, nil
}

// MailRecipientsByIds returns the readers of the given ids.
//
// The dialog of the mail of the table starts with the users that the
// administrator selected: the page asks for those users and it does not need
// the whole list of the users of the installation.
func MailRecipientsByIds(ids []int) ([]MailRecipient, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []User
	if err := mailRecipientQuery().Where("id IN ?", ids).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	recipients := make([]MailRecipient, 0, len(rows))
	for index := range rows {
		recipients = append(recipients, rowToRecipient(&rows[index]))
	}
	return recipients, nil
}

// SetUserMarketingEmail writes the choice of a user about the promotional mail
// of the installation.
//
// The settings of the user are kept: only the flag of the promotional mail
// changes, the notifications and the quota warning of the user stay as they are.
func SetUserMarketingEmail(userId int, enabled bool) error {
	user, err := GetUserById(userId, true)
	if err != nil {
		return err
	}
	setting := user.GetSetting()
	setting.MarketingEmailOptOut = !enabled
	if err := UpdateUserSetting(user.Id, setting); err != nil {
		return fmt.Errorf("the settings of the user could not be written: %w", err)
	}
	return nil
}
