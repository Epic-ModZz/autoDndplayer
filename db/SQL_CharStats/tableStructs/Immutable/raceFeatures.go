package immutable

func ImmutableRaceFeatureSchema() string {
	return `CREATE TABLE IF NOT EXISTS race_features (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		race_id     INTEGER REFERENCES races(id),
		name        TEXT NOT NULL,
		description TEXT NOT NULL,
		level       INTEGER NOT NULL DEFAULT 1
	)`
}

func ImmutableSubraceFeatureSchema() string {
	return `CREATE TABLE IF NOT EXISTS subrace_features (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		subrace_id  INTEGER REFERENCES subraces(id),
		name        TEXT NOT NULL,
		description TEXT NOT NULL,
		level       INTEGER NOT NULL DEFAULT 1
	)`
}
