package immutable

func ImmutableBackgroundSchema() string {
	return `CREATE TABLE IF NOT EXISTS backgrounds (
		id                  INTEGER PRIMARY KEY AUTOINCREMENT,
		name                TEXT UNIQUE NOT NULL,
		source              TEXT,
		origin_feat         TEXT,
		skill_proficiencies TEXT,   -- comma-separated e.g. "Athletics, Deception"
		tool_proficiencies  TEXT,
		languages           TEXT,   -- e.g. "Any one language"
		equipment           TEXT,
		description         TEXT NOT NULL
	)`
}

func ImmutableBackgroundFeatureSchema() string {
	return `CREATE TABLE IF NOT EXISTS background_features (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		background_id INTEGER REFERENCES backgrounds(id),
		name          TEXT NOT NULL,
		description   TEXT NOT NULL
	)`
}
