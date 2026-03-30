package immutable

func ImmutableClassFeatureSchema() string {
	return `CREATE TABLE IF NOT EXISTS class_features (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    class_id     INTEGER REFERENCES classes(id),
    name         TEXT NOT NULL,
    level        INTEGER NOT NULL,  -- level the feature is gained
    description  TEXT NOT NULL
)`
}
