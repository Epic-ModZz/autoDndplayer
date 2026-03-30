package bot

import (
	"fmt"
	"log"
	"strings"
)

// mutableSchema is a condensed description of every table the bot is allowed
// to write to. Reference/immutable tables are omitted — the bot should never
// modify spell descriptions, monster stats, etc.
const mutableSchema = `
-- Character core
characters (id, name, race_id, background_id, alignment, experience, level, hp, max_hp, temp_hp, armor_class, speed, proficiency_bonus, discord_user_id)
character_personality (id, character_id, traits, ideals, bonds, flaws)
character_backstory (id, character_id, backstory)
character_notes (id, character_id, note, knowledge_source, created_at)
  -- knowledge_source: 'ic' | 'ooc' | 'dm'

-- Public face & deception
character_public_persona (id, character_id, persona)
character_lies (id, character_id, lie, target, revealed)

-- Villain mechanics
character_corruption_arc (id, character_id, stage, notes, triggered_at)
character_goals (id, character_id, goal, priority, status)
character_schemes (id, character_id, scheme, status, notes)
character_agents (id, character_id, agent_name, loyalty, role, location, notes)
character_trigger_events (id, character_id, trigger, response, last_fired)

-- Combat state
character_conditions (id, character_id, condition_name, duration, source)
character_exhaustion (id, character_id, level)
character_concentration (id, character_id, spell_id, duration_remaining)
character_death_saves (id, character_id, successes, failures, stable)
character_hit_dice (id, character_id, die_type, remaining, maximum)
character_inspiration (id, character_id, has_inspiration)

-- Resources
character_class_resources (id, character_id, resource_name, current, maximum, recharge_on)
character_spell_slots (id, character_id, slot_level, remaining, maximum)
character_spells_known (id, character_id, spell_id, prepared, always_prepared)

-- Inventory
character_inventory (id, character_id, item_name, item_type, quantity, equipped, attunement, notes)
character_currency (id, character_id, pp, gp, ep, sp, cp)

-- World standing
character_relationships (id, character_id, npc_id, relationship_type, trust_level, notes)
character_faction_standing (id, character_id, faction_name, standing, rank, notes)
character_quest_log (id, character_id, quest_name, status, giver, notes, updated_at)

-- NPCs
npc_details (id, name, race, role, location, disposition, alive, notes, discovered_ic)
  -- discovered_ic: 1 = character met them IC, 0 = player knows via OOC/DM only
npc_secrets (id, npc_id, secret, known_by, revealed, discovered_ic)
  -- discovered_ic: 1 = character learned this IC, 0 = player only

-- Session history
session_log (id, session_date, summary, participants, dm_notes)
character_session_stats (id, character_id, session_id, kills, damage_dealt, healing_done, knocks, deaths)

-- Players
discord_users (id, discord_user_id, username, display_name, is_dm, is_bot, timezone, joined_at, first_seen_at, last_seen_at)
`

// WriteMemory is called after every response is sent. It analyzes the full
// scene and writes any new or changed information back to the database.
// Runs in a goroutine — never blocks the response pipeline.
func WriteMemory(job *MessageJob, botResponse string, dbContext string) {
	mutations, err := generateMemoryWrites(job, botResponse, dbContext)
	if err != nil {
		log.Println("WriteMemory: LLM call failed:", err)
		return
	}
	if mutations == "" {
		return
	}

	results, err := RunMutations(mutations)
	if err != nil {
		log.Println("WriteMemory: RunMutations error:", err)
		return
	}

	for _, r := range results {
		if r.Error != "" {
			log.Printf("WriteMemory: mutation error — %s\n  stmt: %s", r.Error, r.Statement)
		} else {
			log.Printf("WriteMemory: wrote %d row(s) — %s", r.RowsAffected, summariseStmt(r.Statement))
		}
	}
}

