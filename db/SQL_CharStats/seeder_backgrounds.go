package db

import (
	"encoding/json"
	"log"
	"regexp"
	"strings"
)

var reBoldLabel = regexp.MustCompile(`<b>([^<]+)[.:]</b>\s*`)

// SeedBackgrounds inserts all backgrounds from backgrounds.json.
// Version rule: PHB'24 preferred over older sources for the same title.
func SeedBackgrounds() {
	data, err := readDataFile("backgrounds.json")
	if err != nil {
		log.Printf("SeedBackgrounds: read file: %v", err)
		return
	}
	var cards []raceCard
	if err := json.Unmarshal(data, &cards); err != nil {
		log.Printf("SeedBackgrounds: parse failed: %v", err)
		return
	}

	phb24 := make(map[string]bool)
	for _, c := range cards {
		if hasTag(c.Tags, "PHB'24") {
			phb24[c.Title] = true
		}
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Printf("SeedBackgrounds: begin tx: %v", err)
		return
	}
	defer tx.Rollback()

	count := 0

	for _, card := range cards {
		if !hasTag(card.Tags, "PHB'24") && phb24[card.Title] {
			continue
		}

		source := getSource(card.Tags)
		skills, tools, langs, equipment := extractBackgroundFields(card.Contents)

		var descLines []string
		for _, line := range card.Contents {
			parts := splitContent(line)
			if len(parts) >= 2 && (parts[0] == "text" || parts[0] == "property") {
				if v := cleanText(parts[len(parts)-1]); v != "" {
					descLines = append(descLines, v)
				}
			}
		}
		desc := strings.Join(descLines, " ")
		if len(desc) > 500 {
			desc = desc[:500]
		}

		res, err := tx.Exec(
			`INSERT OR IGNORE INTO backgrounds
			 (name, description, skill_proficiencies, tool_proficiencies, languages, equipment, source)
			 VALUES (?,?,?,?,?,?,?)`,
			card.Title, desc, skills, tools, langs, equipment, source,
		)
		if err != nil {
			log.Printf("SeedBackgrounds: insert %q: %v", card.Title, err)
			continue
		}
		id, _ := res.LastInsertId()
		if id > 0 {
			count++
			for _, line := range card.Contents {
				parts := splitContent(line)
				if len(parts) == 3 && parts[0] == "description" {
					name := cleanText(parts[1])
					featDesc := cleanText(parts[2])
					if name != "" && featDesc != "" {
						tx.Exec(
							`INSERT OR IGNORE INTO background_features (background_id, name, description) VALUES (?,?,?)`,
							id, name, featDesc,
						)
					}
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("SeedBackgrounds: commit: %v", err)
		return
	}
	log.Printf("SeedBackgrounds: %d backgrounds", count)
}

// extractBackgroundFields parses bullet lines to extract the four standard fields.
// Handles both PHB'24 format ("<b>Skill Proficiencies:</b> ...") and older formats.
func extractBackgroundFields(contents []string) (skills, tools, langs, equipment string) {
	for _, line := range contents {
		parts := splitContent(line)
		if len(parts) < 2 || parts[0] != "bullet" {
			continue
		}
		raw := parts[len(parts)-1]
		m := reBoldLabel.FindStringSubmatchIndex(raw)
		if m == nil {
			continue
		}
		label := strings.ToLower(raw[m[2]:m[3]])
		value := cleanText(raw[m[1]:])

		switch {
		case strings.Contains(label, "skill"):
			skills = value
		case strings.Contains(label, "tool"):
			tools = value
		case strings.Contains(label, "language"):
			langs = value
		case strings.Contains(label, "equipment"):
			if len(value) > 300 {
				value = value[:300]
			}
			equipment = value
		}
	}
	return
}
