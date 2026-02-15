package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// ScanRow is a helper that scans a single row into dest values.
func ScanRow(row *sql.Row, dest ...interface{}) error {
	return row.Scan(dest...)
}

// ScanRows is a helper that scans all rows using a callback.
func ScanRows(rows *sql.Rows, scanFn func(*sql.Rows) error) error {
	defer rows.Close()
	for rows.Next() {
		if err := scanFn(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// Exists checks if a row exists matching the given condition.
func Exists(db *sql.DB, table, condition string, args ...interface{}) (bool, error) {
	query := fmt.Sprintf("SELECT 1 FROM %s WHERE %s LIMIT 1", table, condition)
	var n int
	err := db.QueryRow(query, args...).Scan(&n)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// BuildUpdateSQL builds a SET clause from a map of column->value pairs.
// Returns the SET clause string and a slice of args.
func BuildUpdateSQL(table string, id string, fields map[string]interface{}) (string, []interface{}) {
	setClauses := make([]string, 0, len(fields))
	args := make([]interface{}, 0, len(fields)+1)

	for col, val := range fields {
		setClauses = append(setClauses, fmt.Sprintf("%s = ?", col))
		args = append(args, val)
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = ?", table, strings.Join(setClauses, ", "))
	args = append(args, id)

	return query, args
}
