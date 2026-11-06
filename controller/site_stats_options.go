package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

type siteStatsOption struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Statistics permissions expose only names and IDs, not user or channel secrets.
func SearchSiteStatsUsers(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	if len(keyword) > 128 {
		keyword = keyword[:128]
	}
	query := model.DB.Model(&model.User{}).Select("id, username AS name")
	if keyword != "" {
		like := "%" + keyword + "%"
		id, _ := strconv.Atoi(keyword)
		if id > 0 {
			query = query.Where("id = ? OR username LIKE ? OR display_name LIKE ?", id, like, like)
		} else {
			query = query.Where("username LIKE ? OR display_name LIKE ?", like, like)
		}
		query = query.Clauses(clause.OrderBy{Expression: clause.Expr{SQL: "CASE WHEN LOWER(username) = LOWER(?) THEN 0 WHEN id = ? THEN 1 ELSE 2 END, id DESC", Vars: []interface{}{keyword, id}}})
	} else {
		query = query.Order("id DESC")
	}
	rows := make([]siteStatsOption, 0)
	if err := query.Limit(50).Scan(&rows).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

func SearchSiteStatsChannels(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	if len(keyword) > 128 {
		keyword = keyword[:128]
	}
	query := model.DB.Model(&model.Channel{}).Select("id, name")
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	rows := make([]siteStatsOption, 0)
	if err := query.Order("id DESC").Limit(50).Scan(&rows).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}
