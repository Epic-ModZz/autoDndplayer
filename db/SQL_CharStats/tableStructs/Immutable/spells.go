package immutable

func ImmutableSpellSchema() string {
	return `CREATE TABLE IF NOT EXISTS spells (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		name          TEXT UNIQUE NOT NULL,
		source        TEXT,
		level         INTEGER,
		school        TEXT,
		casting_time  TEXT,
		range         TEXT,
		components    TEXT,         -- V, S, M (quick reference)
		duration      TEXT,
		concentration INTEGER DEFAULT 0,
		ritual        INTEGER DEFAULT 0,
		description   TEXT NOT NULL,
		higher_levels TEXT,         -- "At Higher Levels" section
		classes       TEXT          -- comma-separated class names
	)`
}

func ImmutableSpellComponentSchema() string {
	return `CREATE TABLE IF NOT EXISTS spell_components (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		spell_id    INTEGER REFERENCES spells(id),
		type        TEXT NOT NULL,  -- V, S, or M
		description TEXT NOT NULL,  -- e.g. "a crystal worth at least 500gp"
		gold_cost   INTEGER,        -- NULL if no minimum cost
		consumed    INTEGER DEFAULT 0,
		notes       TEXT
	)`
}
