package immutable

// ImmutableMonsterSchema — creature_type renamed to type; hp_average renamed to hp;
// strength/dexterity/constitution/intelligence/wisdom/charisma shortened to
// str/dex/con/int/wis/cha; challenge_rating renamed to cr.
func ImmutableMonsterSchema() string {
	return `CREATE TABLE IF NOT EXISTS monsters (
		id                     INTEGER PRIMARY KEY AUTOINCREMENT,
		name                   TEXT UNIQUE NOT NULL,
		source                 TEXT,
		size                   TEXT,
		type                   TEXT,        -- beast, undead, fiend, humanoid, etc.
		subtype                TEXT,
		alignment              TEXT,
		ac                     INTEGER,
		ac_notes               TEXT,
		hp                     INTEGER,
		hp_dice                TEXT,        -- e.g. "10d10 + 40"
		speed                  TEXT,
		cr                     TEXT,        -- TEXT to handle "1/2", "1/4", etc.
		proficiency_bonus      INTEGER,
		xp                     INTEGER,
		str                    INTEGER,
		dex                    INTEGER,
		con                    INTEGER,
		int                    INTEGER,
		wis                    INTEGER,
		cha                    INTEGER,
		save_str               INTEGER,
		save_dex               INTEGER,
		save_con               INTEGER,
		save_int               INTEGER,
		save_wis               INTEGER,
		save_cha               INTEGER,
		darkvision             INTEGER,
		blindsight             INTEGER,
		tremorsense            INTEGER,
		truesight              INTEGER,
		passive_perception     INTEGER,
		languages              TEXT,
		damage_immunities      TEXT,
		damage_resistances     TEXT,
		damage_vulnerabilities TEXT,
		condition_immunities   TEXT
	)`
}

func ImmutableMonsterActionSchema() string {
	return `CREATE TABLE IF NOT EXISTS monster_actions (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		monster_id   INTEGER REFERENCES monsters(id),
		name         TEXT NOT NULL,
		action_type  TEXT NOT NULL,  -- action, bonus_action, reaction, legendary, lair, mythic
		description  TEXT NOT NULL,
		attack_bonus INTEGER,
		reach        TEXT,
		hit_dice     TEXT,
		damage_type  TEXT,
		dc_value     INTEGER,
		dc_ability   TEXT,
		dc_effect    TEXT
	)`
}

func ImmutableMonsterTraitSchema() string {
	return `CREATE TABLE IF NOT EXISTS monster_traits (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		monster_id  INTEGER REFERENCES monsters(id),
		name        TEXT NOT NULL,
		description TEXT NOT NULL
	)`
}

// ImmutableMonsterLegendarySchema — table renamed from monster_legendary_info
// to monster_legendary to match bot code and queryExe immutableTables guard.
func ImmutableMonsterLegendarySchema() string {
	return `CREATE TABLE IF NOT EXISTS monster_legendary (
		id                     INTEGER PRIMARY KEY AUTOINCREMENT,
		monster_id             INTEGER UNIQUE REFERENCES monsters(id),
		legendary_action_count INTEGER,
		lair_action_initiative INTEGER,
		lair_description       TEXT
	)`
}

// ImmutableMonsterSpellcastingSchema — spellcasting_ability renamed to ability;
// spell_save_dc renamed to save_dc; spell_attack_bonus renamed to attack_bonus.
func ImmutableMonsterSpellcastingSchema() string {
	return `CREATE TABLE IF NOT EXISTS monster_spellcasting (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		monster_id   INTEGER REFERENCES monsters(id),
		ability      TEXT,    -- INT, WIS, CHA
		save_dc      INTEGER,
		attack_bonus INTEGER,
		spell_slots  TEXT,    -- e.g. "1st:4, 2nd:3"
		innate       INTEGER DEFAULT 0,
		notes        TEXT
	)`
}

// ImmutableMonsterSpellListSchema — table renamed from monster_spells to
// monster_spell_list to match bot code and queryExe immutableTables guard.
func ImmutableMonsterSpellListSchema() string {
	return `CREATE TABLE IF NOT EXISTS monster_spell_list (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		monster_id INTEGER REFERENCES monsters(id),
		spell_id   INTEGER REFERENCES spells(id),
		uses       TEXT     -- NULL for normal slots; e.g. "3/day", "at will"
	)`
}
