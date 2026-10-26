package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestModelMetaQueryAndMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	origDB := DB
	DB = db
	defer func() { DB = origDB }()

	// First simulate an older schema where Model table exists without display_name, tagline, property_boxes
	type OldModel struct {
		Id           int    `json:"id" gorm:"primaryKey"`
		ModelName    string `json:"model_name"`
		Description  string `json:"description"`
		Icon         string `json:"icon"`
		Tags         string `json:"tags"`
		VendorID     int    `json:"vendor_id"`
		Endpoints    string `json:"endpoints"`
		Status       int    `json:"status"`
		SyncOfficial int    `json:"sync_official"`
	}
	require.NoError(t, db.Table("models").AutoMigrate(&OldModel{}))

	// Now run AutoMigrate or migrateNewColumns
	_ = DB.AutoMigrate(&Model{})
	migrateNewColumns()

	// Now try SearchModels
	models, total, err := SearchModels("", "", "", "", 0, 10)
	require.NoError(t, err, "SearchModels should not fail")
	_ = models
	_ = total
}
