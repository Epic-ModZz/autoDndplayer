package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
)

type optionCard struct {
	Title    string   `json:"title"`
	Contents []string `json:"contents"`
	Tags     []string `json:"tags"`
}

// SeedFeats seeds feats, boons, fighting styles, and eldritch invocations.
// Version rule: PHB'24 preferred over PHB'14 for the same title.
//
// Source layout:
//
//	feats&boons.json  — regular feats, epic boon feats, and fighting style feats
//	fighting_styles_and_EldritchInvocations.json — eldritch invocations/maneuvers only
//
// Category detection uses the Type/Prerequisites property line, not tags:
//
//	"Fighting Style Feat (...)" → fighting_styles table
//	"Epic Boon Feat (...)"      → boons table
//	anything else               → feats table
func SeedFeats() {
	seedFeatsBoons()
	seedInvocations()
}

func seedFeatsBoons() {
	// Actual filename in the data directory is feats&boons.json
	data, err := readDataFile("feats&boons.json")
	if err != nil {
		log.Printf("SeedFeats: read feats&boons: %v", err)
		return
	}
	var cards []optionCard
	if err := json.Unmarshal(data, &cards); err != nil {
		log.Printf("SeedFeats: parse feats&boons: %v", err)
		return
	}

	// Build set of titles that have a PHB'24 version for dedup.
	phb24 := make(map[string]bool)
	for _, c := range cards {
		if hasTag(c.Tags, "PHB'24") {
			phb24[c.Title] = true
		}
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Printf("SeedFeats: begin tx: %v", err)
		return
	}
	defer tx.Rollback()

	featCount, boonCount, styleCount := 0, 0, 0

	for _, card := range cards {
		// Skip older-edition duplicates when a PHB'24 version exists.
		if hasTag(card.Tags, "PHB'14") && phb24[card.Title] {
			continue
		}

		source := getSource(card.Tags)
		prereq, desc := parseFeatContent(card.Contents)

		switch {
		case isFightingStyle(card.Contents):
			availableTo := fightingStyleAvailability(card.Title)
			if _, err := tx.Exec(
				`INSERT OR IGNORE INTO fighting_styles (name, description, available_to, source) VALUES (?,?,?,?)`,
				card.Title, truncate(desc, 1500), availableTo, source,
			); err == nil {
				styleCount++
			} else {
				log.Printf("SeedFeats: insert fighting style %q: %v", card.Title, err)
			}

		case isEpicBoon(card.Contents):
			res, err := tx.Exec(
				`INSERT OR IGNORE INTO boons (name, description, prerequisite, source) VALUES (?,?,?,?)`,
				card.Title, truncate(desc, 2000), prereq, source,
			)
			if err != nil {
				log.Printf("SeedFeats: insert boon %q: %v", card.Title, err)
				continue
			}
			id, _ := res.LastInsertId()
			if id > 0 {
				boonCount++
				insertFeatFeatures(tx, "boon_features", "boon_id", id, card.Contents)
			}

		default:
			res, err := tx.Exec(
				`INSERT OR IGNORE INTO feats (name, prerequisite, description, source) VALUES (?,?,?,?)`,
				card.Title, prereq, truncate(desc, 2000), source,
			)
			if err != nil {
				log.Printf("SeedFeats: insert feat %q: %v", card.Title, err)
				continue
			}
			id, _ := res.LastInsertId()
			if id > 0 {
				featCount++
				insertFeatFeatures(tx, "feat_features", "feat_id", id, card.Contents)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("SeedFeats: commit feats/boons/styles: %v", err)
		return
	}
	log.Printf("SeedFeats: %d feats, %d epic boons, %d fighting styles", featCount, boonCount, styleCount)
}

// seedInvocations seeds eldritch invocations and maneuvers from the dedicated
// invocations file. This file contains only invocations — fighting styles were
// moved to feats&boons.json in the 2024 rules.
func seedInvocations() {
	data, err := readDataFile("fighting_styles_and_EldritchInvocations.json")
	if err != nil {
		log.Printf("SeedFeats: read invocations: %v", err)
		return
	}
	var cards []optionCard
	if err := json.Unmarshal(data, &cards); err != nil {
		log.Printf("SeedFeats: parse invocations: %v", err)
		return
	}

	// Prefer PHB'24 when a duplicate title exists.
	phb24 := make(map[string]bool)
	for _, c := range cards {
		if hasTag(c.Tags, "PHB'24") {
			phb24[c.Title] = true
		}
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Printf("SeedFeats: begin invocations tx: %v", err)
		return
	}
	defer tx.Rollback()

	count := 0

	for _, card := range cards {
		if !hasTag(card.Tags, "PHB'24") && phb24[card.Title] {
			continue
		}

		_, desc := parseFeatContent(card.Contents)
		prereqStr := parseContentsProps(card.Contents)["Prerequisites"]
		if prereqStr == "—" {
			prereqStr = ""
		}

		prereqLevel := parsePrereqLevel(prereqStr)
		prereqSpell := ""
		if strings.Contains(prereqStr, "Pact of") {
			prereqSpell = prereqStr
		}

		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO eldritch_invocations
			 (name, prerequisite_level, prerequisite_spell, description) VALUES (?,?,?,?)`,
			card.Title, prereqLevel, prereqSpell, truncate(desc, 1500),
		); err == nil {
			count++
		} else {
			log.Printf("SeedFeats: insert invocation %q: %v", card.Title, err)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("SeedFeats: commit invocations: %v", err)
		return
	}
	log.Printf("SeedFeats: %d eldritch invocations/maneuvers", count)
}

// isFightingStyle returns true when the card's Type/Prerequisites line
// contains "Fighting Style Feat" — the canonical marker in the 2024 data.
func isFightingStyle(contents []string) bool {
	for _, line := range contents {
		if strings.HasPrefix(line, "property | Type/Prerequisites") {
			return strings.Contains(line, "Fighting Style Feat")
		}
	}
	return false
}

// isEpicBoon returns true when the card's Type/Prerequisites line
// contains "Epic Boon Feat" — the canonical marker in the 2024 data.
func isEpicBoon(contents []string) bool {
	for _, line := range contents {
		if strings.HasPrefix(line, "property | Type/Prerequisites") {
			return strings.Contains(line, "Epic Boon")
		}
	}
	return false
}

// parseFeatContent extracts the prerequisite string and full description text
// from a feat/boon/invocation card's contents array.
func parseFeatContent(contents []string) (prereq, desc string) {
	var descParts []string
	for _, line := range contents {
		parts := splitContent(line)
		if len(parts) == 0 {
			continue
		}
		switch parts[0] {
		case "property":
			if len(parts) == 3 && (strings.Contains(parts[1], "Prerequisites") || strings.Contains(parts[1], "Type")) {
				prereq = cleanText(parts[2])
				if prereq == "—" {
					prereq = ""
				}
			}
		case "description":
			if len(parts) == 3 {
				descParts = append(descParts, cleanText(parts[1])+": "+cleanText(parts[2]))
			}
		case "text", "bullet":
			val := cleanText(parts[len(parts)-1])
			if val != "" {
				descParts = append(descParts, val)
			}
		}
	}
	desc = strings.Join(descParts, " ")
	return
}

// insertFeatFeatures inserts description lines as feature rows.
func insertFeatFeatures(tx *sql.Tx, table, idCol string, parentID int64, contents []string) {
	for _, line := range contents {
		parts := splitContent(line)
		if len(parts) == 3 && parts[0] == "description" {
			name := cleanText(parts[1])
			desc := cleanText(parts[2])
			if name != "" && desc != "" {
				tx.Exec(
					fmt.Sprintf(`INSERT OR IGNORE INTO %s (%s, description) VALUES (?,?)`, table, idCol),
					parentID, desc,
				)
			}
		}
	}
}

// fightingStyleAvailability returns a comma-separated list of classes that
// have access to a given fighting style.
func fightingStyleAvailability(name string) string {
	switch name {
	case "Blessed Warrior":
		return "Paladin"
	case "Druidic Warrior":
		return "Ranger"
	case "Unarmed Fighting":
		return "Fighter, Monk"
	default:
		return "Fighter, Paladin, Ranger"
	}
}

// parsePrereqLevel extracts a minimum level from a prerequisite string.
// "Lvl 5, Pact of the Blade" → 5
func parsePrereqLevel(prereq string) int {
	lower := strings.ToLower(prereq)
	idx := strings.Index(lower, "lvl")
	if idx == -1 {
		return 0
	}
	rest := strings.TrimSpace(prereq[idx+3:])
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimRight(fields[0], ",;"))
	return n
}
