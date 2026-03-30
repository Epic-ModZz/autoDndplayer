package db

import (
	"fmt"
	"strings"
)

// UpsertRow inserts or updates a row in any mutable table.
// tableName is the name of the table to update.
// data is a map of column names to values.
// conflictColumn is the column to check for conflicts on (usually "id" or a unique key like "character_id").
//
// Example:
//
//	UpsertRow("character_hp", map[string]interface{}{
//	    "character_id": 1,
//	    "hp_current":   45,
//	    "hp_max":       60,
//	}, "character_id")
func UpsertRow(tableName string, data map[string]interface{}, conflictColumn string) error {
	if len(data) == 0 {
		return fmt.Errorf("no data provided for table %q", tableName)
	}

	// Build ordered slices of columns and placeholders so the query is deterministic
	columns := make([]string, 0, len(data))
	placeholders := make([]string, 0, len(data))
	values := make([]interface{}, 0, len(data))

	for col, val := range data {
		columns = append(columns, col)
		placeholders = append(placeholders, "?")
		values = append(values, val)
	}

	// Build the ON CONFLICT update clause — update every column except the conflict column
	updateClauses := make([]string, 0, len(columns))
	for _, col := range columns {
		if col != conflictColumn {
			updateClauses = append(updateClauses, fmt.Sprintf("%s = excluded.%s", col, col))
		}
	}

	query := fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s) ON CONFLICT(%s) DO UPDATE SET %s`,
		tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
		conflictColumn,
		strings.Join(updateClauses, ", "),
	)

	_, err := DB.Exec(query, values...)
	if err != nil {
		return fmt.Errorf("UpsertRow failed on table %q: %w", tableName, err)
	}

	return nil
}

// DeleteRow removes a row from any mutable table by a column value.
// Useful for removing conditions, inventory items, active quests, etc.
//
// Example:
//
//	DeleteRow("character_conditions", "id", 42)
func DeleteRow(tableName string, whereColumn string, whereValue interface{}) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE %s = ?`, tableName, whereColumn)
	_, err := DB.Exec(query, whereValue)
	if err != nil {
		return fmt.Errorf("DeleteRow failed on table %q: %w", tableName, err)
	}
	return nil
}

// GetRows fetches all rows from a table matching a column value,
// returning each row as a map of column name to value.
// Useful for the AI to read current state before deciding what to update.
//
// Example:
//
//	rows, err := GetRows("character_conditions", "character_id", 1)
func GetRows(tableName string, whereColumn string, whereValue interface{}) ([]map[string]interface{}, error) {
	query := fmt.Sprintf(`SELECT * FROM %s WHERE %s = ?`, tableName, whereColumn)

	rows, err := DB.Query(query, whereValue)
	if err != nil {
		return nil, fmt.Errorf("GetRows failed on table %q: %w", tableName, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns for table %q: %w", tableName, err)
	}

	var results []map[string]interface{}

	for rows.Next() {
		// Create a slice of interface{} to hold each column value
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row in table %q: %w", tableName, err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	return results, nil
}
