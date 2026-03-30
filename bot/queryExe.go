package bot

import (
	dbpkg "PCL/db/SQL_CharStats"
	"fmt"
	"log"
	"regexp"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var sqlCodeBlockRegex = regexp.MustCompile("(?s)```sql\\s*(.*?)```")

// ExtractSQLQueries parses all ```sql ... ``` blocks out of an LLM response.
func ExtractSQLQueries(llmOutput string) []string {
	matches := sqlCodeBlockRegex.FindAllStringSubmatch(llmOutput, -1)
	var queries []string
	for _, match := range matches {
		if len(match) > 1 {
			q := strings.TrimSpace(match[1])
			if q != "" {
				queries = append(queries, q)
			}
		}
	}
	return queries
}

// QueryResult holds the result of a single SQL query.
type QueryResult struct {
	Query   string
	Columns []string
	Rows    []map[string]string
	Error   string
}

// RunQueries executes each extracted SELECT query and returns structured results.
// Any non-SELECT statement is blocked and logged.
func RunQueries(llmOutput string) ([]QueryResult, error) {
	if dbpkg.DB == nil {
		return nil, fmt.Errorf("DB is not initialized")
	}

	queries := ExtractSQLQueries(llmOutput)
	if len(queries) == 0 {
		return nil, fmt.Errorf("no SQL queries found in LLM output")
	}

	var results []QueryResult

	for _, query := range queries {
		result := QueryResult{Query: query}

		if err := validateReadStatement(query); err != nil {
			result.Error = fmt.Sprintf("blocked: %s", err)
			log.Printf("RunQueries: blocked statement — %s\n  stmt: %.120s", err, query)
			results = append(results, result)
			continue
		}

		rows, err := dbpkg.DB.Query(query)
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}

		cols, err := rows.Columns()
		if err != nil {
			result.Error = err.Error()
			rows.Close()
			results = append(results, result)
			continue
		}
		result.Columns = cols

		for rows.Next() {
			values := make([]interface{}, len(cols))
			valuePtrs := make([]interface{}, len(cols))
			for i := range values {
				valuePtrs[i] = &values[i]
			}
			if err := rows.Scan(valuePtrs...); err != nil {
				result.Error = err.Error()
				break
			}
			row := make(map[string]string)
			for i, col := range cols {
				if values[i] == nil {
					row[col] = "NULL"
				} else {
					row[col] = fmt.Sprintf("%v", values[i])
				}
			}
			result.Rows = append(result.Rows, row)
		}
		rows.Close()
		results = append(results, result)
	}

	return results, nil
}

