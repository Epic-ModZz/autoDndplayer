package immutable

func ImmutableFightingStyleSchema() string {
	return `CREATE TABLE IF NOT EXISTS fighting_styles (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		name         TEXT UNIQUE NOT NULL,
		source       TEXT,
		description  TEXT NOT NULL,
		available_to TEXT    -- comma-separated classes e.g. "Fighter, Paladin, Ranger"
	)`
}
