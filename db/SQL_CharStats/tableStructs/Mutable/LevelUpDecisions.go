package mutable

// CharacterLevelUpDecisionsSchema stores the AI's deliberated choices for each
// level-up. A human reads this and manually applies the changes to the sheet.
// Once applied, applied_at is set so the bot knows the sheet is current.
func CharacterLevelUpDecisionsSchema() string {
	return `CREATE TABLE IF NOT EXISTS character_levelup_decisions (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		character_id    INTEGER NOT NULL,
		new_level       INTEGER NOT NULL,
		decisions_json  TEXT    NOT NULL, -- full structured JSON blob of all choices
		reasoning       TEXT    NOT NULL, -- plain-English explanation of why
		applied         INTEGER NOT NULL DEFAULT 0,
		applied_at      TEXT,
		created_at      TEXT    NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY (character_id) REFERENCES characters(id)
	);`
}
