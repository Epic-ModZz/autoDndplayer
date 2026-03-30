package immutable

func ImmutableSubClassFeatureSchema() string {
	return `CREATE TABLE IF NOT EXISTS subclass_features (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    subclass_id  INTEGER REFERENCES subclasses(id),
    name         TEXT NOT NULL,
    level        INTEGER NOT NULL,
    description  TEXT NOT NULL
)`
}
