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

	// Now run migrateNewColumns without AutoMigrate(&Model{})
	migrateNewColumns()

	// Check if deleted_at was added
	hasDeletedAt := DB.Migrator().HasColumn(&Model{}, "deleted_at")
	t.Logf("Has deleted_at: %v", hasDeletedAt)

	// Now try SearchModels
	models, total, err := SearchModels("", "", "", "", 0, 10)
	require.NoError(t, err, "SearchModels should not fail")
	_ = models
	_ = total

	// Verify GetVendorModelCounts
	counts, err := GetVendorModelCounts()
	require.NoError(t, err)
	require.NotNil(t, counts)

	// Test SearchModels with filters
	testModel := &Model{
		ModelName:     "anthropic/claude-3-5-sonnet",
		DisplayName:   "Claude 3.5 Sonnet",
		Tagline:       "Flagship model • Reasoning • Tool calling",
		PropertyBoxes: `[{"label":"Context","value":"200K"}]`,
		Status:        1,
		SyncOfficial:  1,
	}
	require.NoError(t, testModel.Insert())

	found, foundTotal, err := SearchModels("claude", "", "1", "1", 0, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), foundTotal)
	require.Len(t, found, 1)
	require.Equal(t, "Claude 3.5 Sonnet", found[0].DisplayName)
	require.Equal(t, "Flagship model • Reasoning • Tool calling", found[0].Tagline)
}
