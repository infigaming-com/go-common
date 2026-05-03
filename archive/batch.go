package archive

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// DefaultBatchSize is the default LIMIT used by BatchArchive / BatchDelete.
// Sized so a single batch typically commits in under a second on AlloyDB.
const DefaultBatchSize = 1000

// interBatchSleep throttles between batches to reduce DB pressure during
// long archive runs. Short enough that 1M-row archives still complete in
// minutes, long enough that interactive query latency stays in budget.
const interBatchSleep = 100 * time.Millisecond

// WarnFunc is an optional callback for surfacing schema-drift warnings to
// the caller's logger. Use a closure that wraps your service's log helper,
// e.g.
//
//	warn := func(f string, a ...any) { lg.Warnf(f, a...) }
//
// Pass nil to disable warnings.
type WarnFunc func(format string, args ...any)

// PKSelectAndTuple returns the SELECT list and the row-tuple form for a set
// of primary-key columns, both quoted.
//
// Examples:
//
//	PKSelectAndTuple([]string{"id"})              -> (`"id"`, `("id")`)
//	PKSelectAndTuple([]string{"id", "user_id"})   -> (`"id","user_id"`, `("id","user_id")`)
//
// The names are concatenated raw into SQL — pass trusted, hardcoded
// identifiers, never user input.
func PKSelectAndTuple(pkCols []string) (list, tuple string) {
	list = QuoteIdents(pkCols)
	return list, "(" + list + ")"
}

// BatchArchive copies rows from sourceTable to archiveTable filtered by
// filterCol = filterVal, then deletes them from sourceTable. Repeats in
// batches of DefaultBatchSize until no more matching rows.
//
// SQL pattern (writable CTE, single statement per batch):
//
//	WITH batch AS (
//	    SELECT pk1,pk2,... FROM src WHERE filter = $1 LIMIT $2
//	),
//	inserted AS (
//	    INSERT INTO tgt (cols..., archived_at)
//	    SELECT cols..., $3 FROM src
//	    WHERE filter = $1 AND (pk1,pk2,...) IN (SELECT pk1,pk2,... FROM batch)
//	    ON CONFLICT DO NOTHING
//	    RETURNING 1
//	)
//	DELETE FROM src WHERE filter = $1 AND (pk1,pk2,...) IN (SELECT pk1,pk2,... FROM batch)
//
// Why writable CTE: PG evaluates all CTEs in a single statement against the
// same snapshot, so INSERT and DELETE see the exact same `batch` PKs. Without
// this, two independent LIMIT queries can pick different rows under parallel
// scan — leading to rare DELETE-without-archive (data loss) or
// archive-without-DELETE (self-heals on next pass via ON CONFLICT).
//
// Why PK IN (...) not ctid: ctid is per-partition. ctid IN (...) on a
// partitioned parent table silently matches rows in OTHER partitions that
// share the same physical position, which corrupts unrelated operators'
// data. PG requires partition keys to be part of any PK on a partitioned
// table, so the composite PK is globally unique across partitions.
//
// Why double filterCol = ? guard: defense in depth. Even if the inner PK
// subquery somehow returned an unrelated row (PK collision across
// partitions on a misconfigured schema, etc.), the outer filter prevents
// touching another caller's rows.
//
// Column drift behavior:
//   - source has a column the archive doesn't: silently dropped, warn callback fired
//   - archive has a column the source doesn't: archive uses DEFAULT, no crash
//
// pkCols must uniquely identify a row in sourceTable across all partitions —
// typically the table's primary key.
//
// filterCol filters by an int64-typed column (e.g. operator_id). Single
// int64 covers all current callers.
//
// archivedAt is written into the archive's archived_at column. Pass
// time.Now().UnixMilli() for normal archive runs.
//
// warn is invoked once per call if the source has columns the archive
// doesn't (a sign that the archive table needs an ALTER). Pass nil to
// disable.
//
// Returns total rows deleted from sourceTable. Each row is also present in
// archiveTable (either freshly inserted or pre-existing via ON CONFLICT).
func BatchArchive(
	ctx context.Context,
	db *gorm.DB,
	warn WarnFunc,
	sourceTable, archiveTable, filterCol string,
	pkCols []string,
	filterVal, archivedAt int64,
) (int64, error) {
	cols, err := ColumnIntersection(db.WithContext(ctx), sourceTable, archiveTable)
	if err != nil {
		return 0, err
	}
	if missing, err := SourceOnlyColumns(db.WithContext(ctx), sourceTable, archiveTable); err == nil && len(missing) > 0 && warn != nil {
		warn("archive column drift: %s has columns not in %s (data will be dropped): %v",
			sourceTable, archiveTable, missing)
	}
	quotedCols := QuoteIdents(cols)
	srcCols := PrefixIdents("src", cols)
	pkList, pkTuple := PKSelectAndTuple(pkCols)

	sql := fmt.Sprintf(
		"WITH batch AS (SELECT %s FROM %s WHERE %s = ? LIMIT ?), "+
			"inserted AS ("+
			"INSERT INTO %s (%s, archived_at) "+
			"SELECT %s, ? FROM %s src "+
			"WHERE src.%s = ? AND %s IN (SELECT %s FROM batch) "+
			"ON CONFLICT DO NOTHING "+
			"RETURNING 1"+
			") "+
			"DELETE FROM %s WHERE %s = ? AND %s IN (SELECT %s FROM batch)",
		pkList, sourceTable, filterCol,
		archiveTable, quotedCols,
		srcCols, sourceTable,
		filterCol, pkTuple, pkList,
		sourceTable, filterCol, pkTuple, pkList,
	)

	var total int64
	for {
		// Args: batch (filterVal, batchSize), insert (archivedAt, filterVal), delete (filterVal).
		result := db.WithContext(ctx).Exec(sql, filterVal, DefaultBatchSize, archivedAt, filterVal, filterVal)
		if result.Error != nil {
			return total, fmt.Errorf("batch archive %s -> %s (archived so far: %d): %w",
				sourceTable, archiveTable, total, result.Error)
		}
		total += result.RowsAffected
		if result.RowsAffected == 0 {
			break
		}
		// Pause between batches; honour cancellation.
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		case <-time.After(interBatchSleep):
		}
	}
	return total, nil
}

