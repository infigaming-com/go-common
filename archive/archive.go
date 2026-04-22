// Package archive provides schema-drift resilient helpers for the common
// pattern of copying rows from a business table into an archive table keyed
// by an `archived_at` timestamp column.
//
// Background: Meepo services archive operator data with SQL like
//
//	INSERT INTO schema.archived_foo
//	SELECT src.*, ? AS archived_at FROM schema.foo src WHERE ...
//
// which depends on `schema.foo` and `schema.archived_foo` having identical
// column positions plus `archived_at` appended to the archive. When a business
// table gains a column without a matching ALTER on the archive table, the
// INSERT crashes with "more expressions than target columns" or — worse —
// silently writes a value into the wrong typed column. A single column drift
// (e.g. users.created_by_user_id added 2026-04-02 without syncing
// archived_users) took down prod archive on 2026-04-21.
//
// This package builds INSERT SQL using an **explicit column list** computed
// from the intersection of columns present on both sides (minus `archived_at`),
// queried from information_schema at call time. The archive is then resilient
// in both directions:
//
//   - source has a column archive doesn't  → silently dropped, warning log
//     from SourceOnlyColumns() surfaces which columns are in drift
//   - archive has a column source doesn't  → archive uses DEFAULT, no crash
//
// Use ColumnIntersection + QuoteIdents / PrefixIdents to assemble the SQL in
// whatever batching pattern your service prefers (ctid, PK id, etc.).
package archive

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// ArchivedAtColumn is the well-known timestamp column all archive tables carry.
// It is excluded from ColumnIntersection and SourceOnlyColumns.
const ArchivedAtColumn = "archived_at"

// ColumnIntersection returns columns present in BOTH src and tgt tables,
// excluding ArchivedAtColumn. Result order matches src's ordinal_position.
//
// srcFQ / tgtFQ are fully-qualified `schema.table` names. A bare table name
// (no `.`) is treated as being in the `public` schema.
//
// Returns an error if either table cannot be found in information_schema or
// if the intersection is empty (which would indicate catastrophic schema
// mismatch where no INSERT is possible).
func ColumnIntersection(db *gorm.DB, srcFQ, tgtFQ string) ([]string, error) {
	srcSchema, srcTable := SplitSchemaTable(srcFQ)
	tgtSchema, tgtTable := SplitSchemaTable(tgtFQ)

	var cols []string
	err := db.Raw(`
		SELECT s.column_name
		FROM information_schema.columns s
		JOIN information_schema.columns t
		  ON t.column_name = s.column_name
		 AND t.table_schema = ?
		 AND t.table_name = ?
		WHERE s.table_schema = ?
		  AND s.table_name = ?
		  AND s.column_name <> ?
		ORDER BY s.ordinal_position
	`, tgtSchema, tgtTable, srcSchema, srcTable, ArchivedAtColumn).Scan(&cols).Error
	if err != nil {
		return nil, fmt.Errorf("archive: resolve column intersection for %s -> %s: %w", srcFQ, tgtFQ, err)
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("archive: no overlapping columns between %s and %s", srcFQ, tgtFQ)
	}
	return cols, nil
}

// SourceOnlyColumns returns columns present in src but missing from tgt
// (excluding ArchivedAtColumn). Callers typically log these at WARN level so
// forgotten archive-table migrations surface instead of silently dropping data.
func SourceOnlyColumns(db *gorm.DB, srcFQ, tgtFQ string) ([]string, error) {
	srcSchema, srcTable := SplitSchemaTable(srcFQ)
	tgtSchema, tgtTable := SplitSchemaTable(tgtFQ)

	var cols []string
	err := db.Raw(`
		SELECT s.column_name
		FROM information_schema.columns s
		WHERE s.table_schema = ?
		  AND s.table_name = ?
		  AND s.column_name <> ?
		  AND NOT EXISTS (
		    SELECT 1 FROM information_schema.columns t
		    WHERE t.table_schema = ?
		      AND t.table_name = ?
		      AND t.column_name = s.column_name
		  )
		ORDER BY s.ordinal_position
	`, srcSchema, srcTable, ArchivedAtColumn, tgtSchema, tgtTable).Scan(&cols).Error
	if err != nil {
		return nil, fmt.Errorf("archive: resolve source-only columns for %s -> %s: %w", srcFQ, tgtFQ, err)
	}
	return cols, nil
}

// SplitSchemaTable parses a fully-qualified `schema.table` name into its parts.
// A bare `table` (no dot) is returned with schema = "public", matching Postgres'
// default search_path. The last dot is the split point, so names containing
// dots in the table portion (unusual but legal when quoted) are not supported.
func SplitSchemaTable(fq string) (schema, table string) {
	if i := strings.LastIndexByte(fq, '.'); i >= 0 {
		return fq[:i], fq[i+1:]
	}
	return "public", fq
}

// QuoteIdents wraps each identifier in double quotes and joins them with commas,
// suitable for inclusion in an INSERT column list. Embedded quotes are doubled
// per the SQL standard.
//
// Example: QuoteIdents([]string{"id", "name"}) → `"id","name"`
func QuoteIdents(names []string) string {
	if len(names) == 0 {
		return ""
	}
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = `"` + strings.ReplaceAll(n, `"`, `""`) + `"`
	}
	return strings.Join(parts, ",")
}

// PrefixIdents is like QuoteIdents but each identifier is prefixed with
// `alias.`, suitable for the SELECT list of a join-/CTE-based query.
//
// Example: PrefixIdents("src", []string{"id", "name"}) → `src."id",src."name"`
func PrefixIdents(alias string, names []string) string {
	if len(names) == 0 {
		return ""
	}
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = alias + `."` + strings.ReplaceAll(n, `"`, `""`) + `"`
	}
	return strings.Join(parts, ",")
}
