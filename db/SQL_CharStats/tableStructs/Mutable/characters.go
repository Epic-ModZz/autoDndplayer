package mutable

// CharactersSchema — canonical column names used throughout the bot.
// race_id/background_id replace the old race TEXT and missing background_id.
// level replaces total_level; hp/max_hp/armor_class replace hp_current/hp_max/ac.
// NOTE: respondClassifier.go still references c.race and c.total_level —
// update those two lines after applying this schema (see bottom of file).
func CharactersSchema() string {
	return `CREATE TABLE IF NOT EXISTS characters (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		name              TEXT UNIQUE NOT NULL,
		race_id           INTEGER REFERENCES races(id),
		background_id     INTEGER REFERENCES backgrounds(id),
		alignment         TEXT,
		experience        INTEGER DEFAULT 0,
		level             INTEGER,
		hp                INTEGER,
		max_hp            INTEGER,
		temp_hp           INTEGER DEFAULT 0,
		armor_class       INTEGER,
		speed             INTEGER,
		proficiency_bonus INTEGER,
		discord_user_id   TEXT,
		strength          INTEGER,
		dexterity         INTEGER,
		constitution      INTEGER,
		intelligence      INTEGER,
		wisdom            INTEGER,
		charisma          INTEGER
	)`
}

// NpcDetailsSchema — standalone NPC table, not linked via characters.
// Matches infoGatherer / Memorywriter descriptions exactly.
// discovered_ic is added later by knowledgeSourceMigration.
func NpcDetailsSchema() string {
	return `CREATE TABLE IF NOT EXISTS npc_details (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT NOT NULL,
		race        TEXT,
		role        TEXT,
		location    TEXT,
		disposition TEXT,
		alive       INTEGER DEFAULT 1,
		notes       TEXT
	)`
}

// NpcSecretsSchema — npc_id replaces character_id; known_by replaces revealed_to.
// discovered_ic is added later by knowledgeSourceMigration.
func NpcSecretsSchema() string {
	return `CREATE TABLE IF NOT EXISTS npc_secrets (
		id       INTEGER PRIMARY KEY AUTOINCREMENT,
		npc_id   INTEGER NOT NULL REFERENCES npc_details(id),
		secret   TEXT NOT NULL,
		known_by TEXT,
		revealed INTEGER NOT NULL DEFAULT 0
	)`
}

// CharacterClassesSchema — added is_primary (required by respondClassifier and LevelupPipeline).
func CharacterClassesSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_classes (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL REFERENCES characters(id),
		class_id     INTEGER NOT NULL REFERENCES classes(id),
		subclass_id  INTEGER REFERENCES subclasses(id),
		level        INTEGER NOT NULL,
		is_primary   INTEGER NOT NULL DEFAULT 0
	)`
}

// CharacterSpellSlotsSchema — total renamed to maximum.
func CharacterSpellSlotsSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_spell_slots (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL REFERENCES characters(id),
		slot_level   INTEGER NOT NULL,
		maximum      INTEGER NOT NULL,
		remaining    INTEGER NOT NULL
	)`
}

// CharacterSpellsKnownSchema — is_prepared renamed to prepared; always_prepared added.
func CharacterSpellsKnownSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_spells_known (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id    INTEGER NOT NULL REFERENCES characters(id),
		spell_id        INTEGER NOT NULL REFERENCES spells(id),
		prepared        INTEGER NOT NULL DEFAULT 1,
		always_prepared INTEGER NOT NULL DEFAULT 0
	)`
}

// CharacterFeaturesSchema — source_level added.
func CharacterFeaturesSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_features (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL REFERENCES characters(id),
		feature_id   INTEGER NOT NULL,
		source       TEXT NOT NULL,
		source_level INTEGER
	)`
}

// CharacterFeatsSchema — gained_at_level replaced with source TEXT.
func CharacterFeatsSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_feats (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL REFERENCES characters(id),
		feat_id      INTEGER NOT NULL REFERENCES feats(id),
		source       TEXT
	)`
}

// CharacterProficienciesSchema — proficiency_type renamed to type; expertise added.
func CharacterProficienciesSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_proficiencies (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id   INTEGER NOT NULL REFERENCES characters(id),
		proficiency_id INTEGER NOT NULL REFERENCES proficiencies(id),
		type           TEXT NOT NULL DEFAULT 'full',
		expertise      INTEGER NOT NULL DEFAULT 0
	)`
}

// CharacterInventorySchema — is_equipped renamed to equipped; is_attuned renamed to attunement.
func CharacterInventorySchema() string {
	return `CREATE TABLE IF NOT EXISTS character_inventory (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL REFERENCES characters(id),
		item_name    TEXT NOT NULL,
		item_type    TEXT,
		quantity     INTEGER NOT NULL DEFAULT 1,
		equipped     INTEGER NOT NULL DEFAULT 0,
		attunement   INTEGER NOT NULL DEFAULT 0,
		notes        TEXT
	)`
}

// CharacterCurrencySchema — ep (electrum) added.
func CharacterCurrencySchema() string {
	return `CREATE TABLE IF NOT EXISTS character_currency (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL UNIQUE REFERENCES characters(id),
		pp           INTEGER NOT NULL DEFAULT 0,
		gp           INTEGER NOT NULL DEFAULT 0,
		ep           INTEGER NOT NULL DEFAULT 0,
		sp           INTEGER NOT NULL DEFAULT 0,
		cp           INTEGER NOT NULL DEFAULT 0
	)`
}

