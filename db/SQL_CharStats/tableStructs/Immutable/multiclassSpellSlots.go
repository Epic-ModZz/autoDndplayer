package immutable

// ImmutableClassSpellcastingProgressionSchema — completely restructured from the
// old multiclass-calculation table into a per-level slot/cantrip progression table
// to match the infoGatherer "Class & Subclass Reference" batch description.
// The old progression_type/slot_multiplier columns are no longer used.
func ImmutableClassSpellcastingProgressionSchema() string {
	return `CREATE TABLE IF NOT EXISTS class_spellcasting_progression (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		class_id       INTEGER REFERENCES classes(id),
		level          INTEGER NOT NULL,
		cantrips_known INTEGER NOT NULL DEFAULT 0,
		spells_known   INTEGER,           -- NULL for prepared casters (wizards, clerics, etc.)
		slot_level_1   INTEGER NOT NULL DEFAULT 0,
		slot_level_2   INTEGER NOT NULL DEFAULT 0,
		slot_level_3   INTEGER NOT NULL DEFAULT 0,
		slot_level_4   INTEGER NOT NULL DEFAULT 0,
		slot_level_5   INTEGER NOT NULL DEFAULT 0,
		slot_level_6   INTEGER NOT NULL DEFAULT 0,
		slot_level_7   INTEGER NOT NULL DEFAULT 0,
		slot_level_8   INTEGER NOT NULL DEFAULT 0,
		slot_level_9   INTEGER NOT NULL DEFAULT 0,
		UNIQUE(class_id, level)
	)`
}

// ImmutableMulticlassSpellSlotSchema — spellcaster_level renamed to total_caster_level;
// slots_1st..slots_9th renamed to slot_level_1..slot_level_9.
func ImmutableMulticlassSpellSlotSchema() string {
	return `CREATE TABLE IF NOT EXISTS multiclass_spell_slots (
		total_caster_level INTEGER PRIMARY KEY,
		slot_level_1       INTEGER NOT NULL DEFAULT 0,
		slot_level_2       INTEGER NOT NULL DEFAULT 0,
		slot_level_3       INTEGER NOT NULL DEFAULT 0,
		slot_level_4       INTEGER NOT NULL DEFAULT 0,
		slot_level_5       INTEGER NOT NULL DEFAULT 0,
		slot_level_6       INTEGER NOT NULL DEFAULT 0,
		slot_level_7       INTEGER NOT NULL DEFAULT 0,
		slot_level_8       INTEGER NOT NULL DEFAULT 0,
		slot_level_9       INTEGER NOT NULL DEFAULT 0
	)`
}

// ImmutablePactMagicSlotSchema — slot_count renamed to slots.
func ImmutablePactMagicSlotSchema() string {
	return `CREATE TABLE IF NOT EXISTS pact_magic_slots (
		warlock_level INTEGER PRIMARY KEY,
		slot_level    INTEGER NOT NULL,
		slots         INTEGER NOT NULL,
		recharge_on   TEXT NOT NULL DEFAULT 'short_rest'
	)`
}

// ImmutableSingleClassSpellSlotSchema — class_level renamed to level;
// slots_1st..slots_9th renamed to slot_level_1..slot_level_9;
// cantrips_known moved to class_spellcasting_progression.
func ImmutableSingleClassSpellSlotSchema() string {
	return `CREATE TABLE IF NOT EXISTS single_class_spell_slots (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		class_id     INTEGER REFERENCES classes(id),
		level        INTEGER NOT NULL,
		slot_level_1 INTEGER NOT NULL DEFAULT 0,
		slot_level_2 INTEGER NOT NULL DEFAULT 0,
		slot_level_3 INTEGER NOT NULL DEFAULT 0,
		slot_level_4 INTEGER NOT NULL DEFAULT 0,
		slot_level_5 INTEGER NOT NULL DEFAULT 0,
		slot_level_6 INTEGER NOT NULL DEFAULT 0,
		slot_level_7 INTEGER NOT NULL DEFAULT 0,
		slot_level_8 INTEGER NOT NULL DEFAULT 0,
		slot_level_9 INTEGER NOT NULL DEFAULT 0,
		UNIQUE(class_id, level)
	)`
}
