package immutable

// Covers all mundane items - tools, adventuring gear, trade goods, etc.
// This is the rulebook reference the AI looks up when it needs mechanical details
func ImmutableMundaneItemSchema() string {
	return `CREATE TABLE IF NOT EXISTS mundane_items (
        id              INTEGER PRIMARY KEY AUTOINCREMENT,
        name            TEXT UNIQUE NOT NULL,
        source          TEXT,
        category        TEXT NOT NULL,       -- gear, tool, trade_good, vehicle, tack, food, container, etc.
        subcategory     TEXT,                -- e.g. "artisan_tool", "gaming_set", "musical_instrument"
        weight          REAL,               -- in pounds
        cost_gp         REAL,
        description     TEXT NOT NULL,       -- full item description including all mechanical uses
        -- Mechanical properties the AI needs to reason about
        capacity        TEXT,               -- e.g. "holds 3 gallons", "carries 100lb"
        charges         INTEGER,             -- NULL if no charges
        notes           TEXT                -- edge cases, special rules, common interactions
    )`
}

// All magic items with their full mechanical details
func ImmutableMagicItemSchema() string {
	return `CREATE TABLE IF NOT EXISTS magic_items (
        id                  INTEGER PRIMARY KEY AUTOINCREMENT,
        name                TEXT UNIQUE NOT NULL,
        source              TEXT,
        rarity              TEXT NOT NULL,   -- common, uncommon, rare, very_rare, legendary, artifact
        item_type           TEXT NOT NULL,   -- weapon, armor, wondrous, ring, rod, staff, wand, potion, scroll, ammunition
        subtype             TEXT,            -- e.g. "longsword", "leather armor" if it augments a specific item type
        requires_attunement INTEGER DEFAULT 0,
        attunement_prereq   TEXT,            -- NULL if anyone can attune e.g. "spellcaster", "paladin"
        charges             INTEGER,         -- NULL if no charges
        recharge            TEXT,            -- e.g. "dawn", "midnight", "1d6+1 at dawn"
        charges_on_depleted TEXT,            -- e.g. "roll d20, on 1 item is destroyed"
        description         TEXT NOT NULL,   -- full mechanical description word for word
        -- Quick reference fields so AI doesn't have to parse description every time
        grants_ac_bonus     INTEGER,         -- NULL if no AC bonus
        grants_attack_bonus INTEGER,         -- NULL if no attack bonus
        grants_damage_bonus INTEGER,         -- NULL if no damage bonus
        grants_spell_dc     INTEGER,         -- NULL if no spell DC bonus
        concentration       INTEGER DEFAULT 0, -- 1 if attunement or use requires concentration
        cursed              INTEGER DEFAULT 0,
        curse_description   TEXT,
        notes               TEXT
    )`
}

// The specific abilities and actions that a magic item grants, one row per ability
// Separating these out lets the AI query "what can this item do" without parsing a blob of text
func ImmutableMagicItemAbilitySchema() string {
	return `CREATE TABLE IF NOT EXISTS magic_item_abilities (
        id              INTEGER PRIMARY KEY AUTOINCREMENT,
        magic_item_id   INTEGER REFERENCES magic_items(id),
        name            TEXT NOT NULL,           -- e.g. "Vorpal Strike", "Absorb Elements"
        action_type     TEXT,                    -- action, bonus_action, reaction, passive, command_word
        charges_cost    INTEGER,                 -- NULL if no charge cost
        spell_id        INTEGER REFERENCES spells(id), -- NULL if not a spell
        save_dc         INTEGER,                 -- NULL if no saving throw
        save_ability    TEXT,                    -- STR, DEX, CON, INT, WIS, CHA
        description     TEXT NOT NULL
    )`
}

// Potions get their own table since they are consumed, have specific healing dice,
// and are referenced frequently by the garden facility in Arn
func ImmutablePotionSchema() string {
	return `CREATE TABLE IF NOT EXISTS potions (
        id              INTEGER PRIMARY KEY AUTOINCREMENT,
        magic_item_id   INTEGER REFERENCES magic_items(id),
        healing_dice    TEXT,                    -- e.g. "2d4+2", NULL if not a healing potion
        duration        TEXT,                    -- how long the effect lasts
        description     TEXT NOT NULL
    )`
}

// Stores every distinct DC interaction an item has, one row per action
// This handles items like manacles which have completely different DCs
// for escaping via DEX, bursting via STR, picking the lock via Thieves Tools, etc.
func ImmutableItemInteractionSchema() string {
	return `CREATE TABLE IF NOT EXISTS item_interactions (
        id               INTEGER PRIMARY KEY AUTOINCREMENT,
        mundane_item_id  INTEGER REFERENCES mundane_items(id),  -- NULL if magic item
        magic_item_id    INTEGER REFERENCES magic_items(id),    -- NULL if mundane item
        action_name      TEXT NOT NULL,    -- e.g. "Escape", "Burst", "Pick Lock", "Apply"
        action_type      TEXT NOT NULL,    -- action, bonus_action, reaction, utilize
        dc               INTEGER,          -- NULL if no DC (e.g. automatic success)
        ability          TEXT,             -- STR, DEX, CON, INT, WIS, CHA
        skill            TEXT,             -- Athletics, Sleight of Hand, etc. NULL if raw ability
        tool_required    TEXT,             -- e.g. "Thieves' Tools", NULL if no tool needed
        prerequisite     TEXT,             -- e.g. "target must have Grappled condition"
        description      TEXT NOT NULL     -- full description of what this interaction does
        )`
}