// generateMemoryWrites asks the LLM what should be persisted from this scene.
func generateMemoryWrites(job *MessageJob, botResponse string, dbContext string) (string, error) {
	var convo strings.Builder
	for i := len(job.Messages) - 1; i >= 0; i-- {
		m := job.Messages[i]
		convo.WriteString(fmt.Sprintf("%s: %s\n", m.Author.Username, m.Content))
	}

	fence := "```"

	prompt := fmt.Sprintf(`You are the memory system for a D&D character bot.
Your job is to decide what new information from this scene should be saved to the database,
and write the SQL statements to do it.

## Mutable Tables (the only tables you may write to)
%s

## What Was Already Known (current DB context)
%s

## Scene (recent conversation)
%s

## Latest Message
%s: "%s"

## Bot's Response
"%s"

## SQLite Rules (this is SQLite, not MySQL — follow these exactly)
- Use datetime('now') instead of NOW()
- Do NOT use last_insert_rowid() as a variable or with INTO — SQLite doesn't support that
- To reference a just-inserted NPC's id in a follow-up INSERT, use a subquery: (SELECT id FROM npc_details WHERE name = 'NPC Name' ORDER BY id DESC LIMIT 1)
- Use INSERT OR IGNORE or INSERT OR REPLACE instead of INSERT ... ON DUPLICATE KEY UPDATE
- Never use backtick identifiers — use double quotes or no quotes

## The Bot's Own Character
character_id = 1 belongs to the bot's character only.
Other players' characters mentioned in conversation are NOT character_id = 1.
Other players' characters should ONLY be stored as NPCs in npc_details (with discovered_ic = 0),
never written into character_*, character_spells_known, character_inventory, etc.

## Instructions
Review the scene and the bot's response. Write INSERT or UPDATE statements for anything that:
- Introduces a new NPC or other player's character the bot has not seen before (npc_details only)
- Changes an existing NPC's disposition, location, alive status, or adds a secret
- Updates a relationship between the bot's character (id=1) and an NPC or faction
- Advances or changes a quest (new quest, status update, completion)
- Reveals or creates a new lie, scheme, goal, or agent for the bot's character
- Changes the bot's character's corruption arc stage or adds a note to it
- Adds a character note about something worth remembering (knowledge_source = 'ooc' for OOC scenes)

Do NOT write statements for:
- Things already accurately reflected in the current DB context
- Temporary combat state (conditions, spell slots, HP) unless something clearly changed
- Anything speculative — only record what actually happened in the scene
- Other players' character data into character_* tables — only npc_details

For new NPCs or other player characters, INSERT into npc_details first, then use
last_insert_rowid() to reference the new id in character_relationships.
Use INSERT OR IGNORE for rows that may already exist.

Output ONLY the SQL statements inside %ssql code blocks.
If nothing needs to be written, output nothing at all.`,
		mutableSchema,
		dbContext,
		convo.String(),
		job.Message.Author.Username,
		job.Message.Content,
		botResponse,
		fence,
	)

	config := SQLConfig()
	config.Temperature = 0.1
	config.NumCtx = 8192

	return Query(prompt, config)
}

// summariseStmt returns a short description of a SQL statement for logging.
func summariseStmt(stmt string) string {
	fields := strings.Fields(stmt)
	if len(fields) < 2 {
		return stmt
	}
	verb := strings.ToUpper(fields[0])
	switch verb {
	case "INSERT":
		if len(fields) >= 3 {
			return fmt.Sprintf("INSERT INTO %s", fields[2])
		}
	case "UPDATE":
		return fmt.Sprintf("UPDATE %s", fields[1])
	case "DELETE":
		if len(fields) >= 3 {
			return fmt.Sprintf("DELETE FROM %s", fields[2])
		}
	}
	n := 60
	if len(stmt) < n {
		n = len(stmt)
	}
	return stmt[:n]
}

// dmMutableSchema is a narrower schema focused on what's actually learnable
// from a private conversation with a player. Reference tables and combat state
// are excluded — DMs are about the person and their character's story, not
// their spell slots.
const dmMutableSchema = `
-- Players (timezone, display name, DM status — often revealed in casual DMs)
discord_users (id, discord_user_id, username, display_name, is_dm, is_bot, timezone, joined_at, first_seen_at, last_seen_at)

-- Character sheets linked to this player
character_sheets (id, discord_user_id, character_id, sheet_type, sheet_id, created_at, updated_at)

-- Character notes — OOC things the player says about their character
character_notes (id, character_id, note, knowledge_source, created_at)
  -- knowledge_source: 'ic' | 'ooc' | 'dm'

-- Backstory and personality — players often reveal this OOC before it shows up IC
character_backstory (id, character_id, backstory)
character_personality (id, character_id, traits, ideals, bonds, flaws)

-- Goals and schemes — a player hinting at their character's plans OOC
character_goals (id, character_id, goal, priority, status)
character_schemes (id, character_id, scheme, status, notes)

-- Relationships — player mentions how their character feels about an NPC or faction
character_relationships (id, character_id, npc_id, relationship_type, trust_level, notes)
character_faction_standing (id, character_id, faction_name, standing, rank, notes)

-- NPCs the player mentions or asks about
npc_details (id, name, race, role, location, disposition, alive, notes, discovered_ic)
  -- discovered_ic: 1 = character met them IC, 0 = player knows via OOC/DM only
npc_secrets (id, npc_id, secret, known_by, revealed, discovered_ic)
  -- discovered_ic: 1 = character learned this IC, 0 = player only

-- Quest context — player asking about or referencing a quest
character_quest_log (id, character_id, quest_name, status, giver, notes, updated_at)

-- Lies the character is maintaining — player may reference these OOC
character_lies (id, character_id, lie, target, revealed)
`

