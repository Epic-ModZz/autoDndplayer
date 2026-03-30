package immutable

func ImmutableBoonSchema() string {
	return `CREATE TABLE IF NOT EXISTS boons (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        name        TEXT UNIQUE NOT NULL,
        source      TEXT,                -- DMG, campaign specific, etc.
        prerequisite TEXT,               -- e.g. "level 19"
        description TEXT NOT NULL
    )`
}

func ImmutableBoonFeatureSchema() string {
	return `CREATE TABLE IF NOT EXISTS boon_features (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        boon_id     INTEGER REFERENCES boons(id),
        name        TEXT NOT NULL,
        description TEXT NOT NULL
    )`
}
