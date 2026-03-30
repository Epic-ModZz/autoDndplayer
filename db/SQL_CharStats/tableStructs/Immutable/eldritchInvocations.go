package immutable

func ImmutableEldritchInvocationSchema() string {
	return `CREATE TABLE IF NOT EXISTS eldritch_invocations (
        id                  INTEGER PRIMARY KEY AUTOINCREMENT,
        name                TEXT UNIQUE NOT NULL,
        source              TEXT,
        prerequisite_level  INTEGER,     -- NULL if no level requirement
        prerequisite_spell  TEXT,        -- NULL if no spell requirement e.g. "Pact of the Blade"
        prerequisite_pact   TEXT,        -- NULL if no pact boon requirement
        repeatable          INTEGER DEFAULT 0,
        description         TEXT NOT NULL
    )`
}
