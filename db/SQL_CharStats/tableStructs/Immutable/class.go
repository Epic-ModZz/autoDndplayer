package immutable

func ImmutableClassSchema() string {
	return `CREATE TABLE IF NOT EXISTS classes (
		id                       INTEGER PRIMARY KEY AUTOINCREMENT,
		name                     TEXT UNIQUE NOT NULL,
		source                   TEXT,
		hit_die                  INTEGER,
		primary_ability          TEXT,
		saving_throws            TEXT,        -- comma-separated e.g. "Strength, Constitution"
		spellcasting_ability     TEXT,        -- INT, WIS, CHA, or NULL for non-casters
		armor_proficiencies      TEXT,
		weapon_proficiencies     TEXT,
		tool_proficiencies       TEXT,
		skill_choices            INTEGER,
		skill_options            TEXT,
		multiclass_req_ability   TEXT,
		multiclass_req_score     INTEGER,
		multiclass_proficiencies TEXT,
		description              TEXT
	)`
}
