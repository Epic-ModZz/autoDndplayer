package bot

import (
	dbpkg "PCL/db/SQL_CharStats"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// LevelUpDecisions is the structured output the LLM produces.
// Every field is optional — the LLM only populates what applies at this level.
type LevelUpDecisions struct {
	// HitPoints is the HP increase. We always take max for the bot
	// so this is deterministic, but the LLM confirms it.
	HitPoints int `json:"hit_points"`

	// ASI holds ability score increases if the character chose ASI over a feat.
	// Map of ability name -> increase amount, e.g. {"Charisma": 2} or {"Strength": 1, "Dexterity": 1}
	ASI map[string]int `json:"asi,omitempty"`

	// Feat is the feat name if the character chose a feat over ASI.
	Feat string `json:"feat,omitempty"`

	// Subclass is the subclass name if this level grants a subclass choice.
	Subclass string `json:"subclass,omitempty"`

	// SpellsLearned lists spell names the character is adding to spells known.
	SpellsLearned []string `json:"spells_learned,omitempty"`

	// SpellsSwapped maps old spell name -> new spell name for classes that
	// replace a known spell on level-up (e.g. Sorcerer, Ranger).
	SpellsSwapped map[string]string `json:"spells_swapped,omitempty"`

	// ExtraChoices holds anything that doesn't fit the above — eldritch
	// invocation swaps, fighting style, ranger favored enemy, etc.
	// Key is a short label, value is the chosen option.
	ExtraChoices map[string]string `json:"extra_choices,omitempty"`
}

// LevelUpResult bundles decisions with the LLM's reasoning so we can
// store both and show the human a full picture.
type LevelUpResult struct {
	Decisions LevelUpDecisions `json:"decisions"`
	Reasoning string           `json:"reasoning"`
}

// HandleLevelUp is called immediately after a level-up is confirmed by Avrae.
// It gathers character state, asks the LLM to deliberate on all choices,
// stores the result, and posts a summary to the admin channel.
func HandleLevelUp(s *discordgo.Session, newLevel int, questChannelID string) {
	log.Printf("Starting level-up deliberation for level %d", newLevel)

	// Pull everything we need from the DB for an informed decision.
	characterData, err := gatherLevelUpContext(newLevel)
	if err != nil {
		log.Println("HandleLevelUp: failed to gather character context:", err)
		return
	}

	// Ask the LLM to deliberate.
	result, err := deliberateLevelUp(newLevel, characterData)
	if err != nil {
		log.Println("HandleLevelUp: LLM deliberation failed:", err)
		return
	}

	// Persist decisions to DB.
	if err := storeLevelUpDecisions(newLevel, result); err != nil {
		log.Println("HandleLevelUp: failed to store decisions:", err)
		return
	}

	// Post the human-readable summary to the admin channel.
	adminChannelID := os.Getenv("adminChannelID")
	if adminChannelID == "" {
		log.Println("HandleLevelUp: adminChannelID not set — skipping Discord notification")
		return
	}
	summary := formatLevelUpSummary(newLevel, result)
	if _, err := s.ChannelMessageSend(adminChannelID, summary); err != nil {
		log.Println("HandleLevelUp: failed to post level-up summary:", err)
	}
}

// gatherLevelUpContext runs targeted SQL queries to give the LLM everything
// it needs to make good decisions: class, current spells, feats, ability
// scores, corruption arc stage, goals, and available reference data.
func gatherLevelUpContext(newLevel int) (string, error) {
	if dbpkg.DB == nil {
		return "", fmt.Errorf("DB not initialized")
	}

	queries := []string{
		// Who are we and what class(es) do we have
		`SELECT c.name, c.level, r.name as race, b.name as background, c.alignment
		 FROM characters c
		 LEFT JOIN races r ON r.id = c.race_id
		 LEFT JOIN backgrounds b ON b.id = c.background_id
		 WHERE c.id = 1;`,

		// Class levels — needed to know which features unlock
		`SELECT cl.name as class, cc.level as class_level, cc.is_primary,
		        sc.name as subclass
		 FROM character_classes cc
		 JOIN classes cl ON cl.id = cc.class_id
		 LEFT JOIN subclasses sc ON sc.class_id = cc.class_id
		 WHERE cc.character_id = 1;`,

		// Ability scores (we need these to evaluate ASI options)
		// Derived from character table — adjust if you store them separately
		`SELECT * FROM characters WHERE id = 1;`,

		// Current feats — don't pick the same feat twice
		`SELECT f.name, f.description
		 FROM character_feats cf
		 JOIN feats f ON f.id = cf.feat_id
		 WHERE cf.character_id = 1;`,

		// Current spells known
		`SELECT s.name, s.level as spell_level, s.school, csk.prepared
		 FROM character_spells_known csk
		 JOIN spells s ON s.id = csk.spell_id
		 WHERE csk.character_id = 1
		 ORDER BY s.level;`,

		// Corruption arc — influences which spells and feats fit the narrative
		`SELECT stage, notes FROM character_corruption_arc WHERE character_id = 1;`,

		// Active goals and schemes — level-up choices should serve these
		`SELECT goal, priority, status FROM character_goals WHERE character_id = 1;`,
		`SELECT scheme, status, notes FROM character_schemes WHERE character_id = 1 AND status = 'active';`,

		// Eldritch invocations if warlock
		`SELECT ei.name, ei.description
		 FROM character_features cf
		 JOIN eldritch_invocations ei ON ei.name = cf.feature_id
		 WHERE cf.character_id = 1;`,

		// Available feats from reference (so the LLM can evaluate options)
		`SELECT name, prerequisite, description FROM feats ORDER BY name;`,

		// Class features unlocking at the new level
		fmt.Sprintf(`SELECT cl.name as class, cf.name as feature, cf.level, cf.description
		 FROM class_features cf
		 JOIN classes cl ON cl.id = cf.class_id
		 JOIN character_classes cc ON cc.class_id = cf.class_id
		 WHERE cc.character_id = 1 AND cf.level = %d;`, newLevel),

		// Subclass features unlocking at the new level
		fmt.Sprintf(`SELECT sc.name as subclass, scf.name as feature, scf.description
		 FROM subclass_features scf
		 JOIN subclasses sc ON sc.id = scf.subclass_id
		 JOIN character_classes cc ON cc.class_id = sc.class_id
		 WHERE cc.character_id = 1 AND scf.level = %d;`, newLevel),
	}

	var sb strings.Builder
	for _, q := range queries {
		rows, err := dbpkg.DB.Query(q)
		if err != nil {
			// Non-fatal — log and continue so one bad query doesn't kill the whole context
			sb.WriteString(fmt.Sprintf("-- query error: %v\n\n", err))
			continue
		}

		cols, _ := rows.Columns()
		sb.WriteString(strings.Join(cols, " | ") + "\n")

		for rows.Next() {
			values := make([]interface{}, len(cols))
			ptrs := make([]interface{}, len(cols))
			for i := range values {
				ptrs[i] = &values[i]
			}
			rows.Scan(ptrs...)
			parts := make([]string, len(cols))
			for i, v := range values {
				if v == nil {
					parts[i] = "NULL"
				} else {
					parts[i] = fmt.Sprintf("%v", v)
				}
			}
			sb.WriteString(strings.Join(parts, " | ") + "\n")
		}
		rows.Close()
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// deliberateLevelUp asks the LLM to think through every choice available
// at this level and return structured decisions with plain-English reasoning.
func deliberateLevelUp(newLevel int, characterData string) (*LevelUpResult, error) {
	// fence cannot appear inside a raw string literal — interpolate it instead.
	fence := "```"

	prompt := fmt.Sprintf(`You are making level-up decisions for a D&D 5e character who is secretly becoming a villain.
The character's choices should:
- Serve their active schemes and long-term goals
- Fit their corruption arc stage
- Be mechanically strong for their role
- Feel narratively consistent — a character sliding toward darkness picks differently than a hero would

## Character Data
%s

## Level Being Applied: %d

## Your Task
Decide ALL choices available at this level. Think through each one carefully before committing.

Consider:
1. **Hit Points**: always take max for this character (confirm the die value)
2. **ASI or Feat** (if available at this level): weigh feat utility against raw stat improvements given current scores and goals
3. **Subclass** (if level 3, or first level of a new class): choose based on the corruption arc and schemes
4. **Spells** (if the class learns new spells): pick spells that serve the character's agenda — control, deception, and information are priorities
5. **Spell swaps** (if the class allows replacing a known spell): only swap if a clearly better option serves current goals
6. **Invocation swaps or other choices**: evaluate against current build and schemes

First, write 2-3 sentences of reasoning per choice explaining WHY, referencing the corruption arc, goals, or schemes specifically.
Then output a JSON block in this exact format:

%sjson
{
  "decisions": {
    "hit_points": <number — the die max for this class>,
    "asi": {"AbilityName": <amount>},
    "feat": "<feat name or empty string>",
    "subclass": "<subclass name or empty string>",
    "spells_learned": ["spell name", ...],
    "spells_swapped": {"old spell": "new spell"},
    "extra_choices": {"choice label": "chosen option"}
  },
  "reasoning": "<full plain-English explanation consolidating all reasoning>"
}
%s

Only include fields that are actually relevant at this level. Omit empty ones.
If a choice is not available at this level, omit that field entirely.`,
		characterData,
		newLevel,
		fence,
		fence,
	)

	// Use the roleplay model — we want creative, character-aware reasoning,
	// not just mechanical optimization.
	config := RoleplayConfig()
	config.Temperature = 0.4 // some creativity, but decisions should be deliberate
	config.NumCtx = 8192

	response, err := Query(prompt, config)
	if err != nil {
		return nil, fmt.Errorf("level-up LLM call failed: %w", err)
	}

	// Extract the JSON block
	cleaned := extractJSONBlock(response)
	if cleaned == "" {
		return nil, fmt.Errorf("no JSON block found in level-up response:\n%s", response)
	}

	var result LevelUpResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("failed to parse level-up decisions: %w\nraw: %s", err, cleaned)
	}

	return &result, nil
}

// storeLevelUpDecisions persists the full decision blob and reasoning to the DB.
func storeLevelUpDecisions(newLevel int, result *LevelUpResult) error {
	decisionsJSON, err := json.Marshal(result.Decisions)
	if err != nil {
		return fmt.Errorf("failed to marshal decisions: %w", err)
	}

	return dbpkg.UpsertRow("character_levelup_decisions", map[string]interface{}{
		"character_id":   1,
		"new_level":      newLevel,
		"decisions_json": string(decisionsJSON),
		"reasoning":      result.Reasoning,
		"applied":        0,
	}, "new_level") // conflict on new_level so re-running overwrites stale decisions
}

// formatLevelUpSummary produces the Discord message that gets posted to the
// admin channel. It should be clear enough that a human can apply it to
// Dicecloud without needing to read the raw JSON.
func formatLevelUpSummary(newLevel int, result *LevelUpResult) string {
	var sb strings.Builder
	d := result.Decisions

	sb.WriteString(fmt.Sprintf("## 🎲 Level %d — AI Decisions\n", newLevel))
	sb.WriteString("*Apply these to the sheet manually. Reply with ✅ once done.*\n\n")

	sb.WriteString(fmt.Sprintf("**HP Increase:** +%d (max)\n", d.HitPoints))

	if len(d.ASI) > 0 {
		parts := make([]string, 0, len(d.ASI))
		for ability, amount := range d.ASI {
			parts = append(parts, fmt.Sprintf("%s +%d", ability, amount))
		}
		sb.WriteString(fmt.Sprintf("**ASI:** %s\n", strings.Join(parts, ", ")))
	}

	if d.Feat != "" {
		sb.WriteString(fmt.Sprintf("**Feat:** %s\n", d.Feat))
	}

	if d.Subclass != "" {
		sb.WriteString(fmt.Sprintf("**Subclass:** %s\n", d.Subclass))
	}

	if len(d.SpellsLearned) > 0 {
		sb.WriteString(fmt.Sprintf("**New Spells:** %s\n", strings.Join(d.SpellsLearned, ", ")))
	}

	if len(d.SpellsSwapped) > 0 {
		for old, new := range d.SpellsSwapped {
			sb.WriteString(fmt.Sprintf("**Swap:** %s → %s\n", old, new))
		}
	}

	if len(d.ExtraChoices) > 0 {
		for label, choice := range d.ExtraChoices {
			sb.WriteString(fmt.Sprintf("**%s:** %s\n", label, choice))
		}
	}

	sb.WriteString(fmt.Sprintf("\n**Reasoning:**\n%s\n", result.Reasoning))

	return sb.String()
}

// extractJSONBlock pulls the first ```json ... ``` block out of a response.
// Falls back to finding a raw { ... } if no fences are present.
func extractJSONBlock(s string) string {
	// Try fenced block first
	start := strings.Index(s, "```json")
	if start != -1 {
		start += len("```json")
		end := strings.Index(s[start:], "```")
		if end != -1 {
			return strings.TrimSpace(s[start : start+end])
		}
	}
	// Fall back to raw braces
	start = strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start != -1 && end != -1 && end > start {
		return strings.TrimSpace(s[start : end+1])
	}
	return ""
}
