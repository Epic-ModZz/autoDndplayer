package bot

import (
	dbpkg "PCL/db/SQL_CharStats"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// adminUserID is the Discord user ID allowed to send DM commands.
// Set via the ADMIN_USER_ID environment variable.
var adminUserID = os.Getenv("ADMIN_USER_ID")

// commandHandler is the function signature every command implements.
type commandHandler func(s *discordgo.Session, m *discordgo.MessageCreate, args []string) string

// commands is the registry of all available DM commands.
// Key is the command name without the ! prefix.
var commands = map[string]commandHandler{
	"help":       cmdHelp,
	"levelup":    cmdLevelUp,
	"status":     cmdStatus,
	"goals":      cmdGoals,
	"note":       cmdNote,
	"cooldown":   cmdCooldown,
	"reclassify": cmdReclassify,
	"channels":   cmdChannels,
	"sql":        cmdSQL,
}

// helpText is shown by !help and on unknown commands.
var helpText = strings.TrimSpace(`
**Available commands:**
` + "```" + `
!help                          — this message
!levelup pending               — show unapplied level-up decisions
!levelup applied               — mark current level-up decisions as applied
!levelup reroll                — re-run AI deliberation for the pending level

!status                        — HP, conditions, spell slots, corruption stage
!goals                         — active goals and schemes

!note <text>                   — add an OOC note to character_notes

!cooldown clear <channelID>    — clear the response cooldown for a channel
!reclassify <channelID>        — force channel reclassification
!channels                      — list all cached channel classifications

!sql <statement>               — execute raw SQL (SELECT, INSERT, UPDATE, DELETE)
` + "```")

// HandleDMCommand is the entry point for all DMs.
// Admin + "!" prefix → admin command.
// Everyone else (or admin without "!") → OOC player conversation.
func HandleDMCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	content := strings.TrimSpace(m.Content)

	isAdmin := adminUserID != "" && m.Author.ID == adminUserID
	isCommand := strings.HasPrefix(content, "!")

	if !isAdmin || !isCommand {
		// Any non-command DM — respond as an ordinary OOC player.
		go handlePlayerDM(s, m)
		return
	}

	// Admin command path.
	parts := strings.Fields(strings.TrimPrefix(content, "!"))
	if len(parts) == 0 {
		return
	}

	name := strings.ToLower(parts[0])
	args := parts[1:]

	handler, ok := commands[name]
	if !ok {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❓ Unknown command `!%s`.\n\n%s", name, helpText))
		return
	}

	reply := handler(s, m, args)
	if reply != "" {
		s.ChannelMessageSend(m.ChannelID, reply)
	}
}

// ---------------------------------------------------------------------------
// Command implementations
// ---------------------------------------------------------------------------

func cmdHelp(_ *discordgo.Session, _ *discordgo.MessageCreate, _ []string) string {
	return helpText
}

// !levelup pending | applied | reroll
func cmdLevelUp(s *discordgo.Session, m *discordgo.MessageCreate, args []string) string {
	if len(args) == 0 {
		return "Usage: `!levelup pending | applied | reroll`"
	}

	switch strings.ToLower(args[0]) {

	case "pending":
		return levelUpPending()

	case "applied":
		return levelUpApplied()

	case "reroll":
		return levelUpReroll(s)

	default:
		return "Usage: `!levelup pending | applied | reroll`"
	}
}

