package model

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"gorm.io/gorm"
)

// ProxyPoolProxy is an admin-registered outbound proxy that can be bound to
// individual keys of a multi-key channel. The relay resolves the proxy per
// request through service.EffectiveRelayProxyURL.
type ProxyPoolProxy struct {
	Id             int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name           string `json:"name" gorm:"type:varchar(191);not null;default:''"`
	URL            string `json:"url" gorm:"type:text;not null"`
	ProxyType      string `json:"proxy_type" gorm:"type:varchar(32);default:'standard'"`
	Enabled        bool   `json:"enabled" gorm:"not null;default:true"`
	LastTestStatus string `json:"last_test_status" gorm:"type:varchar(32);default:''"`
	LastTestError  string `json:"last_test_error" gorm:"type:text"`
	LastTestedAt   int64  `json:"last_tested_at" gorm:"not null;default:0"`
	LatencyMs      int64  `json:"latency_ms" gorm:"default:0"`
	EgressIP       string `json:"egress_ip" gorm:"type:varchar(64);default:''"`
	CountryCode    string `json:"country_code" gorm:"type:varchar(8);default:''"`
	CountryName    string `json:"country_name" gorm:"type:varchar(64);default:''"`
	City           string `json:"city" gorm:"type:varchar(64);default:''"`
	ISP            string `json:"isp" gorm:"type:varchar(128);default:''"`
	AssignedKeys   int    `json:"assigned_keys" gorm:"-"`
	IsBackup       bool   `json:"is_backup" gorm:"-"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

// ProxyPoolDailyUsage tracks daily traffic and request counts through a proxy.
type ProxyPoolDailyUsage struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	ProxyId   int    `json:"proxy_id" gorm:"not null;index;uniqueIndex:idx_proxy_date"`
	Date      string `json:"date" gorm:"type:varchar(10);not null;uniqueIndex:idx_proxy_date"` // YYYY-MM-DD
	Bytes     int64  `json:"bytes" gorm:"not null;default:0"`
	Requests  int64  `json:"requests" gorm:"not null;default:0"`
	UpdatedAt int64  `json:"updated_at"`
}

type ProxyDailyMBPoint struct {
	Date     string  `json:"date"`
	MB       float64 `json:"mb"`
	Requests int64   `json:"requests"`
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

type ProxyPoolListResult struct {
	Items        []ProxyPoolProxy `json:"items"`
	Total        int64            `json:"total"`
	Page         int              `json:"page"`
	PageSize     int              `json:"page_size"`
	TotalPages   int              `json:"total_pages"`
	EnabledCount int64            `json:"enabled_count"`
}

func ListProxyPoolProxiesPaged(page, pageSize int, keyword string, status string) (*ProxyPoolListResult, error) {
	if DB == nil {
		return &ProxyPoolListResult{Items: []ProxyPoolProxy{}, Total: 0, Page: page, PageSize: pageSize}, nil
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := DB.Model(&ProxyPoolProxy{})
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR url LIKE ? OR egress_ip LIKE ?", like, like, like)
	}
	if status == "enabled" {
		query = query.Where("enabled = ?", true)
	} else if status == "disabled" {
		query = query.Where("enabled = ?", false)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var enabledCount int64
	_ = DB.Model(&ProxyPoolProxy{}).Where("enabled = ?", true).Count(&enabledCount).Error

	offset := (page - 1) * pageSize
	var proxies []ProxyPoolProxy
	err := query.Order("id ASC").Limit(pageSize).Offset(offset).Find(&proxies).Error
	if err != nil {
		return nil, err
	}

	if len(proxies) > 0 {
		ids := make([]int, len(proxies))
		for i, p := range proxies {
			ids[i] = p.Id
		}
		var counts []struct {
			ProxyId int
			Count   int
		}
		_ = DB.Model(&ProxyPoolAssignment{}).
			Select("proxy_id, count(*) as count").
			Where("proxy_id IN ?", ids).
			Group("proxy_id").
			Scan(&counts).Error

		countMap := make(map[int]int, len(counts))
		for _, c := range counts {
			countMap[c.ProxyId] = c.Count
		}
		for i := range proxies {
			proxies[i].AssignedKeys = countMap[proxies[i].Id]
			if proxies[i].Enabled && proxies[i].AssignedKeys == 0 {
				proxies[i].IsBackup = true
			}
		}
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize != 0 {
		totalPages++
	}

	return &ProxyPoolListResult{
		Items:        proxies,
		Total:        total,
		Page:         page,
		PageSize:     pageSize,
		TotalPages:   totalPages,
		EnabledCount: enabledCount,
	}, nil
}

func BatchCreateProxyPoolProxies(proxies []ProxyPoolProxy) (int, error) {
	if DB == nil || len(proxies) == 0 {
		return 0, nil
	}
	now := common.GetTimestamp()
	for i := range proxies {
		proxies[i].CreatedAt = now
		proxies[i].UpdatedAt = now
	}
	err := DB.CreateInBatches(proxies, 500).Error
	if err != nil {
		return 0, err
	}
	return len(proxies), nil
}

func ListProxyPoolProxies() ([]ProxyPoolProxy, error) {
	if DB == nil {
		return []ProxyPoolProxy{}, nil
	}
	var proxies []ProxyPoolProxy
	err := DB.Order("id ASC").Find(&proxies).Error
	if err != nil {
		return nil, err
	}
	var counts []struct {
		ProxyId int
		Count   int
	}
	_ = DB.Model(&ProxyPoolAssignment{}).
		Select("proxy_id, count(*) as count").
		Group("proxy_id").
		Scan(&counts).Error
	countMap := make(map[int]int, len(counts))
	for _, c := range counts {
		countMap[c.ProxyId] = c.Count
	}
	for i := range proxies {
		proxies[i].AssignedKeys = countMap[proxies[i].Id]
		if proxies[i].Enabled && proxies[i].AssignedKeys == 0 {
			proxies[i].IsBackup = true
		}
	}
	return proxies, nil
}

// ProxyChannelAssignmentDetail is one row of the channel detail view: the
// bound proxy joined with its health state.
type ProxyChannelAssignmentDetail struct {
	KeyIndex       int    `json:"key_index"`
	ProxyID        int    `json:"proxy_id"`
	ProxyName      string `json:"proxy_name"`
	ProxyURL       string `json:"proxy_url"`
	ProxyEnabled   bool   `json:"proxy_enabled"`
	EgressIP       string `json:"egress_ip"`
	LatencyMs      int64  `json:"latency_ms"`
	LastTestStatus string `json:"last_test_status"`
	LastTestError  string `json:"last_test_error"`
	LastTestedAt   int64  `json:"last_tested_at"`
	CountryCode    string `json:"country_code"`
	CountryName    string `json:"country_name"`
}

// ListProxyPoolChannelAssignmentsDetail returns one page of the joined rows
// for the channel detail popup. Proxies can number in the tens of thousands
// for a single channel, so the endpoint paginates.
func ListProxyPoolChannelAssignmentsDetail(channelID int, limit int, offset int) ([]ProxyChannelAssignmentDetail, error) {
	out := []ProxyChannelAssignmentDetail{}
	if DB == nil {
		return out, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	err := DB.Table("proxy_pool_assignments as a").
		Select("a.key_index, p.id as proxy_id, p.name as proxy_name, p.url as proxy_url, p.enabled as proxy_enabled, p.egress_ip, p.latency_ms, p.last_test_status, p.last_test_error, p.last_tested_at, p.country_code, p.country_name").
		Joins("join proxy_pool_proxies as p on p.id = a.proxy_id").
		Where("a.channel_id = ?", channelID).
		Order("a.key_index ASC").
		Limit(limit).
		Offset(offset).
		Scan(&out).Error
	return out, err
}

// CountProxyPoolChannelAssignments returns the number of assignments bound to
// the channel (used to paginate the channel detail popup).
func CountProxyPoolChannelAssignments(channelID int) (int64, error) {
	if DB == nil {
		return 0, nil
	}
	var count int64
	err := DB.Table("proxy_pool_assignments").Where("channel_id = ?", channelID).Count(&count).Error
	return count, err
}

// ListProxyPoolAssignmentsForChannel returns the raw assignments of one
// channel (used by rebalance/assign flows).
func ListProxyPoolAssignmentsForChannel(channelID int) ([]ProxyPoolAssignment, error) {
	out := []ProxyPoolAssignment{}
	if DB == nil {
		return out, nil
	}
	err := DB.Where("channel_id = ?", channelID).Order("key_index ASC").Find(&out).Error
	return out, err
}

// ListProxyPoolProxiesForAutoTest returns the N proxies with the stalest test
// results, so each sweep covers a rotating slice of the pool instead of
// hammering every entry on every run (important with tens of thousands).
func ListProxyPoolProxiesForAutoTest(limit int) ([]ProxyPoolProxy, error) {
	if DB == nil {
		return []ProxyPoolProxy{}, nil
	}
	if limit <= 0 {
		limit = 1000
	}
	var proxies []ProxyPoolProxy
	err := DB.Order("last_tested_at ASC, id ASC").Limit(limit).Find(&proxies).Error
	return proxies, err
}

func UnassignProxyPoolProxy(id int) error {
	return DB.Where("proxy_id = ?", id).Delete(&ProxyPoolAssignment{}).Error
}

func RecordProxyPoolUsage(proxyId int, bytes int64) {
	if DB == nil || proxyId <= 0 || bytes <= 0 {
		return
	}
	date := time.Now().Format("2006-01-02")
	now := time.Now().Unix()
	var usage ProxyPoolDailyUsage
	err := DB.Where("proxy_id = ? AND date = ?", proxyId, date).First(&usage).Error
	if err == nil {
		_ = DB.Model(&usage).Updates(map[string]interface{}{
			"bytes":      gorm.Expr("bytes + ?", bytes),
			"requests":   gorm.Expr("requests + 1"),
			"updated_at": now,
		}).Error
	} else {
		usage = ProxyPoolDailyUsage{
			ProxyId:   proxyId,
			Date:      date,
			Bytes:     bytes,
			Requests:  1,
			UpdatedAt: now,
		}
		_ = DB.Create(&usage).Error
	}
}

func GetProxyPoolDailyUsageHistory(proxyId int, days int) []ProxyDailyMBPoint {
	if days <= 0 {
		days = 30
	}
	now := time.Now()
	points := make([]ProxyDailyMBPoint, days)
	dateMap := make(map[string]int, days)
	for i := days - 1; i >= 0; i-- {
		d := now.AddDate(0, 0, -i).Format("2006-01-02")
		idx := days - 1 - i
		points[idx] = ProxyDailyMBPoint{Date: d, MB: 0, Requests: 0}
		dateMap[d] = idx
	}

	if DB == nil {
		return points
	}

	var rows []ProxyPoolDailyUsage
	query := DB.Model(&ProxyPoolDailyUsage{})
	if proxyId > 0 {
		query = query.Where("proxy_id = ?", proxyId)
	}
	startDate := now.AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	_ = query.Where("date >= ?", startDate).Find(&rows).Error

	for _, r := range rows {
		if idx, ok := dateMap[r.Date]; ok {
			points[idx].MB += float64(r.Bytes) / (1024 * 1024)
			points[idx].Requests += r.Requests
		}
	}
	return points
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
		if len(mapping) == 0 {
			return nil
		}
		assignments := make([]ProxyPoolAssignment, 0, len(mapping))
		for keyIndex, proxyId := range mapping {
			assignments = append(assignments, ProxyPoolAssignment{
				ChannelId: channelId,
				KeyIndex:  keyIndex,
				ProxyId:   proxyId,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}
		// Bulk insert in batches: with 65k proxies a per-row insert exceeded
		// the server-side prepared-statement budget (ER_MAX_PREPARED_STMT_COUNT_REACHED).
		return tx.CreateInBatches(assignments, 500).Error
	})
}

func DeleteProxyPoolAssignments(channelId int) error {
	return DB.Where("channel_id = ?", channelId).Delete(&ProxyPoolAssignment{}).Error
}

// ProxyPoolAssignmentURLMap loads the (channel_id, key_index) -> proxy URL
// mapping for every assignment whose proxy is enabled. Disabled proxies are
// skipped, leaving their keys on the channel's own default path (or direct).
func ProxyPoolAssignmentURLMap() (map[int]map[int]string, error) {
	if DB == nil {
		return make(map[int]map[int]string), nil
	}
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
		isPoolEligible := ch.ChannelInfo.IsMultiKey || ch.Type == constant.ChannelTypeOpenCode || ch.Type == constant.ChannelTypeKiloCodeAnon || ch.Type == constant.ChannelTypeOHVCloudAnon
		if !isPoolEligible {
			continue
		}
		keyCount := ch.ChannelInfo.MultiKeySize
		if keyCount <= 0 {
			keyCount = 1
		}
		summaries = append(summaries, MultiKeyChannelSummary{
			Id:           ch.Id,
			Name:         ch.Name,
			Type:         ch.Type,
			Status:       ch.Status,
			Group:        ch.Group,
			MultiKeySize: keyCount,
		})
	}
	return summaries, nil
}
