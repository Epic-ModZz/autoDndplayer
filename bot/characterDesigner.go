package bot

import (
	dbpkg "PCL/db/SQL_CharStats"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// startingLevel is the westmarch starting level for the bot's character.
// Level 3 means the character has their subclass and core class features.
const startingLevel = 3

// DesignedCharacter is the full character spec the LLM produces.
type DesignedCharacter struct {
	Name        string   `json:"name"`
	Race        string   `json:"race"`
	Class       string   `json:"class"`
	Subclass    string   `json:"subclass"`
	Level       int      `json:"level"`
	Background  string   `json:"background"`
	Alignment   string   `json:"alignment"`
	Personality string   `json:"personality"`
	Backstory   string   `json:"backstory"`
	PublicFace  string   `json:"public_face"` // what the world sees
	TrueGoal    string   `json:"true_goal"`   // the secret agenda
	Feats       []string `json:"feats"`
	Spells      []string `json:"spells"`
	Reasoning   string   `json:"reasoning"`
}

// DesignAndSeedCharacter asks the LLM to design the optimal villain character
// using the game's actual available options, then seeds it into the database.
func DesignAndSeedCharacter() error {
	if dbpkg.DB == nil {
		return fmt.Errorf("DB not initialized")
	}

	// Pull reference data the LLM needs to make informed choices.
	options, err := gatherDesignOptions()
	if err != nil {
		return fmt.Errorf("failed to gather options: %w", err)
	}

	// Ask the LLM to design the character.
	character, err := designCharacterWithLLM(options)
	if err != nil {
		return fmt.Errorf("LLM design failed: %w", err)
	}

	log.Printf("DesignAndSeedCharacter: designed %s (%s %s %d)", character.Name, character.Race, character.Class, character.Level)
	log.Printf("DesignAndSeedCharacter: true goal — %s", character.TrueGoal)

	// Seed the character into the database.
	if err := seedCharacter(character); err != nil {
		return fmt.Errorf("seeding failed: %w", err)
	}

	// Invalidate the classifier cache so it picks up the new character.
	InvalidateCharacterContext()

	log.Printf("DesignAndSeedCharacter: %s seeded and ready", character.Name)
	return nil
}

// gatherDesignOptions pulls the available races, classes, feats, and spells
// from the DB so the LLM can make choices grounded in what actually exists.
func gatherDesignOptions() (string, error) {
	var sb strings.Builder

	// Races: name-only list so the LLM copies the exact string into JSON.
	sb.WriteString("## Available Races (use the EXACT name as shown)\n")
	{
		rows, err := dbpkg.DB.Query(`SELECT name FROM races ORDER BY name`)
		if err != nil {
			sb.WriteString(fmt.Sprintf("(query error: %v)\n\n", err))
		} else {
			for rows.Next() {
				var name string
				rows.Scan(&name)
				sb.WriteString(name + "\n")
			}
			rows.Close()
		}
	}
	sb.WriteString("\n")

	// Backgrounds: exact names only.
	sb.WriteString("## Available Backgrounds (use the EXACT name as shown)\n")
	{
		rows, err := dbpkg.DB.Query(`SELECT name FROM backgrounds ORDER BY name LIMIT 40`)
		if err != nil {
			sb.WriteString(fmt.Sprintf("(query error: %v)\n\n", err))
		} else {
			for rows.Next() {
				var name string
				rows.Scan(&name)
				sb.WriteString(name + "\n")
			}
			rows.Close()
		}
	}
	sb.WriteString("\n")

	// Classes: name + hit die so the LLM can reason about HP.
	sb.WriteString("## Available Classes (use the EXACT name as shown)\n")
	{
		rows, err := dbpkg.DB.Query(`SELECT name, hit_die FROM classes ORDER BY name`)
		if err != nil {
			sb.WriteString(fmt.Sprintf("(query error: %v)\n\n", err))
		} else {
			for rows.Next() {
				var name string
				var hitDie int
				rows.Scan(&name, &hitDie)
				sb.WriteString(fmt.Sprintf("%s (d%d)\n", name, hitDie))
			}
			rows.Close()
		}
	}
	sb.WriteString("\n")

	// Subclasses available at the starting level — the character MUST have one.
	sb.WriteString(fmt.Sprintf("## Available Subclasses at Level %d (class | subclass — use the EXACT subclass name)\n", startingLevel))
	{
		rows, err := dbpkg.DB.Query(
			`SELECT cl.name, sc.name FROM subclasses sc
			 JOIN classes cl ON cl.id = sc.class_id
			 ORDER BY cl.name, sc.name`)
		if err != nil {
			sb.WriteString(fmt.Sprintf("(query error: %v)\n\n", err))
		} else {
			for rows.Next() {
				var class, sub string
				rows.Scan(&class, &sub)
				sb.WriteString(fmt.Sprintf("%s | %s\n", class, sub))
			}
			rows.Close()
		}
	}
	sb.WriteString("\n")

	// Class features that unlock at levels 1–3 so the LLM understands what
	// the character actually has and can write a believable backstory.
	sb.WriteString("## Class Features (levels 1–3)\n")
	{
		rows, err := dbpkg.DB.Query(
			`SELECT cl.name, cf.level, cf.name FROM class_features cf
			 JOIN classes cl ON cl.id = cf.class_id
			 WHERE cf.level <= 3
			 ORDER BY cl.name, cf.level, cf.name`)
		if err != nil {
			sb.WriteString(fmt.Sprintf("(query error: %v)\n\n", err))
		} else {
			for rows.Next() {
				var class, feat string
				var level int
				rows.Scan(&class, &level, &feat)
				sb.WriteString(fmt.Sprintf("%s L%d: %s\n", class, level, feat))
			}
			rows.Close()
		}
	}
	sb.WriteString("\n")

	// Feats relevant to a villain (control, deception, influence).
	sb.WriteString("## Feats (use the EXACT name as shown)\n")
	{
		rows, err := dbpkg.DB.Query(`
			SELECT name, description FROM feats WHERE
				description LIKE '%damage%' OR description LIKE '%control%' OR
				description LIKE '%fear%' OR description LIKE '%charm%' OR
				description LIKE '%necrotic%' OR description LIKE '%psychic%' OR
				description LIKE '%invisible%' OR description LIKE '%illusion%'
			ORDER BY name LIMIT 40`)
		if err != nil {
			sb.WriteString(fmt.Sprintf("(query error: %v)\n\n", err))
		} else {
			for rows.Next() {
				var name, desc string
				rows.Scan(&name, &desc)
				if len(desc) > 120 {
					desc = desc[:120] + "..."
				}
				sb.WriteString(fmt.Sprintf("%s | %s\n", name, desc))
			}
			rows.Close()
		}
	}
	sb.WriteString("\n")

	// Spells relevant to a villain.
	sb.WriteString("## High-impact Spells (use the EXACT name as shown)\n")
	{
		rows, err := dbpkg.DB.Query(`
			SELECT name, level, school, description FROM spells WHERE
				(description LIKE '%destroy%' OR description LIKE '%dominate%' OR
				 description LIKE '%fear%' OR description LIKE '%charm%' OR
				 description LIKE '%control%' OR description LIKE '%necrotic%' OR
				 description LIKE '%psychic%' OR description LIKE '%illusion%' OR
				 description LIKE '%compulsion%' OR description LIKE '%suggestion%')
				AND level <= 6
			ORDER BY level, name LIMIT 60`)
		if err != nil {
			sb.WriteString(fmt.Sprintf("(query error: %v)\n\n", err))
		} else {
			for rows.Next() {
				var name, school, desc string
				var level int
				rows.Scan(&name, &level, &school, &desc)
				if len(desc) > 120 {
					desc = desc[:120] + "..."
				}
				sb.WriteString(fmt.Sprintf("%s (level %d %s) | %s\n", name, level, school, desc))
			}
			rows.Close()
		}
	}
	sb.WriteString("\n")

	return sb.String(), nil
}

// designCharacterWithLLM asks the LLM to create the optimal villain character.
// Uses a two-step approach: free-text reasoning first, then a strict JSON
// extraction call.
func designCharacterWithLLM(options string) (*DesignedCharacter, error) {
	// ── Step 1: free-text reasoning ─────────────────────────────────────────
	reasoningPrompt := fmt.Sprintf(`You are designing a D&D 5e character for a westmarch Discord server.
This character is secretly a long-running villain — they intend to corrupt institutions,
manipulate other players, and destroy the setting from within while appearing trustworthy.

The character must be:
- Believable as a hero or neutral party to other players
- Mechanically optimized for deception, control, and long-term influence
- Narratively compelling — corruption that feels earned, not cartoonish
- Capable of operating alone when necessary
- Level %d — they already have their subclass and core class features

## Available Options (from the actual game database)
%s

Reason through every choice below. Be specific about WHY each pick serves the villain agenda.
Cover: race, class, subclass (REQUIRED — the character is level %d and has their subclass),
background, alignment, name, personality (public face), backstory (cover story),
true secret goal, feats (up to 1 at level %d), and starting spells.

IMPORTANT: When choosing race, class, subclass, background, feats, and spells, you MUST
use the exact name as it appears in the lists above. Do not invent names or use alternatives.`,
		startingLevel, options, startingLevel, startingLevel)

	reasoningConfig := RoleplayConfig()
	reasoningConfig.Temperature = 0.7
	reasoningConfig.NumCtx = 12288

	reasoning, err := Query(reasoningPrompt, reasoningConfig)
	if err != nil {
		return nil, fmt.Errorf("reasoning call failed: %w", err)
	}
	log.Printf("DesignAndSeedCharacter: reasoning complete (%d chars)", len(reasoning))

	// ── Step 2: strict JSON extraction ───────────────────────────────────────
	jsonPrompt := fmt.Sprintf(`Convert the following character design into a JSON object.
Output ONLY the raw JSON object — no markdown, no backticks, no explanation, no extra text.
Start your response with { and end with }.

CRITICAL: The "race", "class", "subclass", "background", "feats", and "spells" fields MUST
use names copied exactly (character-for-character) from the Available Options lists in the
design. Do not paraphrase, abbreviate, or combine names. The "subclass" field is REQUIRED
and must not be empty — the character is level %d.

Character design to convert:
%s

Required JSON format:
{
  "name": "character name",
  "race": "exact race name from the Available Races list",
  "class": "exact class name from the Available Classes list",
  "subclass": "exact subclass name from the Available Subclasses list",
  "level": %d,
  "background": "exact background name from the Available Backgrounds list",
  "alignment": "alignment",
  "personality": "outward personality traits other players will see",
  "backstory": "the cover story in 2-3 sentences",
  "public_face": "one sentence describing how the character presents to the world",
  "true_goal": "the secret agenda in 2-3 sentences",
  "feats": ["exact feat name from the Feats list"],
  "spells": ["exact spell name from the High-impact Spells list"],
  "reasoning": "one sentence summary of the core design concept"
}`, startingLevel, reasoning, startingLevel)

	jsonConfig := SQLConfig()
	jsonConfig.Temperature = 0.0
	jsonConfig.NumCtx = 8192

	jsonResponse, err := Query(jsonPrompt, jsonConfig)
	if err != nil {
		return nil, fmt.Errorf("JSON extraction call failed: %w", err)
	}

	cleaned := extractJSONBlock(jsonResponse)
	if cleaned == "" {
		return nil, fmt.Errorf("no JSON found in extraction response:\n%s", jsonResponse)
	}

	var character DesignedCharacter
	if err := json.Unmarshal([]byte(cleaned), &character); err != nil {
		return nil, fmt.Errorf("JSON parse failed: %w\nraw: %s", err, cleaned)
	}

	// Enforce the starting level regardless of what the LLM wrote.
	character.Level = startingLevel

	return &character, nil
}

// lookupID tries an exact case-insensitive match first, then falls back to a
// LIKE contains-match. Returns 0 if nothing is found.
func lookupID(table, column, name string) int {
	if name == "" {
		return 0
	}
	var id int
	// Exact match
	dbpkg.DB.QueryRow(
		fmt.Sprintf(`SELECT id FROM %s WHERE LOWER(%s) = LOWER(?)`, table, column),
		name,
	).Scan(&id)
	if id != 0 {
		return id
	}
	// Partial match — DB name contains the LLM name, or vice versa
	dbpkg.DB.QueryRow(
		fmt.Sprintf(`SELECT id FROM %s WHERE LOWER(%s) LIKE LOWER(?) OR LOWER(?) LIKE '%%' || LOWER(%s) || '%%' LIMIT 1`, table, column, column),
		"%"+name+"%", name,
	).Scan(&id)
	return id
}

// seedCharacter inserts the designed character into all relevant tables.
func seedCharacter(c *DesignedCharacter) error {
	tx, err := dbpkg.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Look up race_id — exact match first, fuzzy fallback second.
	raceID := lookupID("races", "name", c.Race)
	if raceID == 0 {
		log.Printf("seedCharacter: race %q not found in DB, leaving NULL", c.Race)
	}

	// Look up background_id
	backgroundID := lookupID("backgrounds", "name", c.Background)
	if backgroundID == 0 {
		log.Printf("seedCharacter: background %q not found in DB, leaving NULL", c.Background)
	}

	// Look up class_id and hit die so we can compute accurate max HP.
	classID := lookupID("classes", "name", c.Class)
	var hitDie int
	if classID > 0 {
		dbpkg.DB.QueryRow(`SELECT hit_die FROM classes WHERE id = ?`, classID).Scan(&hitDie)
	}
	if classID == 0 {
		log.Printf("seedCharacter: class %q not found, skipping class row", c.Class)
	}

	// Max HP = hit die * level (taking max at every level, no con modifier
	// since ability scores start at 10 and therefore +0).
	// Falls back to d8 if the class lookup failed.
	if hitDie == 0 {
		hitDie = 8
	}
	maxHP := hitDie * c.Level

	// Proficiency bonus derived from level (standard 5e table).
	profBonus := proficiencyBonus(c.Level)

	var raceArg, bgArg interface{}
	if raceID > 0 {
		raceArg = raceID
	}
	if backgroundID > 0 {
		bgArg = backgroundID
	}

	res, err := tx.Exec(`
		INSERT INTO characters
			(name, race_id, background_id, alignment, level,
			 hp, max_hp, temp_hp, armor_class, speed, proficiency_bonus,
			 strength, dexterity, constitution, intelligence, wisdom, charisma)
		VALUES (?, ?, ?, ?, ?,
		        ?, ?, 0, 10, 30, ?,
		        10, 10, 10, 10, 10, 10)`,
		c.Name, raceArg, bgArg, c.Alignment, c.Level,
		maxHP, maxHP, profBonus,
	)
	if err != nil {
		return fmt.Errorf("insert character: %w", err)
	}
	charID, _ := res.LastInsertId()

	// Insert class
	if classID > 0 {
		_, err = tx.Exec(`
			INSERT INTO character_classes (character_id, class_id, level, is_primary)
			VALUES (?, ?, ?, 1)`,
			charID, classID, c.Level,
		)
		if err != nil {
			log.Printf("seedCharacter: insert class: %v", err)
		}
	}

	// Insert subclass — mandatory at level 3.
	if classID > 0 && c.Subclass != "" {
		var subclassID int
		tx.QueryRow(
			`SELECT id FROM subclasses WHERE LOWER(name) = LOWER(?) AND class_id = ?`,
			c.Subclass, classID,
		).Scan(&subclassID)
		if subclassID == 0 {
			// Fuzzy fallback
			tx.QueryRow(
				`SELECT id FROM subclasses WHERE class_id = ? AND (LOWER(name) LIKE LOWER(?) OR LOWER(?) LIKE '%' || LOWER(name) || '%') LIMIT 1`,
				classID, "%"+c.Subclass+"%", c.Subclass,
			).Scan(&subclassID)
		}
		if subclassID > 0 {
			tx.Exec(
				`UPDATE character_classes SET subclass_id = ? WHERE character_id = ? AND class_id = ?`,
				subclassID, charID, classID,
			)
		} else {
			log.Printf("seedCharacter: subclass %q not found for class %q", c.Subclass, c.Class)
		}
	}

	// Insert personality
	_, err = tx.Exec(`
		INSERT INTO character_personality (character_id, traits, ideals, bonds, flaws)
		VALUES (?, ?, '', '', '')`,
		charID, c.Personality,
	)
	if err != nil {
		log.Printf("seedCharacter: insert personality: %v", err)
	}

	// Insert backstory
	_, err = tx.Exec(`
		INSERT INTO character_backstory (character_id, backstory)
		VALUES (?, ?)`,
		charID, c.Backstory,
	)
	if err != nil {
		log.Printf("seedCharacter: insert backstory: %v", err)
	}

	// Insert public persona
	_, err = tx.Exec(`
		INSERT INTO character_public_persona (character_id, persona)
		VALUES (?, ?)`,
		charID, c.PublicFace,
	)
	if err != nil {
		log.Printf("seedCharacter: insert persona: %v", err)
	}

	// Insert corruption arc
	_, err = tx.Exec(`
		INSERT INTO character_corruption_arc (character_id, stage, notes, triggered_at)
		VALUES (?, 'nascent', ?, datetime('now'))`,
		charID, c.TrueGoal,
	)
	if err != nil {
		log.Printf("seedCharacter: insert corruption arc: %v", err)
	}

	// Insert primary goal
	_, err = tx.Exec(`
		INSERT INTO character_goals (character_id, goal, priority, status)
		VALUES (?, ?, 1, 'active')`,
		charID, c.TrueGoal,
	)
	if err != nil {
		log.Printf("seedCharacter: insert goal: %v", err)
	}

	// Insert spells known
	for _, spellName := range c.Spells {
		var spellID int
		err := tx.QueryRow(`SELECT id FROM spells WHERE LOWER(name) = LOWER(?)`, spellName).Scan(&spellID)
		if err != nil {
			log.Printf("seedCharacter: spell %q not found, skipping", spellName)
			continue
		}
		tx.Exec(`
			INSERT OR IGNORE INTO character_spells_known (character_id, spell_id, prepared, always_prepared)
			VALUES (?, ?, 1, 0)`,
			charID, spellID,
		)
	}

	// Insert feats
	for _, featName := range c.Feats {
		var featID int
		err := tx.QueryRow(`SELECT id FROM feats WHERE LOWER(name) = LOWER(?)`, featName).Scan(&featID)
		if err != nil {
			log.Printf("seedCharacter: feat %q not found, skipping", featName)
			continue
		}
		tx.Exec(`
			INSERT OR IGNORE INTO character_feats (character_id, feat_id, source)
			VALUES (?, ?, 'design')`,
			charID, featID,
		)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	log.Printf("seedCharacter: inserted character id=%d name=%q level=%d hp=%d race_id=%d background_id=%d class_id=%d subclass=%q",
		charID, c.Name, c.Level, maxHP, raceID, backgroundID, classID, c.Subclass)
	return nil
}

// proficiencyBonus returns the standard 5e proficiency bonus for a given level.
func proficiencyBonus(level int) int {
	switch {
	case level >= 17:
		return 6
	case level >= 13:
		return 5
	case level >= 9:
		return 4
	case level >= 5:
		return 3
	default:
		return 2
	}
}
