package bot

import (
	"fmt"
	"strings"
)

// batchDescriptions gives the classifier a plain-English summary of each batch
// so it can decide relevance without seeing the full schema.
var batchDescriptions = map[string]string{
	"Character Identity & Roleplay":       "character name, race, level, HP, personality, backstory, public persona, lies, corruption arc, goals, schemes, notes",
	"Character Narrative Mechanics":       "trigger events, character agents, spies or informants the character controls, channel config",
	"Character Combat State":              "active conditions, exhaustion, concentration, death saves, hit dice, inspiration",
	"Character Resources & Spell Slots":   "class levels, class resources (ki, rage, etc.), spell slots remaining, spells known and prepared",
	"Character Abilities & Proficiencies": "class features, subclass features, feats, skill and tool proficiencies",
	"Character Inventory & Economy":       "items carried, equipped gear, currency (gp, sp, cp, etc.)",
	"Character World Standing":            "relationships with NPCs, faction standings, active and completed quests",
	"NPCs & Session Logs":                 "NPC details, NPC secrets, session history, per-session combat stats",
	"Class & Subclass Reference":          "class hit dice, saving throws, armor and weapon proficiencies, class features by level, spellcasting progression",
	"Race & Subrace Reference":            "racial traits, ability score increases, racial features, subraces",
	"Feats, Boons & Invocations":          "feat descriptions and prerequisites, boons, eldritch invocations, fighting styles",
	"Backgrounds Reference":               "background skill proficiencies, equipment, background features",
	"Conditions Reference":                "what each condition does mechanically (blinded, frightened, poisoned, etc.)",
	"Spells Reference":                    "spell descriptions, components, casting time, range, duration, spell slot tables",
	"Weapons & Armor Reference":           "weapon damage and properties, weapon masteries, armor AC values and requirements",
	"Items Reference":                     "mundane items, magic items and their abilities, potions, proficiencies, languages",
	"Monsters Reference":                  "monster stats, actions, traits, legendary actions, monster spellcasting",
	"Bastion & Stronghold Reference":      "stronghold tiers, facility types, facility benefits and upgrades",
	"Player Information":                  "time zones, list of player character sheets & the discord user who made them",
}

// FilterRelevantBatches uses a small fast model to decide which schema batches
// are relevant to the current roleplay context. Returns a filtered slice of SchemaBatch.
func FilterRelevantBatches(roleplay string) ([]SchemaBatch, error) {
	// Build the menu of batch names + descriptions for the classifier
	var menu strings.Builder
	for _, batch := range schemaBatches {
		desc, ok := batchDescriptions[batch.Name]
		if !ok {
			desc = batch.Name
		}
		menu.WriteString(fmt.Sprintf("- %s: %s\n", batch.Name, desc))
	}

	prompt := fmt.Sprintf(`You are a classifier for a D&D discord bot.

Given a roleplay scene, select ONLY the data categories needed to formulate a response.
Be selective — only include categories directly relevant to the scene.
Err on the side of fewer categories unless something in the scene clearly requires more.

## Available Categories
%s

## Roleplay Scene
%s

## Instructions
Reply with a newline-separated list of category names exactly as written above.
Include only the categories you need. Do not explain your choices.
Do not include any other text.`,
		menu.String(),
		roleplay,
	)

	// Use the tiny classifier model — fast and cheap, just needs to follow instructions
	config := ClassifierConfig()
	config.NumCtx = 4096

	response, err := Query(prompt, config)
	if err != nil {
		return nil, fmt.Errorf("batch pre-filter failed: %w", err)
	}

	// Parse the response into a set of selected batch names
	selected := make(map[string]bool)
	for _, line := range strings.Split(response, "\n") {
		name := strings.TrimSpace(line)
		name = strings.TrimPrefix(name, "- ")
		if name != "" {
			selected[name] = true
		}
	}

	// If the classifier returned nothing useful, fall back to all batches
	if len(selected) == 0 {
		return schemaBatches, nil
	}

	// Filter schemaBatches to only the selected ones, preserving order
	var filtered []SchemaBatch
	for _, batch := range schemaBatches {
		if selected[batch.Name] {
			filtered = append(filtered, batch)
		}
	}

	// Safety: if nothing matched (model hallucinated names), fall back to all
	if len(filtered) == 0 {
		return schemaBatches, nil
	}

	return filtered, nil
}