// WriteDMMemory is the DM-channel equivalent of WriteMemory.
// It runs after every player DM response and persists anything useful the
// player revealed about themselves, their character, or the world.
// Runs in a goroutine — never blocks the DM response.
func WriteDMMemory(job *MessageJob, botResponse string, dbContext string) {
	mutations, err := generateDMMemoryWrites(job, botResponse, dbContext)
	if err != nil {
		log.Println("WriteDMMemory: LLM call failed:", err)
		return
	}
	if mutations == "" {
		return
	}

	results, err := RunMutations(mutations)
	if err != nil {
		log.Println("WriteDMMemory: RunMutations error:", err)
		return
	}

	for _, r := range results {
		if r.Error != "" {
			log.Printf("WriteDMMemory [%s]: mutation error — %s\n  stmt: %s",
				job.Message.Author.Username, r.Error, r.Statement)
		} else {
			log.Printf("WriteDMMemory [%s]: wrote %d row(s) — %s",
				job.Message.Author.Username, r.RowsAffected, summariseStmt(r.Statement))
		}
	}
}

// generateDMMemoryWrites asks the LLM what to persist from a private
// conversation. The prompt is tuned for player-level information — the kinds
// of things people reveal casually in DMs that never make it into IC channels.
func generateDMMemoryWrites(job *MessageJob, botResponse string, dbContext string) (string, error) {
	var convo strings.Builder
	for i := len(job.Messages) - 1; i >= 0; i-- {
		m := job.Messages[i]
		convo.WriteString(fmt.Sprintf("%s: %s\n", m.Author.Username, m.Content))
	}

	fence := "```"

	prompt := fmt.Sprintf(`You are the memory system for a D&D character bot.
You are reviewing a private message conversation between the bot and a player.
Your job is to extract anything useful the player revealed and write SQL to persist it.

## Player
Discord username: %s
Discord user ID:  %s

## Tables You May Write To
%s

## What Was Already Known
%s

## DM Conversation
%s

## Bot's Response
"%s"

## What to look for in DMs specifically
Persist information when the player:
- Mentions their timezone or availability ("I'm in EST", "I'm usually online evenings")
- Reveals something about their character's personality, backstory, or motivation that isn't in the DB yet
- Hints at their character's plans, goals, or relationships with NPCs or factions
- Mentions another player's character in a way that reveals something about that relationship
- Asks about an NPC in a way that implies their character has a specific relationship with them
- Confirms or denies something about their character ("yeah my character definitely wouldn't trust X")
- Mentions their character sheet type or ID (dicecloud, dndbeyond link, etc.)
- Reveals an NPC detail or secret they know about from a previous session

Do NOT write statements for:
- Small talk with no game-relevant content ("haha yeah", "sounds fun", "thanks")
- Things already accurately in the DB
- Anything the player said hypothetically or speculatively
- Other players' private information — only record what the sender revealed about themselves

For discord_users: use discord_user_id = '%s' and username = '%s'.
For character rows: look up character_id by discord_user_id if needed, or use a subquery.
For character_notes rows, always set knowledge_source = 'dm'.
For new NPCs mentioned in DMs, set discovered_ic = 0 — the character has not met them IC yet.
For npc_secrets learned via DM, set discovered_ic = 0.
Use INSERT OR REPLACE or ON CONFLICT DO UPDATE where the row may already exist.

Output ONLY SQL inside %ssql code blocks.
If nothing is worth remembering, output nothing at all.`,
		job.Message.Author.Username,
		job.Message.Author.ID,
		dmMutableSchema,
		dbContext,
		convo.String(),
		botResponse,
		job.Message.Author.ID,
		job.Message.Author.Username,
		fence,
	)

	config := SQLConfig()
	config.Temperature = 0.1
	config.NumCtx = 8192

	return Query(prompt, config)
}
