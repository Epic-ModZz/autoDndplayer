package immutable

// ImmutableArmorSchema — dex_bonus TEXT kept for type ("full"/"max2"/"none");
// max_dex_bonus INTEGER added for numeric cap; strength_required renamed to
// strength_requirement; stealth_penalty renamed to stealth_disadvantage;
// cost_gp renamed to cost.
func ImmutableArmorSchema() string {
	return `CREATE TABLE IF NOT EXISTS armor (
		id                   INTEGER PRIMARY KEY AUTOINCREMENT,
		name                 TEXT UNIQUE NOT NULL,
		source               TEXT,
		armor_type           TEXT NOT NULL,       -- light, medium, heavy, shield
		base_ac              INTEGER NOT NULL,
		dex_bonus            TEXT,                -- "full", "max2", "none"
		max_dex_bonus        INTEGER,             -- numeric cap; NULL means full DEX applies
		strength_requirement INTEGER,             -- NULL if no STR requirement
		stealth_disadvantage INTEGER DEFAULT 0,   -- 1 if disadvantage on Stealth checks
		don_time             TEXT,
		doff_time            TEXT,
		weight               REAL,
		cost                 TEXT,                -- e.g. "1500 gp"
		description          TEXT
	)`
}
