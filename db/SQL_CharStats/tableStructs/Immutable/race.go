package immutable

func ImmutableRaceSchema() string {
	return `CREATE TABLE IF NOT EXISTS races (
		id                    INTEGER PRIMARY KEY AUTOINCREMENT,
		name                  TEXT UNIQUE NOT NULL,
		source                TEXT,
		size                  TEXT,
		speed                 INTEGER,
		ability_score_increases TEXT, -- e.g. "Choose +2/+1 or three +1s"
		languages             TEXT,   -- e.g. "Common plus one of your choice"
		traits_summary        TEXT,   -- comma-separated list of trait names
		description           TEXT
	)`
}
