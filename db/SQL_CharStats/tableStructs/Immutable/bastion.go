package immutable

// The connection levels used to gate stronghold upgrades and NPC interactions
func ImmutableConnectionLevelSchema() string {
	return `CREATE TABLE IF NOT EXISTS connection_levels (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        name        TEXT UNIQUE NOT NULL,    -- Contact, Acquaintance, Associate, Ally
        rank        INTEGER UNIQUE NOT NULL, -- 1-4 for ordering
        requirement TEXT NOT NULL            -- description of how this level is earned
    )`
}

// The different stronghold sizes available, indexed by facility points
func ImmutableStrongholdTierSchema() string {
	return `CREATE TABLE IF NOT EXISTS stronghold_tiers (
        id                      INTEGER PRIMARY KEY AUTOINCREMENT,
        facility_points         INTEGER UNIQUE NOT NULL, -- 1, 3, 5, 10, 15, 20
        cost_gp                 INTEGER NOT NULL,
        build_time_days         INTEGER NOT NULL,
        level_requirement       INTEGER,                 -- NULL for 1 and 3 FP tiers
        connection_required_id  INTEGER REFERENCES connection_levels(id), -- NULL for 1 and 3 FP tiers
        land_requirement        TEXT                     -- description of the RP requirement to acquire land
    )`
}

// All facility types available to build, including all Arn custom facilities
func ImmutableFacilityTypeSchema() string {
	return `CREATE TABLE IF NOT EXISTS facility_types (
        id                       INTEGER PRIMARY KEY AUTOINCREMENT,
        name                     TEXT UNIQUE NOT NULL,
        level_requirement        INTEGER NOT NULL,        -- 5, 9, 13, or 17
        base_facility_points     INTEGER NOT NULL,        -- base FP cost before discounts
        cost_gp                  INTEGER NOT NULL,
        build_time_days          INTEGER NOT NULL,
        prerequisite_feature     TEXT,                    -- e.g. "Pact Magic", "Martial Arts", "Druidic Focus"
        prerequisite_proficiency TEXT,                    -- e.g. "Thieves Tools", "Survival"
        prerequisite_class       TEXT,                    -- NULL if no class restriction
        requires_arn_approval    INTEGER DEFAULT 0,       -- 1 for special facilities like Soul Tethering Idol
        faction_only             INTEGER DEFAULT 0,       -- 1 if only factions can build this
        adds_fp_to_other_room    INTEGER DEFAULT 0,       -- 1 if this facility adds FP cost to another room rather than its own
        description              TEXT NOT NULL
    )`
}

// The mechanical benefits each facility provides, broken into individual queryable rows
func ImmutableFacilityBenefitSchema() string {
	return `CREATE TABLE IF NOT EXISTS facility_benefits (
        id               INTEGER PRIMARY KEY AUTOINCREMENT,
        facility_type_id INTEGER REFERENCES facility_types(id),
        name             TEXT NOT NULL,
        description      TEXT NOT NULL,
        frequency        TEXT,             -- passive, daily, weekly, monthly
        cost_gp          INTEGER,          -- NULL if no gold cost to activate
        requires_rp      INTEGER DEFAULT 0 -- 1 if an RP scene is required to use this benefit
    )`
}

func ImmutableFacilityUpgradeSchema() string {
	return `CREATE TABLE IF NOT EXISTS facility_upgrades (
        id               INTEGER PRIMARY KEY AUTOINCREMENT,
        facility_type_id INTEGER REFERENCES facility_types(id),
        upgrade_name     TEXT NOT NULL,    -- e.g. "Expanded", "Focused", "Specialized"
        level_requirement INTEGER,
        additional_fp    INTEGER NOT NULL, -- extra FP this upgrade costs
        cost_gp          INTEGER NOT NULL,
        build_time_days  INTEGER NOT NULL,
        description      TEXT NOT NULL
    )`
}

// FP discounts that apply when co-locating certain facilities
// e.g. Armory costs -1 FP if you also have a Barracks
func ImmutableFacilityDiscountSchema() string {
	return `CREATE TABLE IF NOT EXISTS facility_discounts (
        id                   INTEGER PRIMARY KEY AUTOINCREMENT,
        facility_type_id     INTEGER REFERENCES facility_types(id),   -- facility receiving the discount
        requires_facility_id INTEGER REFERENCES facility_types(id),   -- facility that must already exist
        fp_discount          INTEGER NOT NULL
    )`
}
