package model

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/QuantumNous/new-api/common"
)

// Early blog builds created blog_comments and blog_reactions with a
// `blog_id NOT NULL` column and columns the canonical schema later dropped
// (avatar_url, type). AutoMigrate can add missing columns but can never
// relax a NOT NULL constraint or drop stray constraints/indexes, so
// inserts/updates/deletes kept failing on databases created by those builds.
// When such a legacy table is detected, rebuild it from the canonical schema
// and copy the rows over. A crashed rebuild leaves a "<table>_legacy_*"
// table behind; finishCrashedBlogRebuild completes that copy on the next
// start so no data is stranded.
func normalizeLegacyBlogTables() error {
	legacyTables := map[string]any{
		"blog_comments":  &BlogComment{},
		"blog_reactions": &BlogReaction{},
	}
	for table, dst := range legacyTables {
		if err := normalizeLegacyBlogTable(table, dst); err != nil {
			return fmt.Errorf("normalize legacy table %s: %w", table, err)
		}
	}
	return nil
}

// blogColumnFallbacks maps a canonical column to the legacy column that held
// the same data in early builds, used when the legacy table lacks the new
// column (or holds the real value in the old one).
var blogColumnFallbacks = map[string]map[string]string{
	"blog_comments":  {"avatar": "avatar_url"},
	"blog_reactions": {"target_type": "type"},
}

func normalizeLegacyBlogTable(table string, dst any) error {
	if err := finishCrashedBlogRebuild(table, dst); err != nil {
		return err
	}
	if !DB.Migrator().HasTable(table) {
		return nil // AutoMigrate created the canonical table
	}
	columnTypes, err := DB.Migrator().ColumnTypes(table)
	if err != nil {
		return err
	}
	for _, columnType := range columnTypes {
		if columnType.Name() != "blog_id" {
			continue
		}
		if nullable, ok := columnType.Nullable(); ok && !nullable {
			common.SysLog("rebuilding legacy blog table " + table + " (blog_id NOT NULL)")
			return rebuildBlogTable(table, dst)
		}
	}
	return nil
}

// finishCrashedBlogRebuild copies rows out of leftover "<table>_legacy_*"
// tables from an interrupted rebuild, then drops them.
func finishCrashedBlogRebuild(table string, dst any) error {
	legacyNames, err := findLegacyBlogTables(table)
	if err != nil || len(legacyNames) == 0 {
		return err
	}
	if !DB.Migrator().HasTable(table) {
		if err := DB.AutoMigrate(dst); err != nil {
			return err
		}
	}
	for _, legacyName := range legacyNames {
		common.SysLog("finishing interrupted blog table rebuild from " + legacyName)
		if err := copyBlogTableRows(DB, legacyName, table); err != nil {
			return err
		}
		if err := DB.Migrator().DropTable(legacyName); err != nil {
			return err
		}
	}
	return nil
}

func findLegacyBlogTables(table string) ([]string, error) {
	var names []string
	switch {
	case common.UsingMainDatabase(common.DatabaseTypeSQLite):
		err := DB.Raw(
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name LIKE ? ESCAPE '\'`,
			table+`\_legacy\_%`,
		).Scan(&names).Error
		return names, err
	case common.UsingMainDatabase(common.DatabaseTypeMySQL):
		err := DB.Raw(
			`SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name LIKE ?`,
			table+`_legacy_%`,
		).Scan(&names).Error
		return names, err
	default:
		err := DB.Raw(
			`SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema() AND table_name LIKE ?`,
			table+`_legacy_%`,
		).Scan(&names).Error
		return names, err
	}
}

func rebuildBlogTable(table string, dst any) error {
	legacyName := table + "_legacy_" + fmt.Sprint(time.Now().Unix())
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Migrator().RenameTable(table, legacyName); err != nil {
			return err
		}
		// Renamed tables keep their indexes on SQLite/Postgres; drop them so
		// the canonical table can recreate indexes with the same names.
		if err := dropTableIndexes(tx, legacyName); err != nil {
			return err
		}
		// Create the canonical table (with its indexes) before copying rows.
		if err := tx.AutoMigrate(dst); err != nil {
			return err
		}
		if err := copyBlogTableRows(tx, legacyName, table); err != nil {
			return err
		}
		return tx.Migrator().DropTable(legacyName)
	})
}