func levelUpPending() string {
	if dbpkg.DB == nil {
		return "❌ DB not initialized."
	}

	rows, err := dbpkg.DB.Query(`
		SELECT new_level, decisions_json, reasoning, created_at
		FROM character_levelup_decisions
		WHERE character_id = 1 AND applied = 0
		ORDER BY new_level DESC
		LIMIT 1;`)
	if err != nil {
		return fmt.Sprintf("❌ Query error: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return "✅ No pending level-up decisions."
	}

	var newLevel int
	var decisionsJSON, reasoning, createdAt string
	rows.Scan(&newLevel, &decisionsJSON, &reasoning, &createdAt)

	var d LevelUpDecisions
	json.Unmarshal([]byte(decisionsJSON), &d)

	return formatLevelUpSummary(newLevel, &LevelUpResult{Decisions: d, Reasoning: reasoning}) +
		fmt.Sprintf("\n*Recorded at: %s*", createdAt)
}

func levelUpApplied() string {
	if dbpkg.DB == nil {
		return "❌ DB not initialized."
	}

	result, err := dbpkg.DB.Exec(`
		UPDATE character_levelup_decisions
		SET applied = 1, applied_at = datetime('now')
		WHERE character_id = 1 AND applied = 0;`)
	if err != nil {
		return fmt.Sprintf("❌ Update error: %v", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return "ℹ️ No pending level-up decisions to mark as applied."
	}

	// Also clear the pending marker in character_pending_levelup
	dbpkg.DB.Exec(`UPDATE character_pending_levelup SET pending = 0 WHERE character_id = 1;`)

	return "✅ Level-up decisions marked as applied. Sheet is now considered current."
}

func levelUpReroll(s *discordgo.Session) string {
	if dbpkg.DB == nil {
		return "❌ DB not initialized."
	}

	row := dbpkg.DB.QueryRow(`
		SELECT new_level FROM character_levelup_decisions
		WHERE character_id = 1 AND applied = 0
		ORDER BY new_level DESC LIMIT 1;`)

	var newLevel int
	if err := row.Scan(&newLevel); err != nil {
		return "ℹ️ No pending level-up found to reroll."
	}

	go HandleLevelUp(s, newLevel, "")
	return fmt.Sprintf("🎲 Re-running AI deliberation for level %d. Check back shortly.", newLevel)
}

// !status
func cmdStatus(_ *discordgo.Session, _ *discordgo.MessageCreate, _ []string) string {
	if dbpkg.DB == nil {
		return "❌ DB not initialized."
	}

	var sb strings.Builder

	row := dbpkg.DB.QueryRow(`SELECT name, level, hp, max_hp, armor_class FROM characters WHERE id = 1;`)
	var name string
	var level, hp, maxHP, ac int
	if err := row.Scan(&name, &level, &hp, &maxHP, &ac); err != nil {
		sb.WriteString(fmt.Sprintf("⚠️ Could not load character: %v\n", err))
	} else {
		sb.WriteString(fmt.Sprintf("**%s** — Level %d | HP %d/%d | AC %d\n\n", name, level, hp, maxHP, ac))
	}

	rows, err := dbpkg.DB.Query(`SELECT condition_name, duration, source FROM character_conditions WHERE character_id = 1;`)
	if err == nil {
		defer rows.Close()
		var conditions []string
		for rows.Next() {
			var cond, duration, source string
			rows.Scan(&cond, &duration, &source)
			conditions = append(conditions, fmt.Sprintf("%s (duration: %s, from: %s)", cond, duration, source))
		}
		if len(conditions) > 0 {
			sb.WriteString("**Conditions:**\n")
			for _, c := range conditions {
				sb.WriteString(fmt.Sprintf("  • %s\n", c))
			}
			sb.WriteString("\n")
		} else {
			sb.WriteString("**Conditions:** None\n\n")
		}
	}

	rows2, err := dbpkg.DB.Query(`SELECT slot_level, remaining, maximum FROM character_spell_slots WHERE character_id = 1 ORDER BY slot_level;`)
	if err == nil {
		defer rows2.Close()
		var slots []string
		for rows2.Next() {
			var slotLevel, remaining, maximum int
			rows2.Scan(&slotLevel, &remaining, &maximum)
			slots = append(slots, fmt.Sprintf("L%d: %d/%d", slotLevel, remaining, maximum))
		}
		if len(slots) > 0 {
			sb.WriteString(fmt.Sprintf("**Spell Slots:** %s\n\n", strings.Join(slots, " | ")))
		}
	}

	row3 := dbpkg.DB.QueryRow(`SELECT stage, notes FROM character_corruption_arc WHERE character_id = 1;`)
	var stage, notes string
	if err := row3.Scan(&stage, &notes); err == nil {
		sb.WriteString(fmt.Sprintf("**Corruption Stage:** %s\n_%s_\n", stage, notes))
	}

	return sb.String()
}

// !goals
func cmdGoals(_ *discordgo.Session, _ *discordgo.MessageCreate, _ []string) string {
	if dbpkg.DB == nil {
		return "❌ DB not initialized."
	}

	var sb strings.Builder

	rows, err := dbpkg.DB.Query(`SELECT goal, priority, status FROM character_goals WHERE character_id = 1 ORDER BY priority;`)
	if err != nil {
		return fmt.Sprintf("❌ Query error: %v", err)
	}
	defer rows.Close()

	sb.WriteString("**Goals:**\n")
	any := false
	for rows.Next() {
		var goal, priority, status string
		rows.Scan(&goal, &priority, &status)
		sb.WriteString(fmt.Sprintf("  • [%s] %s (%s)\n", priority, goal, status))
		any = true
	}
	if !any {
		sb.WriteString("  None\n")
	}

	rows2, err := dbpkg.DB.Query(`SELECT scheme, status, notes FROM character_schemes WHERE character_id = 1 AND status = 'active';`)
	if err != nil {
		return sb.String()
	}
	defer rows2.Close()

	sb.WriteString("\n**Active Schemes:**\n")
	any = false
	for rows2.Next() {
		var scheme, status, notes string
		rows2.Scan(&scheme, &status, &notes)
		sb.WriteString(fmt.Sprintf("  • %s\n    _%s_\n", scheme, notes))
		any = true
	}
	if !any {
		sb.WriteString("  None\n")
	}

	return sb.String()
}

// !note <text>
// Inserts a new character note tagged as OOC knowledge (admin-added via DM,
// not something the character experienced in-fiction).
func cmdNote(_ *discordgo.Session, _ *discordgo.MessageCreate, args []string) string {
	if len(args) == 0 {
		return "Usage: `!note <text>`"
	}

	if dbpkg.DB == nil {
		return "❌ DB not initialized."
	}

	text := strings.Join(args, " ")
	now := time.Now().UTC().Format(time.RFC3339)

	// Use a direct INSERT rather than UpsertRow — notes are append-only
	// and have no natural unique key to conflict on.
	_, err := dbpkg.DB.Exec(
		`INSERT INTO character_notes (character_id, note, knowledge_source, created_at) VALUES (?, ?, 'ooc', ?)`,
		1, text, now,
	)
	if err != nil {
		return fmt.Sprintf("❌ Failed to save note: %v", err)
	}

	return fmt.Sprintf("✅ Note saved: _%s_", text)
}

// !cooldown clear <channelID>
func cmdCooldown(_ *discordgo.Session, _ *discordgo.MessageCreate, args []string) string {
	if len(args) < 2 || strings.ToLower(args[0]) != "clear" {
		return "Usage: `!cooldown clear <channelID>`"
	}

	channelID := args[1]
	cooldownMu.Lock()
	delete(cooldowns, channelID)
	cooldownMu.Unlock()

	return fmt.Sprintf("✅ Cooldown cleared for channel `%s`.", channelID)
}

// !reclassify <channelID>
func cmdReclassify(s *discordgo.Session, _ *discordgo.MessageCreate, args []string) string {
	if len(args) == 0 {
		return "Usage: `!reclassify <channelID>`"
	}

	channelID := args[0]

	channelModeCacheMu.Lock()
	delete(channelModeCache, channelID)
	channelModeCacheMu.Unlock()

	mode, err := classifyChannelWithLLM(s, channelID)
	if err != nil {
		return fmt.Sprintf("❌ Classification failed: %v", err)
	}

	channelModeCacheMu.Lock()
	channelModeCache[channelID] = mode
	channelModeCacheMu.Unlock()

	return fmt.Sprintf("✅ Channel `%s` reclassified as **%s**.", channelID, mode)
}

// !channels
func cmdChannels(_ *discordgo.Session, _ *discordgo.MessageCreate, _ []string) string {
	channelModeCacheMu.RLock()
	defer channelModeCacheMu.RUnlock()

	if len(channelModeCache) == 0 {
		return "ℹ️ No channels classified yet."
	}

	var sb strings.Builder
	sb.WriteString("**Classified channels:**\n```\n")
	for id, mode := range channelModeCache {
		sb.WriteString(fmt.Sprintf("%-20s %s\n", id, mode))
	}
	sb.WriteString("```")
	return sb.String()
}

// ---------------------------------------------------------------------------
// Player DM handler
// ---------------------------------------------------------------------------

// handlePlayerDM responds to DMs from non-admin players as if the bot is an
// ordinary OOC player — friendly, human-sounding, knowledgeable about the
// campaign but not breaking the illusion of being a real person.
func handlePlayerDM(s *discordgo.Session, m *discordgo.MessageCreate) {
	history, err := s.ChannelMessages(m.ChannelID, 10, "", "", "")
	if err != nil {
		history = nil
	}

	job := &MessageJob{
		Session:  s,
		Message:  m,
		Messages: history,
		Mode:     ChannelModeOOC,
	}

	ctx := ResponseContext{}

	sqlText, err := GatherInfo(job)
	if err == nil && sqlText != "" {
		queryResults, err := RunQueries(sqlText)
		if err == nil {
			formatted := FormatResultsForLLM(queryResults)
			if formatted != "" {
				knowledge, err := SummarizeDBResults(formatted, job)
				if err == nil && knowledge != nil {
					ctx.ICKnowledge = knowledge.IC
					ctx.OOCKnowledge = knowledge.OOC
					if knowledge.IC == "" && knowledge.OOC == "" {
						ctx.RawFallback = formatted
					}
				} else {
					ctx.RawFallback = formatted
				}
			}
		}
	}
	if sqlText == "" {
		ctx.ContextOnly = true
	}

	response, err := generateDMResponse(job, ctx)
	if err != nil {
		return
	}

	s.ChannelMessageSend(m.ChannelID, response)

	dbContext := ctx.ICKnowledge
	if dbContext == "" {
		dbContext = ctx.RawFallback
	}
	go WriteDMMemory(job, response, dbContext)
}

// generateDMResponse is a tighter version of GenerateOOCResponse tuned for
// private messages. The bot is posing as a regular player — casual, warm,
// never robotic, never mentioning it's a bot or an AI.
func generateDMResponse(job *MessageJob, ctx ResponseContext) (string, error) {
	var history strings.Builder
	for i := len(job.Messages) - 1; i >= 0; i-- {
		m := job.Messages[i]
		history.WriteString(fmt.Sprintf("%s: %s\n", m.Author.Username, m.Content))
	}

	dbSection := buildDBSection(ctx)

	prompt := fmt.Sprintf(`You are a player in a D&D westmarch Discord server talking to another player over private message.
You are not a bot. You are not an AI. You are just another member of the server having a casual conversation.
Never acknowledge being a bot, an AI, or anything other than a regular player.

## Recent DM History
%s

## Latest Message from %s
"%s"

%s

## Instructions
- Be warm, casual, and human — this is a private message, not a public channel
- Keep responses short — 1 to 3 sentences unless they asked something detailed
- If they are asking about game mechanics, lore, or character info, answer naturally using the retrieved data where relevant
- If the topic is personal or social (asking how you are, small talk), respond like a real person would
- Never use bullet points, headers, or any formatting that would look unnatural in a DM
- Never reveal you are an AI or that you have a database — you just "know" things the way any player would

Write only your reply, nothing else.`,
		history.String(),
		job.Message.Author.Username,
		job.Message.Content,
		dbSection,
	)

	config := RoleplayConfig()
	config.Temperature = 0.6
	return Query(prompt, config)
}

// ---------------------------------------------------------------------------
// !sql command
// ---------------------------------------------------------------------------

// cmdSQL executes a raw SQL statement against the database.
// SELECT statements return formatted results.
// All other statements (INSERT, UPDATE, DELETE, etc.) return rows affected.
func cmdSQL(_ *discordgo.Session, _ *discordgo.MessageCreate, args []string) string {
	if len(args) == 0 {
		return "Usage: `!sql <statement>`\nExample: `!sql SELECT * FROM characters`"
	}

	if dbpkg.DB == nil {
		return "❌ DB not initialized."
	}

	stmt := strings.Join(args, " ")
	keyword := strings.ToUpper(strings.Fields(stmt)[0])

	if keyword == "SELECT" {
		return execSQLQuery(stmt)
	}
	return execSQLMutation(stmt)
}

// execSQLQuery runs a SELECT and formats the results as a code block.
// Discord messages cap at 2000 characters — results are truncated with a
// notice rather than silently cut off.
func execSQLQuery(stmt string) string {
	rows, err := dbpkg.DB.Query(stmt)
	if err != nil {
		return fmt.Sprintf("❌ Query error:\n```\n%v\n```", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Sprintf("❌ Could not read columns: %v", err)
	}

	var sb strings.Builder
	sb.WriteString(strings.Join(cols, " | "))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("-", 40))
	sb.WriteString("\n")

	rowCount := 0
	for rows.Next() {
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			sb.WriteString(fmt.Sprintf("<scan error: %v>\n", err))
			continue
		}
		parts := make([]string, len(cols))
		for i, v := range values {
			if v == nil {
				parts[i] = "NULL"
			} else {
				parts[i] = fmt.Sprintf("%v", v)
			}
		}
		sb.WriteString(strings.Join(parts, " | "))
		sb.WriteString("\n")
		rowCount++
	}

	if rowCount == 0 {
		return "✅ Query returned no rows."
	}

	body := sb.String()
	suffix := fmt.Sprintf("\n_%d row(s)_", rowCount)

	wrapped := fmt.Sprintf("```\n%s```%s", body, suffix)
	if len(wrapped) > 1900 {
		cutAt := 1900 - len("```\n...(truncated)```") - len(suffix)
		wrapped = fmt.Sprintf("```\n%s\n...(truncated)```%s", body[:cutAt], suffix)
	}

	return wrapped
}

// execSQLMutation runs a non-SELECT statement and reports rows affected.
func execSQLMutation(stmt string) string {
	result, err := dbpkg.DB.Exec(stmt)
	if err != nil {
		return fmt.Sprintf("❌ Error:\n```\n%v\n```", err)
	}

	affected, _ := result.RowsAffected()
	return fmt.Sprintf("✅ OK — %d row(s) affected.", affected)
}
