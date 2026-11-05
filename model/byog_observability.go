package model

// The observability of the BYOG network for the administration.
//
// The queries of this file answer the operational questions of the panel:
// how healthy is the node network right now, which crashes did the workers
// report and which nodes are stuck with a failed model transfer. Every query
// runs with plain aggregates so the same code answers on SQLite, MySQL and
// PostgreSQL.

// byogNodeStaleSeconds is the age of the heartbeat after which a node that
// claims to be online is counted as stale, it matches the online window of
// the site wide statistics.
const byogNodeStaleSeconds int64 = 180

// ByogNodeHealth is the operational state of the whole node network.
type ByogNodeHealth struct {
	Total       int64 `json:"total"`
	Online      int64 `json:"online"`
	Stale       int64 `json:"stale"`
	Offline     int64 `json:"offline"`
	Paused      int64 `json:"paused"`
	Banned      int64 `json:"banned"`
	Downloading int64 `json:"downloading"`
	Errors      int64 `json:"errors"`
}

// GetByogNodeHealth counts the nodes of every operational state. A node is
// online when its heartbeat is fresh, stale when it claims to be online but
// the heartbeat died, and it carries an error when the worker reported a
// failed model state or a failed transfer.
func GetByogNodeHealth(now int64) (*ByogNodeHealth, error) {
	health := &ByogNodeHealth{}
	fresh := now - byogNodeStaleSeconds

	steps := []struct {
		target *int64
		where  string
		args   []any
	}{
		{target: &health.Total, where: ""},
		{target: &health.Online, where: "last_heartbeat_at >= ?", args: []any{fresh}},
		{target: &health.Stale, where: "status = ? AND last_heartbeat_at < ?", args: []any{"online", fresh}},
		{target: &health.Offline, where: "status = ?", args: []any{"offline"}},
		{target: &health.Paused, where: "status = ?", args: []any{"paused"}},
		{target: &health.Banned, where: "status = ?", args: []any{"banned"}},
		{target: &health.Downloading, where: "model_status = ? OR download_status = ?", args: []any{"downloading", "downloading"}},
		{target: &health.Errors, where: "model_status = ? OR download_status = ?", args: []any{"error", "error"}},
	}
	for _, step := range steps {
		tx := DB.Model(&ByogNode{})
		if step.where != "" {
			tx = tx.Where(step.where, step.args...)
		}
		if err := tx.Count(step.target).Error; err != nil {
			return nil, err
		}
	}
	return health, nil
}

// CountByogCrashReports counts every crash report of the installation.
func CountByogCrashReports() (int64, error) {
	var count int64
	err := DB.Model(&ByogCrashReport{}).Count(&count).Error
	return count, err
}

// CountByogCrashReportsSince counts the crash reports that arrived at or
// after the moment.
func CountByogCrashReportsSince(since int64) (int64, error) {
	var count int64
	err := DB.Model(&ByogCrashReport{}).Where("created_at >= ?", since).Count(&count).Error
	return count, err
}

// ListByogCrashReportsPage returns one page of the crash reports ordered by
// the arrival, the empty filters select every report. The total is the count
// of the reports that match the filters so the panel can page over them.
func ListByogCrashReportsPage(nodeId string, errorCode string, page int, pageSize int) ([]*ByogCrashReport, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	countTx := DB.Model(&ByogCrashReport{})
	findTx := DB.Model(&ByogCrashReport{})
	if nodeId != "" {
		countTx = countTx.Where("node_id = ?", nodeId)
		findTx = findTx.Where("node_id = ?", nodeId)
	}
	if errorCode != "" {
		countTx = countTx.Where("error_code = ?", errorCode)
		findTx = findTx.Where("error_code = ?", errorCode)
	}

	var total int64
	if err := countTx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []*ByogCrashReport
	err := findTx.
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error
	return rows, total, err
}

// TopByogCrashErrorCodes returns the error codes of the crash reports
// ordered by their count, the panel shows the loudest failures first.
func TopByogCrashErrorCodes(limit int) ([]SiteStatsNameCount, error) {
	var rows []SiteStatsNameCount
	err := DB.Table("byog_crash_reports").
		Select("error_code AS name, COUNT(*) AS count").
		Where("error_code <> ''").
		Group("error_code").
		Order("count DESC").
		Limit(rankingLimit(limit)).
		Scan(&rows).Error
	return rows, err
}

// ListByogNodesWithErrors returns the nodes whose model state or transfer
// state is an error, the panel shows them as the download problems of the
// network.
func ListByogNodesWithErrors(limit int) ([]*ByogNode, error) {
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var rows []*ByogNode
	err := DB.Where("model_status = ? OR download_status = ?", "error", "error").
		Order("updated_at DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// GetByogNodeDeviceNames returns the device names of the node ids, the crash
// reports show the name of the device that crashed.
func GetByogNodeDeviceNames(nodeIds []string) (map[string]string, error) {
	result := make(map[string]string, len(nodeIds))
	if len(nodeIds) == 0 {
		return result, nil
	}
	var rows []struct {
		NodeId     string `gorm:"column:node_id"`
		DeviceName string `gorm:"column:device_name"`
	}
	err := DB.Table("byog_nodes").
		Select("node_id, device_name").
		Where("node_id IN ?", nodeIds).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.NodeId] = row.DeviceName
	}
	return result, nil
}