// copyBlogTableRows copies rows between two shapes of the same table, using
// only the columns present on both sides plus the legacy column fallbacks.
func copyBlogTableRows(tx *gorm.DB, from string, to string) error {
	fromColumns, err := tableColumnNames(tx, from)
	if err != nil {
		return err
	}
	toColumns, err := tableColumnNames(tx, to)
	if err != nil {
		return err
	}
	fromSet := make(map[string]bool, len(fromColumns))
	for _, name := range fromColumns {
		fromSet[name] = true
	}

	fallbacks := blogColumnFallbacks[to]
	insertColumns := make([]string, 0, len(toColumns))
	selectExprs := make([]string, 0, len(toColumns))
	for _, column := range toColumns {
		expr := ""
		fallback, hasFallback := fallbacks[column]
		fallbackExists := hasFallback && fromSet[fallback]
		switch {
		case fromSet[column] && fallbackExists:
			expr = fmt.Sprintf("COALESCE(NULLIF(%s, ''), %s)", quoteBlogIdent(column), quoteBlogIdent(fallback))
		case fromSet[column]:
			expr = quoteBlogIdent(column)
		case fallbackExists:
			expr = quoteBlogIdent(fallback)
		default:
			continue // canonical-only column: default/NULL
		}
		insertColumns = append(insertColumns, quoteBlogIdent(column))
		selectExprs = append(selectExprs, expr)
	}
	if len(insertColumns) == 0 {
		return nil
	}

	insert := "INSERT INTO"
	switch {
	case common.UsingMainDatabase(common.DatabaseTypeSQLite):
		insert = "INSERT OR IGNORE INTO"
	case common.UsingMainDatabase(common.DatabaseTypeMySQL):
		insert = "INSERT IGNORE INTO"
	}
	statement := fmt.Sprintf(
		"%s %s (%s) SELECT %s FROM %s",
		insert,
		quoteBlogIdent(to),
		strings.Join(insertColumns, ", "),
		strings.Join(selectExprs, ", "),
		quoteBlogIdent(from),
	)
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		statement += " ON CONFLICT DO NOTHING"
	}
	return tx.Exec(statement).Error
}

func tableColumnNames(tx *gorm.DB, table string) ([]string, error) {
	columnTypes, err := tx.Migrator().ColumnTypes(table)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(columnTypes))
	for _, columnType := range columnTypes {
		names = append(names, columnType.Name())
	}
	return names, nil
}

func quoteBlogIdent(name string) string {
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
	}
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// dropTableIndexes removes every index still attached to a (renamed) table.
func dropTableIndexes(tx *gorm.DB, table string) error {
	switch {
	case common.UsingMainDatabase(common.DatabaseTypeSQLite):
		var names []string
		rows, err := tx.Raw(`SELECT name FROM sqlite_master WHERE type = 'index' AND tbl_name = ? AND name NOT LIKE 'sqlite_%'`, table).Rows()
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return err
			}
			names = append(names, name)
		}
		for _, name := range names {
			if err := tx.Exec(`DROP INDEX IF EXISTS ` + quoteBlogIdent(name)).Error; err != nil {
				return err
			}
		}
		return nil
	case common.UsingMainDatabase(common.DatabaseTypeMySQL):
		var names []string
		if err := tx.Raw(
			`SELECT DISTINCT index_name FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name <> 'PRIMARY'`,
			table,
		).Scan(&names).Error; err != nil {
			return err
		}
		for _, name := range names {
			if err := tx.Exec(fmt.Sprintf("ALTER TABLE %s DROP INDEX %s", quoteBlogIdent(table), quoteBlogIdent(name))).Error; err != nil {
				return err
			}
		}
		return nil
	default: // PostgreSQL keeps index ownership across renames as well
		var names []string
		if err := tx.Raw(
			`SELECT indexname FROM pg_indexes WHERE schemaname = current_schema() AND tablename = ?`,
			table,
		).Scan(&names).Error; err != nil {
			return err
		}
		for _, name := range names {
			if err := tx.Exec(`DROP INDEX IF EXISTS ` + quoteBlogIdent(name)).Error; err != nil {
				return err
			}
		}
		return nil
	}
}