// CharacterConditionsSchema — condition_id INTEGER replaced with condition_name TEXT.
func CharacterConditionsSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_conditions (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id   INTEGER NOT NULL REFERENCES characters(id),
		condition_name TEXT NOT NULL,
		duration       TEXT,
		source         TEXT
	)`
}

// CharacterHitDiceSchema — die_type added; class_id removed; total renamed to maximum.
func CharacterHitDiceSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_hit_dice (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL REFERENCES characters(id),
		die_type     TEXT NOT NULL,
		remaining    INTEGER NOT NULL,
		maximum      INTEGER NOT NULL
	)`
}

// CharacterDeathSavesSchema — stable column added.
func CharacterDeathSavesSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_death_saves (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL UNIQUE REFERENCES characters(id),
		successes    INTEGER NOT NULL DEFAULT 0,
		failures     INTEGER NOT NULL DEFAULT 0,
		stable       INTEGER NOT NULL DEFAULT 0
	)`
}

// CharacterConcentrationSchema — simplified to match bot description:
// spell_name/cast_at_level/started_at replaced with duration_remaining.
func CharacterConcentrationSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_concentration (
		id                 INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id       INTEGER NOT NULL UNIQUE REFERENCES characters(id),
		spell_id           INTEGER REFERENCES spells(id),
		duration_remaining INTEGER
	)`
}

func CharacterInspirationSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_inspiration (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id    INTEGER NOT NULL UNIQUE REFERENCES characters(id),
		has_inspiration INTEGER NOT NULL DEFAULT 0
	)`
}

// CharacterPersonalitySchema — personality_traits renamed to traits.
func CharacterPersonalitySchema() string {
	return `CREATE TABLE IF NOT EXISTS character_personality (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL UNIQUE REFERENCES characters(id),
		traits       TEXT,
		ideals       TEXT,
		bonds        TEXT,
		flaws        TEXT
	)`
}

func CharacterBackstorySchema() string {
	return `CREATE TABLE IF NOT EXISTS character_backstory (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL UNIQUE REFERENCES characters(id),
		backstory    TEXT,
		updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
}

// CharacterNotesSchema — session_id removed; knowledge_source added by migration.
func CharacterNotesSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_notes (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL REFERENCES characters(id),
		note         TEXT NOT NULL,
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
}

// CharacterRelationshipsSchema — related_to_id/related_to_name/disposition replaced
// with npc_id/relationship_type/trust_level.
func CharacterRelationshipsSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_relationships (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id      INTEGER NOT NULL REFERENCES characters(id),
		npc_id            INTEGER REFERENCES npc_details(id),
		relationship_type TEXT,
		trust_level       INTEGER,
		notes             TEXT
	)`
}

// CharacterFactionStandingSchema — reputation renamed to standing; notes added.
func CharacterFactionStandingSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_faction_standing (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL REFERENCES characters(id),
		faction_name TEXT NOT NULL,
		standing     TEXT,
		rank         TEXT,
		notes        TEXT
	)`
}

// CharacterQuestLogSchema — session_acquired renamed to giver; updated_at added.
func CharacterQuestLogSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_quest_log (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL REFERENCES characters(id),
		quest_name   TEXT NOT NULL,
		status       TEXT NOT NULL DEFAULT 'active',
		giver        TEXT,
		notes        TEXT,
		updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
}

// SessionLogSchema — session_number renamed to session_date; participants and dm_notes added.
func SessionLogSchema() string {
	return `CREATE TABLE IF NOT EXISTS session_log (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		session_date TEXT,
		summary      TEXT,
		participants TEXT,
		dm_notes     TEXT
	)`
}

// CharacterSessionStatsSchema — damage_taken/spells_cast replaced with
// healing_done/knocks/deaths to match bot code.
func CharacterSessionStatsSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_session_stats (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL REFERENCES characters(id),
		session_id   INTEGER NOT NULL REFERENCES session_log(id),
		kills        INTEGER DEFAULT 0,
		damage_dealt INTEGER DEFAULT 0,
		healing_done INTEGER DEFAULT 0,
		knocks       INTEGER DEFAULT 0,
		deaths       INTEGER DEFAULT 0
	)`
}

// CharacterClassResourcesSchema — remaining→current, total→maximum, resets_on→recharge_on.
func CharacterClassResourcesSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_class_resources (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id  INTEGER NOT NULL REFERENCES characters(id),
		resource_name TEXT NOT NULL,
		current       INTEGER NOT NULL,
		maximum       INTEGER NOT NULL,
		recharge_on   TEXT NOT NULL
	)`
}

func CharacterExhaustionSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_exhaustion (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL UNIQUE REFERENCES characters(id),
		level        INTEGER NOT NULL DEFAULT 0
	)`
}

func CharacterCharSheetSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_sheets (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		discord_user_id TEXT     NOT NULL REFERENCES discord_users(discord_user_id),
		character_id    INTEGER  NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
		sheet_type      TEXT     NOT NULL CHECK(sheet_type IN ('gsheet', 'dicecloudv2')),
		sheet_id        TEXT     NOT NULL,
		created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(discord_user_id, character_id)
	)`
}

func OOCPlayerSchema() string {
	return `CREATE TABLE IF NOT EXISTS discord_users (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		discord_user_id TEXT     NOT NULL UNIQUE,
		username        TEXT     NOT NULL,
		display_name    TEXT,
		is_dm           BOOLEAN  NOT NULL DEFAULT 0,
		is_bot          BOOLEAN  NOT NULL DEFAULT 0,
		timezone        TEXT,
		joined_at       DATETIME,
		first_seen_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_seen_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`
}

func CharacterPendingLevelUpSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_pending_levelup (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id INTEGER NOT NULL UNIQUE REFERENCES characters(id),
		new_level    INTEGER NOT NULL,
		pending      BOOLEAN NOT NULL DEFAULT TRUE,
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
}
