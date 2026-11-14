package model

import (
	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// ProxyPoolProxy is an admin-registered outbound proxy that can be bound to
// individual keys of a multi-key channel. The relay resolves the proxy per
// request through service.EffectiveRelayProxyURL.
type ProxyPoolProxy struct {
	Id             int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name           string `json:"name" gorm:"type:varchar(191);not null;default:''"`
	URL            string `json:"url" gorm:"type:text;not null"`
	Enabled        bool   `json:"enabled" gorm:"not null;default:true"`
	LastTestStatus string `json:"last_test_status" gorm:"type:varchar(32);default:''"`
	LastTestError  string `json:"last_test_error" gorm:"type:text;default:''"`
	LastTestedAt   int64  `json:"last_tested_at" gorm:"not null;default:0"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

// ProxyPoolAssignment binds one key (by index) of one multi-key channel to a
// proxy from the pool. The unique composite index guarantees one proxy per key.
type ProxyPoolAssignment struct {
	Id        int   `json:"id" gorm:"primaryKey;autoIncrement"`
	ChannelId int   `json:"channel_id" gorm:"not null;uniqueIndex:idx_proxy_pool_assignment_key"`
	KeyIndex  int   `json:"key_index" gorm:"not null;uniqueIndex:idx_proxy_pool_assignment_key"`
	ProxyId   int   `json:"proxy_id" gorm:"not null;index"`
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

func ListProxyPoolProxies() ([]ProxyPoolProxy, error) {
	var proxies []ProxyPoolProxy
	err := DB.Order("id ASC").Find(&proxies).Error
	return proxies, err
}

func GetProxyPoolProxy(id int) (*ProxyPoolProxy, error) {
	var proxy ProxyPoolProxy
	err := DB.First(&proxy, id).Error
	if err != nil {
		return nil, err
	}
	return &proxy, nil
}

func CreateProxyPoolProxy(proxy *ProxyPoolProxy) error {
	now := common.GetTimestamp()
	proxy.CreatedAt = now
	proxy.UpdatedAt = now
	return DB.Create(proxy).Error
}

func UpdateProxyPoolProxy(proxy *ProxyPoolProxy) error {
	proxy.UpdatedAt = common.GetTimestamp()
	return DB.Save(proxy).Error
}

func DeleteProxyPoolProxy(id int) error {
	// Removing a proxy also drops every key bound to it so no key is left
	// silently routed through an unmanaged exit point.
	if err := DB.Where("proxy_id = ?", id).Delete(&ProxyPoolAssignment{}).Error; err != nil {
		return err
	}
	return DB.Delete(&ProxyPoolProxy{}, id).Error
}

func ListProxyPoolAssignments(channelId int) ([]ProxyPoolAssignment, error) {
	var assignments []ProxyPoolAssignment
	query := DB.Model(&ProxyPoolAssignment{})
	if channelId > 0 {
		query = query.Where("channel_id = ?", channelId)
	}
	err := query.Order("channel_id ASC, key_index ASC").Find(&assignments).Error
	return assignments, err
}

func ReplaceProxyPoolAssignments(channelId int, mapping map[int]int) error {
	now := common.GetTimestamp()
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("channel_id = ?", channelId).Delete(&ProxyPoolAssignment{}).Error; err != nil {
			return err
		}
		for keyIndex, proxyId := range mapping {
			assignment := &ProxyPoolAssignment{
				ChannelId: channelId,
				KeyIndex:  keyIndex,
				ProxyId:   proxyId,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := tx.Create(assignment).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func DeleteProxyPoolAssignments(channelId int) error {
	return DB.Where("channel_id = ?", channelId).Delete(&ProxyPoolAssignment{}).Error
}

// ProxyPoolAssignmentURLMap loads the (channel_id, key_index) -> proxy URL
// mapping for every assignment whose proxy is enabled. Disabled proxies are
// skipped, leaving their keys on the channel's own default path (or direct).
func ProxyPoolAssignmentURLMap() (map[int]map[int]string, error) {
	var rows []struct {
		ChannelId int
		KeyIndex  int
		URL       string
	}
	err := DB.Table("proxy_pool_assignments").
		Select("proxy_pool_assignments.channel_id AS channel_id, proxy_pool_assignments.key_index AS key_index, proxy_pool_proxies.url AS url").
		Joins("JOIN proxy_pool_proxies ON proxy_pool_proxies.id = proxy_pool_assignments.proxy_id").
		Where("proxy_pool_proxies.enabled = ?", true).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int]map[int]string, 16)
	for _, row := range rows {
		channelKeys, ok := result[row.ChannelId]
		if !ok {
			channelKeys = make(map[int]string)
			result[row.ChannelId] = channelKeys
		}
		channelKeys[row.KeyIndex] = row.URL
	}
	return result, nil
}

// MultiKeyChannelSummary is the safe projection the Proxy Pool admin page
// lists: identity plus key count and current per-key assignment state. Keys
// themselves are never exposed.
type MultiKeyChannelSummary struct {
	Id           int    `json:"id"`
	Name         string `json:"name"`
	Type         int    `json:"type"`
	Status       int    `json:"status"`
	Group        string `json:"group"`
	MultiKeySize int    `json:"multi_key_size"`
}

// ListMultiKeyChannels returns every multi-key channel ordered by id.
func ListMultiKeyChannels() ([]MultiKeyChannelSummary, error) {
	var channels []Channel
	if err := DB.Order("id ASC").Find(&channels).Error; err != nil {
		return nil, err
	}
	summaries := make([]MultiKeyChannelSummary, 0, len(channels))
	for _, ch := range channels {
		if !ch.ChannelInfo.IsMultiKey {
			continue
		}
		summaries = append(summaries, MultiKeyChannelSummary{
			Id:           ch.Id,
			Name:         ch.Name,
			Type:         ch.Type,
			Status:       ch.Status,
			Group:        ch.Group,
			MultiKeySize: ch.ChannelInfo.MultiKeySize,
		})
	}
	return summaries, nil
}
