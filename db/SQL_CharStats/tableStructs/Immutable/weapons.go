package immutable

func ImmutableWeaponSchema() string {
	return `CREATE TABLE IF NOT EXISTS weapons (
        id              INTEGER PRIMARY KEY AUTOINCREMENT,
        name            TEXT UNIQUE NOT NULL,
        source          TEXT,
        weapon_type     TEXT,        -- simple, martial
        damage_dice     TEXT,        -- e.g. "1d8"
        damage_type     TEXT,        -- slashing, piercing, bludgeoning
        weight          REAL,
        cost            TEXT,        -- e.g. "15gp"
        properties      TEXT,        -- comma separated e.g. "versatile, thrown"
        mastery         TEXT,        -- the mastery property name e.g. "Cleave"
        range_normal    INTEGER,     -- NULL if not a ranged weapon
        range_long      INTEGER      -- NULL if not a ranged weapon
    )`
}

func ImmutableWeaponMasterySchema() string {
	return `CREATE TABLE IF NOT EXISTS weapon_masteries (
        id              INTEGER PRIMARY KEY AUTOINCREMENT,
        name            TEXT UNIQUE NOT NULL,   -- e.g. "Cleave", "Graze", "Push"
        source          TEXT,
        description     TEXT NOT NULL,
        applicable_weapons TEXT                 -- comma separated list of weapons that have this mastery
    )`
}
