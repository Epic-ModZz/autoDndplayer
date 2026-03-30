package immutable

func ImmutableSubraceSchema() string {
	return `CREATE TABLE IF NOT EXISTS subraces (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		race_id     INTEGER REFERENCES races(id),
		name        TEXT UNIQUE NOT NULL,
		source      TEXT,
		description TEXT
	)`
}
