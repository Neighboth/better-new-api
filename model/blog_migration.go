package model

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/QuantumNous/new-api/common"
)

// Early blog builds created blog_comments and blog_reactions with a
// `blog_id NOT NULL` column — and, depending on the build, with foreign keys
// or unique indexes whose semantics differ from the canonical schema.
// AutoMigrate can add missing columns but can never relax a NOT NULL
// constraint or drop stray constraints, so inserts/updates/deletes kept
// failing on databases created by those builds. When such a legacy table is
// detected, rebuild it from the canonical schema and copy the rows over.
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

func normalizeLegacyBlogTable(table string, dst any) error {
	if !DB.Migrator().HasTable(table) {
		return nil // AutoMigrate will create the canonical table
	}
	columnTypes, err := DB.Migrator().ColumnTypes(table)
	if err != nil {
		return err
	}
	isLegacy := false
	for _, columnType := range columnTypes {
		if columnType.Name() != "blog_id" {
			continue
		}
		if nullable, ok := columnType.Nullable(); ok && !nullable {
			isLegacy = true
		}
	}
	if !isLegacy {
		return nil
	}

	common.SysLog("rebuilding legacy blog table " + table + " (blog_id NOT NULL)")
	columnNames := make([]string, 0, len(columnTypes))
	for _, columnType := range columnTypes {
		columnNames = append(columnNames, columnType.Name())
	}

	legacyName := table + "_legacy_" + fmt.Sprint(time.Now().Unix())
	quote := "`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		quote = "\""
	}
	quoteIdent := func(name string) string { return quote + name + quote }
	quotedColumns := make([]string, 0, len(columnNames))
	for _, name := range columnNames {
		quotedColumns = append(quotedColumns, quoteIdent(name))
	}
	columnList := strings.Join(quotedColumns, ", ")

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
		if err := tx.Exec(fmt.Sprintf(
			"INSERT INTO %s (%s) SELECT %s FROM %s",
			quoteIdent(table), columnList, columnList, quoteIdent(legacyName),
		)).Error; err != nil {
			return err
		}
		return tx.Migrator().DropTable(legacyName)
	})
}

// dropTableIndexes removes every index still attached to a (renamed) table.
func dropTableIndexes(tx *gorm.DB, table string) error {
	switch {
	case common.UsingMainDatabase(common.DatabaseTypeSQLite):
		rows, err := tx.Raw(`PRAGMA index_list(` + quoteSQLiteIdent(table) + `)`).Rows()
		if err != nil {
			return err
		}
		defer rows.Close()
		var names []string
		for rows.Next() {
			var seq int
			var name string
			var unique, origin, partial any
			if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
				return err
			}
			if name != "" {
				names = append(names, name)
			}
		}
		for _, name := range names {
			if err := tx.Exec(`DROP INDEX IF EXISTS ` + quoteSQLiteIdent(name)).Error; err != nil {
				return err
			}
		}
		return nil
	case common.UsingMainDatabase(common.DatabaseTypeMySQL):
		rows, err := tx.Raw(`SHOW INDEX FROM ` + "`" + table + "`").Rows()
		if err != nil {
			return err
		}
		defer rows.Close()
		columns, err := rows.Columns()
		if err != nil {
			return err
		}
		keyNameIdx := -1
		for i, column := range columns {
			if column == "Key_name" {
				keyNameIdx = i
			}
		}
		seen := map[string]bool{}
		for rows.Next() {
			values := make([]any, len(columns))
			scanArgs := make([]any, len(columns))
			for i := range values {
				scanArgs[i] = &values[i]
			}
			if err := rows.Scan(scanArgs...); err != nil {
				return err
			}
			if keyNameIdx < 0 {
				continue
			}
			nameBytes, ok := values[keyNameIdx].([]byte)
			if !ok {
				continue
			}
			name := string(nameBytes)
			if name == "PRIMARY" || seen[name] {
				continue
			}
			seen[name] = true
			if err := tx.Exec(fmt.Sprintf("ALTER TABLE `%s` DROP INDEX `%s`", table, name)).Error; err != nil {
				return err
			}
		}
		return nil
	default: // PostgreSQL keeps index ownership across renames as well
		return tx.Exec(fmt.Sprintf(
			`DO $$ DECLARE idx RECORD; BEGIN
				FOR idx IN SELECT indexname FROM pg_indexes WHERE tablename = '%s' AND schemaname = current_schema()
				LOOP EXECUTE format('DROP INDEX IF EXISTS %%I', idx.indexname); END LOOP;
			END $$;`, table)).Error
	}
}

func quoteSQLiteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}
