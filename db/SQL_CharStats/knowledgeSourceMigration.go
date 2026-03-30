package db

import (
	"log"
	"strings"
)

// RunKnowledgeSourceMigration adds the columns that distinguish what the
// character learned in-fiction from what the player knows out-of-character.
//
// Call this from Init() after createTables(). It is safe to run on an
// existing database — SQLite will error on duplicate columns, which we
// catch and ignore.
//
// Columns added:
//   character_notes.knowledge_source  TEXT DEFAULT 'ic'
//     Values: 'ic' (learned in-fiction), 'ooc' (OOC channel), 'dm' (private message)
//
//   npc_details.discovered_ic  INTEGER DEFAULT 1
//     0 = the player knows about this NPC but the character has not met them IC
//     1 = the character has encountered this NPC in-fiction
//
//   npc_secrets.discovered_ic  INTEGER DEFAULT 1
//     Same semantics — was this secret learned IC or via OOC/DM?
func RunKnowledgeSourceMigration() {
	migrations := []struct {
		label string
		sql   string
	}{
		{
			"character_notes.knowledge_source",
			`ALTER TABLE character_notes ADD COLUMN knowledge_source TEXT NOT NULL DEFAULT 'ic'`,
		},
		{
			"npc_details.discovered_ic",
			`ALTER TABLE npc_details ADD COLUMN discovered_ic INTEGER NOT NULL DEFAULT 1`,
		},
		{
			"npc_secrets.discovered_ic",
			`ALTER TABLE npc_secrets ADD COLUMN discovered_ic INTEGER NOT NULL DEFAULT 1`,
		},
	}

	for _, m := range migrations {
		if _, err := DB.Exec(m.sql); err != nil {
			// "duplicate column name" is expected on subsequent startups — ignore it.
			if strings.Contains(err.Error(), "duplicate column") {
				continue
			}
			// Anything else is unexpected — log loudly but don't fatal.
			log.Printf("knowledge source migration warning [%s]: %v", m.label, err)
		} else {
			log.Printf("knowledge source migration applied: %s", m.label)
		}
	}
}
