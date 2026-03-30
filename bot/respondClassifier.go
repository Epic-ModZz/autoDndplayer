package bot

import (
	dbpkg "PCL/db/SQL_CharStats"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// characterContext is a lightweight snapshot of the character used to ground
// the classifier. It is cached so we don't hit the DB on every message.
type characterContext struct {
	Name        string
	Race        string
	Class       string
	Level       int
	Alignment   string
	Persona     string // public face — what other players see
	Personality string // traits line
	Stage       string // corruption arc stage
}

var (
	cachedCharCtx   *characterContext
	charCtxMu       sync.RWMutex
	charCtxLoadedAt time.Time
	charCtxTTL      = 5 * time.Minute
)

// loadCharacterContext fetches a fresh snapshot from the DB and caches it.
// Falls back to a minimal stub if the DB isn't ready or the row doesn't exist yet.
func loadCharacterContext() *characterContext {
	charCtxMu.Lock()
	defer charCtxMu.Unlock()

	if cachedCharCtx != nil && time.Since(charCtxLoadedAt) < charCtxTTL {
		return cachedCharCtx
	}

	ctx := &characterContext{}

	if dbpkg.DB == nil {
		return ctx
	}

	// Core character info joined with race and primary class.
	row := dbpkg.DB.QueryRow(`
		SELECT
			c.name,
			COALESCE((SELECT name FROM races WHERE id = c.race_id), ''),
			COALESCE(
				(SELECT cl.name FROM character_classes cc
				 JOIN classes cl ON cl.id = cc.class_id
				 WHERE cc.character_id = c.id AND cc.is_primary = 1
				 LIMIT 1), ''
			),
			c.level,
			''
		FROM characters c
		WHERE c.id = 1
	`)
	if err := row.Scan(&ctx.Name, &ctx.Race, &ctx.Class, &ctx.Level, &ctx.Alignment); err != nil {
		log.Printf("loadCharacterContext: core query: %v", err)
	}

	// Public persona — what other characters perceive.
	dbpkg.DB.QueryRow(
		`SELECT persona FROM character_public_persona WHERE character_id = 1`,
	).Scan(&ctx.Persona)

	// Personality traits — gives the classifier a feel for how the character engages.
	dbpkg.DB.QueryRow(
		`SELECT traits FROM character_personality WHERE character_id = 1`,
	).Scan(&ctx.Personality)

	// Corruption arc stage — informs tone and engagement style.
	dbpkg.DB.QueryRow(
		`SELECT stage FROM character_corruption_arc WHERE character_id = 1`,
	).Scan(&ctx.Stage)

	cachedCharCtx = ctx
	charCtxLoadedAt = time.Now()
	return ctx
}

// InvalidateCharacterContext clears the cache so the next classifier call
// fetches fresh data. Call this after any write that updates the character.
func InvalidateCharacterContext() {
	charCtxMu.Lock()
	cachedCharCtx = nil
	charCtxMu.Unlock()
}

// buildCharacterBlock formats the character snapshot into the classifier prompt section.
func buildCharacterBlock(ctx *characterContext) string {
	if ctx.Name == "" {
		return "Character: unknown (DB not seeded yet)"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Name: %s\n", ctx.Name))
	if ctx.Race != "" || ctx.Class != "" {
		sb.WriteString(fmt.Sprintf("Race/Class: %s %s (Level %d)\n", ctx.Race, ctx.Class, ctx.Level))
	}
	if ctx.Alignment != "" {
		sb.WriteString(fmt.Sprintf("Alignment: %s\n", ctx.Alignment))
	}
	if ctx.Persona != "" {
		sb.WriteString(fmt.Sprintf("How others see them: %s\n", ctx.Persona))
	}
	if ctx.Personality != "" {
		sb.WriteString(fmt.Sprintf("Personality: %s\n", ctx.Personality))
	}
	if ctx.Stage != "" {
		sb.WriteString(fmt.Sprintf("Corruption stage: %s\n", ctx.Stage))
	}
	return strings.TrimSpace(sb.String())
}

func shouldRespondWithLLM(job *MessageJob) (bool, error) {
	charCtx := loadCharacterContext()

	var context strings.Builder
	for i := len(job.Messages) - 1; i >= 0; i-- {
		m := job.Messages[i]
		context.WriteString(fmt.Sprintf("%s: %s\n", m.Author.Username, m.Content))
	}

	channelMode := job.Mode.String()

	// Prefer the DB character name; fall back to the env var.
	characterName := charCtx.Name
	if characterName == "" {
		characterName = CharacterName
	}

	// The player persona name is how others address the bot OOC.
	playerName := PlayerName
	if playerName == "" {
		playerName = characterName
	}

	// Build a comma-separated list of all names and nicknames the bot answers
	// to. This is passed verbatim into the prompt so the LLM can do fuzzy
	// matching (e.g. "doog" matching "Dugal", "Doog" as a nickname, etc.)
	// rather than failing on case or truncation differences.
	//
	// Start with the two canonical names, deduplicate, then append any
	// additional nicknames stored in the public persona field.
	seen := map[string]bool{}
	var names []string
	for _, n := range []string{characterName, playerName} {
		if n != "" && !seen[strings.ToLower(n)] {
			seen[strings.ToLower(n)] = true
			names = append(names, n)
		}
	}
	// Persona often contains phrasing like "Known as Doog to friends" — include
	// it as free text so the classifier can pick out nicknames from it.
	personaHint := ""
	if charCtx.Persona != "" {
		personaHint = fmt.Sprintf("\nPublic persona / how others know them: %s", charCtx.Persona)
	}
	nameList := strings.Join(names, ", ")

	var prompt string
	if job.Mode == ChannelModeOOC {
		prompt = fmt.Sprintf(`A Discord bot participates in an OOC channel as a real person.
Known names / aliases: %s%s

Should this bot respond to the following message?

Recent conversation:
%s

Latest message from %s: "%s"

Reply YES if any of these are true:
- The message uses any of the known names or a plausible nickname/shortening of them
- The message asks about this person's D&D character (build, backstory, abilities, etc.)
- The message is a D&D rules or mechanics question addressed to the channel
- The recent conversation makes clear this message is a natural continuation directed at this person

Reply NO only if the message is clearly directed at someone else or is off-topic chatter with no game relevance.

Reply with only YES or NO.`,
			nameList,
			personaHint,
			context.String(),
			job.Message.Author.Username,
			job.Message.Content,
		)
	} else {
		prompt = fmt.Sprintf(`A Discord bot plays a D&D character in an active scene.
Known names / aliases: %s%s

Channel type: %s

Recent conversation:
%s

Latest message from %s: "%s"

Reply YES if the message:
- Uses any of the known names or a plausible nickname/shortening of them
- Is a direct question or action aimed at this character
- Is a roleplay action this character would naturally react to given the scene

Reply NO if the message is clearly directed at a different character or is background chatter this character has no reason to engage with.

Reply with only YES or NO.`,
			nameList,
			personaHint,
			channelMode,
			context.String(),
			job.Message.Author.Username,
			job.Message.Content,
		)
	}

	response, err := Query(prompt, ClassifierConfig())
	if err != nil {
		return false, err
	}
	log.Printf("shouldRespondWithLLM raw response: %q", response)
	return strings.HasPrefix(strings.TrimSpace(strings.ToUpper(response)), "YES"), nil
}