// BatchDelete batch-deletes rows from `table` where filterCol = filterVal.
// Used for tables that do not need archival (config, logs, etc).
//
// SQL pattern:
//
//	DELETE FROM tbl WHERE filterCol = ? AND (pk) IN (
//	    SELECT pk FROM tbl WHERE filterCol = ? LIMIT ?
//	)
//
// Same constraints as BatchArchive: pkCols must be the partition-aware
// primary key, and the outer filterCol = ? guard prevents cross-row
// matches even if PKs were ever ambiguous.
//
// Returns total rows deleted.
func BatchDelete(
	ctx context.Context,
	db *gorm.DB,
	table, filterCol string,
	pkCols []string,
	filterVal int64,
) (int64, error) {
	pkList, pkTuple := PKSelectAndTuple(pkCols)
	sql := fmt.Sprintf(
		"DELETE FROM %s WHERE %s = ? AND %s IN (SELECT %s FROM %s WHERE %s = ? LIMIT ?)",
		table, filterCol, pkTuple, pkList, table, filterCol,
	)

	var total int64
	for {
		result := db.WithContext(ctx).Exec(sql, filterVal, filterVal, DefaultBatchSize)
		if result.Error != nil {
			return total, fmt.Errorf("batch delete %s (deleted so far: %d): %w", table, total, result.Error)
		}
		total += result.RowsAffected
		if result.RowsAffected == 0 {
			break
		}
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		case <-time.After(interBatchSleep):
		}
	}
	return total, nil
}
