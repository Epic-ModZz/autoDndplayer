package immutable

func ImmutableFeatSchema() string {
	return `CREATE TABLE IF NOT EXISTS feats (
        id           INTEGER PRIMARY KEY AUTOINCREMENT,
        name         TEXT UNIQUE NOT NULL,
        source       TEXT,
        prerequisite TEXT,
        origin       INTEGER DEFAULT 0,  -- 1 if can be taken at character creation (2024 rules)
        repeatable   INTEGER DEFAULT 0,  -- 1 if can be taken multiple times e.g. Fighting Style
        description  TEXT NOT NULL
    )`
}