// FormatResultsForLLM converts query results into a readable string for the LLM.
func FormatResultsForLLM(results []QueryResult) string {
	var sb strings.Builder
	for _, result := range results {
		sb.WriteString(fmt.Sprintf("Query: %s\n", result.Query))
		if result.Error != "" {
			sb.WriteString(fmt.Sprintf("Error: %s\n\n", result.Error))
			continue
		}
		if len(result.Rows) == 0 {
			sb.WriteString("Result: No rows found\n\n")
			continue
		}
		sb.WriteString(strings.Join(result.Columns, " | "))
		sb.WriteString("\n")
		for _, row := range result.Rows {
			values := make([]string, len(result.Columns))
			for i, col := range result.Columns {
				values[i] = row[col]
			}
			sb.WriteString(strings.Join(values, " | "))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

var mutationRegex = regexp.MustCompile("(?s)```sql\\s*(.*?)```")

// allowedMutationKeywords are the only verbs the memory writer may execute.
var allowedMutationKeywords = map[string]bool{
	"INSERT": true,
	"UPDATE": true,
	"DELETE": true,
}

// immutableTables are seeded once at startup and must never be modified at runtime.
var immutableTables = map[string]bool{
	"races": true, "race_features": true, "subraces": true, "subrace_features": true,
	"classes": true, "subclasses": true, "class_features": true, "subclass_features": true,
	"class_spellcasting_progression": true, "single_class_spell_slots": true,
	"multiclass_spell_slots": true, "pact_magic_slots": true,
	"feats": true, "feat_features": true, "boons": true, "boon_features": true,
	"eldritch_invocations": true, "fighting_styles": true,
	"backgrounds": true, "background_features": true,
	"conditions": true, "condition_effects": true,
	"spells": true, "spell_components": true,
	"weapons": true, "weapon_masteries": true, "armor": true,
	"mundane_items": true, "magic_items": true, "magic_item_abilities": true,
	"potions": true, "item_interactions": true,
	"proficiencies": true, "languages": true,
	"monsters": true, "monster_actions": true, "monster_traits": true,
	"monster_legendary": true, "monster_spellcasting": true, "monster_spell_list": true,
	"connection_levels": true, "stronghold_tiers": true, "facility_types": true,
	"facility_benefits": true, "facility_upgrades": true, "facility_discounts": true,
}

// MutationResult holds the outcome of a single write statement.
type MutationResult struct {
	Statement    string
	RowsAffected int64
	Error        string
}

// RunMutations extracts, validates, and executes all SQL write statements from
// an LLM response. Validation runs before any transaction is opened so no
// partial writes are committed if a statement is blocked.
//
// Validation rules (applied to every statement individually):
//  1. Only INSERT, UPDATE, DELETE are permitted verbs
//  2. UPDATE and DELETE must have a WHERE clause
//  3. UPDATE and DELETE WHERE must not be trivially unbounded (1=1, TRUE, etc.)
//  4. Immutable reference tables may never be written to
func RunMutations(llmOutput string) ([]MutationResult, error) {
	if dbpkg.DB == nil {
		return nil, fmt.Errorf("DB is not initialized")
	}

	matches := mutationRegex.FindAllStringSubmatch(llmOutput, -1)
	if len(matches) == 0 {
		return nil, nil
	}

	type pendingStmt struct {
		stmt    string
		blocked string // non-empty → rejected, holds reason
	}

	var pending []pendingStmt
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		for _, stmt := range splitSQLStatements(match[1]) {
			if err := validateMutationStatement(stmt); err != nil {
				log.Printf("RunMutations: blocked — %s\n  stmt: %.120s", err, stmt)
				pending = append(pending, pendingStmt{stmt: stmt, blocked: err.Error()})
			} else {
				pending = append(pending, pendingStmt{stmt: stmt})
			}
		}
	}

	if len(pending) == 0 {
		return nil, nil
	}

	var results []MutationResult
	var valid []string

	for _, p := range pending {
		if p.blocked != "" {
			results = append(results, MutationResult{
				Statement: p.stmt,
				Error:     "blocked: " + p.blocked,
			})
		} else {
			valid = append(valid, p.stmt)
		}
	}

	if len(valid) == 0 {
		return results, nil
	}

	// Execute all valid statements in a single transaction.
	tx, err := dbpkg.DB.Begin()
	if err != nil {
		return results, fmt.Errorf("RunMutations: begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, stmt := range valid {
		res, err := tx.Exec(stmt)
		mr := MutationResult{Statement: stmt}
		if err != nil {
			mr.Error = err.Error()
			log.Printf("RunMutations: exec error — %s\n  stmt: %.120s", err, stmt)
		} else {
			mr.RowsAffected, _ = res.RowsAffected()
		}
		results = append(results, mr)
	}

	if err := tx.Commit(); err != nil {
		return results, fmt.Errorf("RunMutations: commit: %w", err)
	}

	return results, nil
}

// validateReadStatement ensures only SELECT reaches the DB in the read pipeline.
func validateReadStatement(stmt string) error {
	// Strip leading comment lines before checking the verb
	var firstNonComment string
	for _, line := range strings.Split(stmt, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "--") {
			firstNonComment = trimmed
			break
		}
	}

	fields := strings.Fields(firstNonComment)
	if len(fields) == 0 {
		return fmt.Errorf("empty statement")
	}
	if strings.ToUpper(fields[0]) != "SELECT" {
		return fmt.Errorf("only SELECT is permitted in the read pipeline, got %q", fields[0])
	}
	return nil
}

// reTrivialWhere matches WHERE clauses that unconditionally match all rows.
// Catches: WHERE 1=1, WHERE 1, WHERE TRUE, WHERE (1=1).
var reTrivialWhere = regexp.MustCompile(`WHERE\s+\(?(\s*(1\s*=\s*1|TRUE|1)\s*)\)?(\s|$)`)

// validateMutationStatement applies all safety rules to a single write statement.
func validateMutationStatement(stmt string) error {
	var firstNonComment string
	for _, line := range strings.Split(stmt, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "--") {
			firstNonComment = trimmed
			break
		}
	}

	fields := strings.Fields(firstNonComment)
	if len(fields) == 0 {
		return fmt.Errorf("empty statement")
	}

	verb := strings.ToUpper(fields[0])

	// Rule 1: permitted verbs only.
	if !allowedMutationKeywords[verb] {
		return fmt.Errorf("%q is not a permitted mutation verb", verb)
	}

	upper := strings.ToUpper(stmt)

	// Rule 2: UPDATE and DELETE must have a WHERE clause.
	if (verb == "UPDATE" || verb == "DELETE") && !strings.Contains(upper, "WHERE") {
		return fmt.Errorf("%s without WHERE clause would affect all rows", verb)
	}

	// Rule 3: WHERE must not be trivially unbounded.
	if (verb == "UPDATE" || verb == "DELETE") && reTrivialWhere.MatchString(upper) {
		return fmt.Errorf("%s has a trivially unbounded WHERE clause", verb)
	}

	// Rule 4: immutable tables may not be written to.
	if table := extractTargetTable(verb, fields); table != "" {
		if immutableTables[strings.ToLower(table)] {
			return fmt.Errorf("table %q is immutable and may not be modified at runtime", table)
		}
	}

	return nil
}

// extractTargetTable returns the table name a write statement targets.
func extractTargetTable(verb string, fields []string) string {
	switch verb {
	case "INSERT":
		// INSERT [OR IGNORE|OR REPLACE] INTO <table>
		for i, f := range fields {
			if strings.ToUpper(f) == "INTO" && i+1 < len(fields) {
				return strings.Split(fields[i+1], "(")[0]
			}
		}
	case "UPDATE":
		// UPDATE <table> SET ...
		if len(fields) >= 2 {
			return fields[1]
		}
	case "DELETE":
		// DELETE FROM <table> WHERE ...
		for i, f := range fields {
			if strings.ToUpper(f) == "FROM" && i+1 < len(fields) {
				return fields[i+1]
			}
		}
	}
	return ""
}

// splitSQLStatements splits a SQL block on semicolons, discarding empty fragments.
func splitSQLStatements(block string) []string {
	parts := strings.Split(block, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
