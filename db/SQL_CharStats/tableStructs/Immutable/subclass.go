package immutable

func ImmutableSubClassSchema() string {
	return `CREATE TABLE IF NOT EXISTS subclasses (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    class_id    INTEGER REFERENCES classes(id),
    name        TEXT UNIQUE NOT NULL,
    source      TEXT,
    description TEXT
)`
}
