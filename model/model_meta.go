package model

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	NameRuleExact = iota
	NameRulePrefix
	NameRuleContains
	NameRuleSuffix
)

type BoundChannel struct {
	Name string `json:"name"`
	Type int    `json:"type"`
}

type Model struct {
	Id            int            `json:"id"`
	ModelName     string         `json:"model_name" gorm:"size:128;not null;uniqueIndex:uk_model_name_delete_at,priority:1"`
	DisplayName   string         `json:"display_name,omitempty" gorm:"type:varchar(128)"`
	Tagline       string         `json:"tagline,omitempty" gorm:"type:varchar(255)"`
	PropertyBoxes string         `json:"property_boxes,omitempty" gorm:"type:text"`
	Description   string         `json:"description,omitempty" gorm:"type:text"`
	Icon         string         `json:"icon,omitempty" gorm:"type:varchar(128)"`
	Tags         string         `json:"tags,omitempty" gorm:"type:varchar(255)"`
	VendorID     int            `json:"vendor_id,omitempty" gorm:"index"`
	Endpoints    string         `json:"endpoints,omitempty" gorm:"type:text"`
	Status       int            `json:"status" gorm:"default:1"`
	SyncOfficial int            `json:"sync_official" gorm:"default:1"`
	CreatedTime  int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime  int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:uk_model_name_delete_at,priority:2"`

	BoundChannels []BoundChannel `json:"bound_channels,omitempty" gorm:"-"`
	EnableGroups  []string       `json:"enable_groups,omitempty" gorm:"-"`
	QuotaTypes    []int          `json:"quota_types,omitempty" gorm:"-"`
	NameRule      int            `json:"name_rule" gorm:"default:0"`
	ServerlessPriority int       `json:"serverless_priority" gorm:"default:0"` // 0: Channels first, 1: Serverless first

	MatchedModels []string `json:"matched_models,omitempty" gorm:"-"`
	MatchedCount  int      `json:"matched_count,omitempty" gorm:"-"`
}

func (mi *Model) Insert() error {
	now := common.GetTimestamp()
	mi.CreatedTime = now
	mi.UpdatedTime = now

	// 保存原始值（因为 Create 后可能被 GORM 的 default 标签覆盖为 1）
	originalStatus := mi.Status
	originalSyncOfficial := mi.SyncOfficial

	// 先创建记录（GORM 会对零值字段应用默认值）
	if err := DB.Create(mi).Error; err != nil {
		return err
	}

	// 使用保存的原始值进行更新，确保零值能正确保存
	return DB.Model(&Model{}).Where("id = ?", mi.Id).Updates(map[string]interface{}{
		"status":        originalStatus,
		"sync_official": originalSyncOfficial,
	}).Error
}

func IsModelNameDuplicated(id int, name string) (bool, error) {
	if name == "" {
		return false, nil
	}
	var cnt int64
	err := DB.Model(&Model{}).Where("model_name = ? AND id <> ?", name, id).Count(&cnt).Error
	return cnt > 0, err
}

func (mi *Model) Update() error {
	mi.UpdatedTime = common.GetTimestamp()
	// 使用 Select 强制更新所有字段，包括零值
	return DB.Model(&Model{}).Where("id = ?", mi.Id).
		Select("model_name", "display_name", "tagline", "property_boxes", "description", "icon", "tags", "vendor_id", "endpoints", "status", "sync_official", "name_rule", "serverless_priority", "updated_time").
		Updates(mi).Error
}

func (mi *Model) Delete() error {
	return DB.Delete(mi).Error
}

func GetVendorModelCounts() (map[int64]int64, error) {
	m := make(map[int64]int64)
	if DB == nil {
		return m, nil
	}
	var stats []struct {
		VendorID int64
		Count    int64
	}
	if err := DB.Model(&Model{}).
		Select("vendor_id, count(*) as count").
		Group("vendor_id").
		Scan(&stats).Error; err != nil {
		return m, nil
	}
	for _, s := range stats {
		m[s.VendorID] = s.Count
	}
	return m, nil
}

func GetAllModels(offset int, limit int) ([]*Model, error) {
	models, _, err := SearchModels("", "", "", "", offset, limit)
	return models, err
}

func GetBoundChannelsByModelsMap(modelNames []string) (map[string][]BoundChannel, error) {
	result := make(map[string][]BoundChannel)
	if len(modelNames) == 0 {
		return result, nil
	}
	type row struct {
		Model string
		Name  string
		Type  int
	}
	var rows []row
	err := DB.Table("channels").
		Select("abilities.model as model, channels.name as name, channels.type as type").
		Joins("JOIN abilities ON abilities.channel_id = channels.id").
		Where("abilities.model IN ? AND abilities.enabled = ?", modelNames, true).
		Distinct().
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.Model] = append(result[r.Model], BoundChannel{Name: r.Name, Type: r.Type})
	}
	return result, nil
}

func normalizeLookupValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func GetPreferredModelOwnerChannelTypes(modelNames []string, groups []string) (map[string]int, error) {
	result := make(map[string]int)
	modelNames = normalizeLookupValues(modelNames)
	if len(modelNames) == 0 {
		return result, nil
	}

	type row struct {
		Model       string
		ChannelType int
	}
	var rows []row

	query := DB.Table("abilities").
		Select("abilities.model as model, channels.type as channel_type").
		Joins("JOIN channels ON abilities.channel_id = channels.id").
		Where("abilities.model IN ? AND abilities.enabled = ? AND channels.status = ?", modelNames, true, common.ChannelStatusEnabled).
		Order("COALESCE(abilities.priority, 0) DESC").
		Order("abilities.weight DESC").
		Order("abilities.channel_id ASC")

	groups = normalizeLookupValues(groups)
	if len(groups) > 0 {
		query = query.Where("abilities."+commonGroupCol+" IN ?", groups)
	}

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, r := range rows {
		if _, ok := result[r.Model]; ok {
			continue
		}
		result[r.Model] = r.ChannelType
	}
	return result, nil
}

var (
	modelColumnsEnsured bool
	ensureModelLock     sync.Mutex
)

func ensureTableColumn(tableName, colName, standardDDL, sqliteDDL string) {
	if DB == nil {
		return
	}
	ddl := standardDDL
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) && sqliteDDL != "" {
		ddl = sqliteDDL
	}

	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		sql := fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN IF NOT EXISTS "%s" %s`, tableName, colName, ddl)
		_ = DB.Exec(sql).Error
		return
	}

	if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		var cnt int64
		_ = DB.Raw("SELECT count(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?", tableName, colName).Scan(&cnt).Error
		if cnt == 0 {
			sql := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` %s", tableName, colName, ddl)
			_ = DB.Exec(sql).Error
		}
		return
	}

	// SQLite
	if DB.Migrator().HasTable(tableName) && !DB.Migrator().HasColumn(tableName, colName) {
		sql := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` %s", tableName, colName, ddl)
		_ = DB.Exec(sql).Error
	}
}

func EnsureModelAndVendorColumns() {
	if DB == nil {
		return
	}
	ensureModelLock.Lock()
	defer ensureModelLock.Unlock()
	if modelColumnsEnsured {
		return
	}

	_ = DB.AutoMigrate(&Model{}, &Vendor{})

	// Explicitly ensure columns in models table with cross-DB SQL
	ensureTableColumn("models", "display_name", "varchar(128) DEFAULT ''", "text DEFAULT ''")
	ensureTableColumn("models", "tagline", "varchar(255) DEFAULT ''", "text DEFAULT ''")
	ensureTableColumn("models", "property_boxes", "text", "text")
	ensureTableColumn("models", "endpoints", "text", "text")
	ensureTableColumn("models", "name_rule", "integer DEFAULT 0", "integer DEFAULT 0")
	ensureTableColumn("models", "serverless_priority", "integer DEFAULT 0", "integer DEFAULT 0")
	ensureTableColumn("models", "sync_official", "integer DEFAULT 1", "integer DEFAULT 1")
	ensureTableColumn("models", "status", "integer DEFAULT 1", "integer DEFAULT 1")
	ensureTableColumn("models", "vendor_id", "integer DEFAULT 0", "integer DEFAULT 0")
	ensureTableColumn("models", "icon", "varchar(128) DEFAULT ''", "text DEFAULT ''")
	ensureTableColumn("models", "tags", "varchar(255) DEFAULT ''", "text DEFAULT ''")
	ensureTableColumn("models", "description", "text", "text")
	ensureTableColumn("models", "created_time", "bigint DEFAULT 0", "bigint DEFAULT 0")
	ensureTableColumn("models", "updated_time", "bigint DEFAULT 0", "bigint DEFAULT 0")
	if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		ensureTableColumn("models", "deleted_at", "datetime(3) NULL", "")
	} else if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		ensureTableColumn("models", "deleted_at", "timestamp with time zone NULL", "")
	} else {
		ensureTableColumn("models", "deleted_at", "datetime NULL", "")
	}

	// Explicitly ensure columns in vendors table
	ensureTableColumn("vendors", "keywords", "text", "text")
	ensureTableColumn("vendors", "is_builtin", "boolean DEFAULT false", "numeric DEFAULT 0")
	ensureTableColumn("vendors", "status", "integer DEFAULT 1", "integer DEFAULT 1")
	ensureTableColumn("vendors", "icon", "varchar(255) DEFAULT ''", "text DEFAULT ''")
	ensureTableColumn("vendors", "description", "text", "text")
	ensureTableColumn("vendors", "created_time", "bigint DEFAULT 0", "bigint DEFAULT 0")
	ensureTableColumn("vendors", "updated_time", "bigint DEFAULT 0", "bigint DEFAULT 0")
	if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		ensureTableColumn("vendors", "deleted_at", "datetime(3) NULL", "")
	} else if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		ensureTableColumn("vendors", "deleted_at", "timestamp with time zone NULL", "")
	} else {
		ensureTableColumn("vendors", "deleted_at", "datetime NULL", "")
	}

	modelColumnsEnsured = true
}

func SearchModels(keyword string, vendor string, status string, syncOfficial string, offset int, limit int) ([]*Model, int64, error) {
	if DB == nil {
		return []*Model{}, 0, nil
	}

	EnsureModelAndVendorColumns()

	var total int64
	var models []*Model

	// 1. Count query without SELECT column projection (MySQL-safe)
	countDB := DB.Model(&Model{})
	if keyword != "" {
		like := "%" + keyword + "%"
		countDB = countDB.Where("models.model_name LIKE ? OR models.display_name LIKE ? OR models.description LIKE ? OR models.tags LIKE ?", like, like, like, like)
	}
	if vendor != "" {
		if vid, err := strconv.Atoi(vendor); err == nil {
			countDB = countDB.Where("models.vendor_id = ?", vid)
		} else {
			countDB = countDB.Joins("JOIN vendors ON vendors.id = models.vendor_id").Where("vendors.name LIKE ?", "%"+vendor+"%")
		}
	}
	if statusValue, ok := parseModelStatusFilter(status); ok {
		countDB = countDB.Where("models.status = ?", statusValue)
	}
	if syncValue, ok := parseModelSyncFilter(syncOfficial); ok {
		countDB = countDB.Where("models.sync_official = ?", syncValue)
	}
	_ = countDB.Count(&total).Error

	// 2. Data query
	dataDB := DB.Model(&Model{})
	if keyword != "" {
		like := "%" + keyword + "%"
		dataDB = dataDB.Where("models.model_name LIKE ? OR models.display_name LIKE ? OR models.description LIKE ? OR models.tags LIKE ?", like, like, like, like)
	}
	if vendor != "" {
		if vid, err := strconv.Atoi(vendor); err == nil {
			dataDB = dataDB.Where("models.vendor_id = ?", vid)
		} else {
			dataDB = dataDB.Joins("JOIN vendors ON vendors.id = models.vendor_id").Where("vendors.name LIKE ?", "%"+vendor+"%").Select("models.*")
		}
	}
	if statusValue, ok := parseModelStatusFilter(status); ok {
		dataDB = dataDB.Where("models.status = ?", statusValue)
	}
	if syncValue, ok := parseModelSyncFilter(syncOfficial); ok {
		dataDB = dataDB.Where("models.sync_official = ?", syncValue)
	}

	if err := dataDB.Order("models.id DESC").Offset(offset).Limit(limit).Find(&models).Error; err != nil {
		// Fallback without table qualifier
		if err2 := DB.Model(&Model{}).Order("id DESC").Offset(offset).Limit(limit).Find(&models).Error; err2 != nil {
			return []*Model{}, total, err
		}
	}

	if models == nil {
		models = []*Model{}
	}

	return models, total, nil
}

// parseModelStatusFilter maps UI/API status values to the models.status column.
// Returns ok=false when no status filter should be applied.
func parseModelStatusFilter(status string) (value int, ok bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "all":
		return 0, false
	case "enabled", "1":
		return 1, true
	case "disabled", "0":
		return 0, true
	default:
		n, err := strconv.Atoi(status)
		if err != nil {
			return 0, false
		}
		return n, true
	}
}

// parseModelSyncFilter maps UI/API sync values to the models.sync_official column.
// Returns ok=false when no sync filter should be applied.
func parseModelSyncFilter(syncOfficial string) (value int, ok bool) {
	switch strings.ToLower(strings.TrimSpace(syncOfficial)) {
	case "", "all":
		return 0, false
	case "yes", "1":
		return 1, true
	case "no", "0":
		return 0, true
	default:
		n, err := strconv.Atoi(syncOfficial)
		if err != nil {
			return 0, false
		}
		return n, true
	}
}
