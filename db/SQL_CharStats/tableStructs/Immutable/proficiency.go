package immutable

// A single unified table handles all proficiency types efficiently.
// proficiency_type distinguishes between skill, tool, weapon, armor, and language.
// Tool proficiencies get extra columns for their unique mechanical interactions
// with ability checks, while skills and others leave those NULL.
func ImmutableProficiencySchema() string {
	return `CREATE TABLE IF NOT EXISTS proficiencies (
        id                  INTEGER PRIMARY KEY AUTOINCREMENT,
        name                TEXT UNIQUE NOT NULL,   -- e.g. "Stealth", "Thieves' Tools", "Longsword"
        proficiency_type    TEXT NOT NULL,           -- skill, tool, weapon, armor, language
        ability             TEXT,                    -- associated ability score e.g. "DEX" for Stealth
                                                     -- tools can use multiple abilities so this reflects the primary one
        alternate_abilities TEXT,                    -- comma separated e.g. "STR,INT" for tools that flex
        description         TEXT,
        -- Tool specific fields, NULL for non-tools
        craft_item          TEXT,                    -- what the tool can craft e.g. "Herbalism Kit -> potions"
        tool_notes          TEXT                     -- special interactions e.g. "Thieves' Tools required to pick locks"
    )`
}
