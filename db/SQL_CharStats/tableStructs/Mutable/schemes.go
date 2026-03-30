package mutable

// CharacterGoalsSchema — scope and notes removed to match bot schema description.
func CharacterGoalsSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_goals (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL REFERENCES characters(id),
		goal         TEXT NOT NULL,
		priority     INTEGER NOT NULL DEFAULT 1,
		status       TEXT NOT NULL DEFAULT 'active'
	)`
}

// CharacterSchemesSchema — scheme_name renamed to scheme; description/current_phase/
// exposure_risk/npcs_involved collapsed into notes to match bot schema description.
func CharacterSchemesSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_schemes (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL REFERENCES characters(id),
		scheme       TEXT NOT NULL,
		status       TEXT NOT NULL DEFAULT 'active',
		notes        TEXT
	)`
}

// CharacterPublicPersonaSchema — all specific columns replaced with single persona TEXT
// to match respondClassifier query and bot schema description.
func CharacterPublicPersonaSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_public_persona (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL UNIQUE REFERENCES characters(id),
		persona      TEXT
	)`
}

// CharacterLiesSchema — told_to renamed to target; still_active renamed to revealed
// (semantics inverted: 0=not revealed, 1=revealed).
func CharacterLiesSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_lies (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL REFERENCES characters(id),
		lie          TEXT NOT NULL,
		target       TEXT NOT NULL,
		revealed     INTEGER NOT NULL DEFAULT 0
	)`
}

// CharacterCorruptionArcSchema — corruption_level/origin_event/point_of_no_return/
// redemption_condition replaced with stage/notes/triggered_at to match LevelupPipeline
// query and bot schema description.
func CharacterCorruptionArcSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_corruption_arc (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL UNIQUE REFERENCES characters(id),
		stage        TEXT,
		notes        TEXT,
		triggered_at DATETIME
	)`
}

// CharacterTriggerEventsSchema — priority/fired/fired_session replaced with last_fired
// to match bot schema description.
func CharacterTriggerEventsSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_trigger_events (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL REFERENCES characters(id),
		trigger      TEXT NOT NULL,
		response     TEXT NOT NULL,
		last_fired   DATETIME
	)`
}

// CharacterAgentsSchema — agent_id/what_they_know/status replaced with location/notes
// to match bot schema description.
func CharacterAgentsSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_agents (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL REFERENCES characters(id),
		agent_name   TEXT,
		loyalty      INTEGER NOT NULL DEFAULT 50,
		role         TEXT,
		location     TEXT,
		notes        TEXT
	)`
}
