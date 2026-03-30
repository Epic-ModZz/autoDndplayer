package db

import (
	"database/sql"
	"encoding/json"
	"log"
	"strconv"
	"strings"
)

// SeedSpells inserts all spells from spells.json.
// No version filtering needed — the file is already curated to 2024 content.
func SeedSpells() {
	data, err := readDataFile("spells.json")
	if err != nil {
		log.Printf("SeedSpells: read file: %v", err)
		return
	}
	var cards []raceCard
	if err := json.Unmarshal(data, &cards); err != nil {
		log.Printf("SeedSpells: parse failed: %v", err)
		return
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Printf("SeedSpells: begin tx: %v", err)
		return
	}
	defer tx.Rollback()

	count := 0

	for _, card := range cards {
		source := getSource(card.Tags)
		level, school := parseSpellSubtitle(card.Contents)
		props := parseContentsProps(card.Contents)

		castingTime := props["Casting Time"]
		spellRange  := props["Range"]
		duration    := props["Duration"]
		components  := props["Components"]

		concentration := 0
		if strings.Contains(strings.ToLower(duration), "concentration") {
			concentration = 1
		}
		ritual := 0
		if strings.Contains(strings.ToLower(castingTime), "ritual") {
			ritual = 1
		}

		var descLines, higherLines []string
		inHigher := false
		for _, line := range card.Contents {
			parts := splitContent(line)
			if len(parts) < 2 {
				continue
			}
			switch parts[0] {
			case "section":
				if len(parts) >= 2 && strings.Contains(strings.ToLower(parts[1]), "higher") {
					inHigher = true
				}
			case "text":
				text := cleanText(parts[len(parts)-1])
				if text == "" {
					continue
				}
				if inHigher {
					higherLines = append(higherLines, text)
				} else {
					descLines = append(descLines, text)
				}
			}
		}
		desc := strings.Join(descLines, " ")
		higherLevels := strings.Join(higherLines, " ")
		if len(desc) > 2000 {
			desc = desc[:2000]
		}

		classNames := extractClassTags(card.Tags)

		res, err := tx.Exec(
			`INSERT OR IGNORE INTO spells
			 (name, level, school, casting_time, range, duration, concentration,
			  ritual, description, higher_levels, classes, source)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			card.Title, level, school, castingTime, spellRange, duration,
			concentration, ritual, desc, higherLevels, classNames, source,
		)
		if err != nil {
			log.Printf("SeedSpells: insert %q: %v", card.Title, err)
			continue
		}
		id, _ := res.LastInsertId()
		if id > 0 {
			count++
			insertSpellComponents(tx, id, components)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("SeedSpells: commit: %v", err)
		return
	}
	log.Printf("SeedSpells: %d spells", count)
}

// parseSpellSubtitle extracts level and school from a subtitle line.
// "Level 1 Abjuration" → 1, "Abjuration"
// "Cantrip Evocation"  → 0, "Evocation"
func parseSpellSubtitle(contents []string) (level int, school string) {
	for _, line := range contents {
		parts := splitContent(line)
		if len(parts) >= 2 && parts[0] == "subtitle" {
			sub := parts[1]
			if strings.HasPrefix(sub, "Cantrip") {
				school = strings.TrimSpace(strings.TrimPrefix(sub, "Cantrip"))
				return 0, school
			}
			if strings.HasPrefix(sub, "Level ") {
				rest := strings.TrimPrefix(sub, "Level ")
				fields := strings.SplitN(rest, " ", 2)
				level, _ = strconv.Atoi(fields[0])
				if len(fields) == 2 {
					school = fields[1]
				}
				return
			}
		}
	}
	return
}

// insertSpellComponents inserts V/S/M components for a spell.
func insertSpellComponents(tx *sql.Tx, spellID int64, components string) {
	if components == "" {
		return
	}
	for _, comp := range strings.Split(components, ",") {
		comp = strings.TrimSpace(comp)
		if comp == "" {
			continue
		}
		compType := string([]rune(comp)[0]) // first character: V, S, or M
		compDesc := ""
		if compType == "M" {
			if start := strings.Index(comp, "("); start != -1 {
				if end := strings.LastIndex(comp, ")"); end > start {
					compDesc = comp[start+1 : end]
				}
			}
		}
		tx.Exec(
			`INSERT OR IGNORE INTO spell_components (spell_id, type, description) VALUES (?,?,?)`,
			spellID, compType, compDesc,
		)
	}
}

var knownClassTags = map[string]bool{
	"Wizard": true, "Sorcerer": true, "Cleric": true, "Druid": true,
	"Bard": true, "Warlock": true, "Paladin": true, "Ranger": true,
	"Fighter": true, "Rogue": true, "Monk": true, "Artificer": true,
}

// extractClassTags filters tags to deduplicated class name tags.
func extractClassTags(tags []string) string {
	seen := make(map[string]bool)
	var classes []string
	for _, t := range tags {
		if knownClassTags[t] && !seen[t] {
			classes = append(classes, t)
			seen[t] = true
		}
	}
	return strings.Join(classes, ", ")
}
