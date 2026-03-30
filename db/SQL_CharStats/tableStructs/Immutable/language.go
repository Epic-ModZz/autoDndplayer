package immutable

func ImmutableLanguageSchema() string {
	return `CREATE TABLE IF NOT EXISTS languages (
        id              INTEGER PRIMARY KEY AUTOINCREMENT,
        name            TEXT UNIQUE NOT NULL,   -- e.g. "Common", "Elvish", "Infernal"
        source          TEXT,
        language_type   TEXT NOT NULL,          -- standard, exotic, secret
        typical_speakers TEXT,                  -- e.g. "Humans, Halflings", "Devils, Tieflings"
        script          TEXT,                   -- e.g. "Common", "Elvish", "Infernal" -- the writing system used
        description     TEXT
    )`
}
