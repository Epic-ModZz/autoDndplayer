package immutable

func ImmutableConditionSchema() string {
	return `CREATE TABLE IF NOT EXISTS conditions (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        name        TEXT UNIQUE NOT NULL,   -- e.g. "Frightened", "Grappled", "Exhaustion"
        source      TEXT,
        stackable   INTEGER DEFAULT 0,      -- 1 if condition has multiple levels e.g. Exhaustion
        max_stacks  INTEGER,                -- NULL if not stackable, 6 for Exhaustion
        description TEXT NOT NULL
    )`
}

func ImmutableConditionEffectSchema() string {
	return `CREATE TABLE IF NOT EXISTS condition_effects (
        id           INTEGER PRIMARY KEY AUTOINCREMENT,
        condition_id INTEGER REFERENCES conditions(id),
        stack_level  INTEGER DEFAULT 1,     -- which stack level this effect applies to, 1 for non-stackable conditions
        description  TEXT NOT NULL          -- e.g. "Speed halved" at exhaustion level 1
    )`
}
