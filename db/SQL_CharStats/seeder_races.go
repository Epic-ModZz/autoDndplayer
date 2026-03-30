package db

import (
	"database/sql"
	"encoding/json"
	"log"
	"strings"
)

type raceCard struct {
	Title    string   `json:"title"`
	Contents []string `json:"contents"`
	Tags     []string `json:"tags"`
}

// SeedRaces inserts all races and subraces from races.json.
// Version rule: PHB'24 preferred over PHB'14 for the same title.
// Titles containing ";" are subrace entries: "Elf; High Elf Lineage" →
// parent "Elf", subrace "High Elf Lineage".
func SeedRaces() {
	data, err := readDataFile("races.json")
	if err != nil {
		log.Printf("SeedRaces: read file: %v", err)
		return
	}
	var cards []raceCard
	if err := json.Unmarshal(data, &cards); err != nil {
		log.Printf("SeedRaces: parse failed: %v", err)
		return
	}

	// Build set of titles that have a PHB'24 version.
	phb24 := make(map[string]bool)
	for _, c := range cards {
		if hasTag(c.Tags, "PHB'24") {
			phb24[c.Title] = true
		}
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Printf("SeedRaces: begin tx: %v", err)
		return
	}
	defer tx.Rollback()

	raceIDs := make(map[string]int64) // race name → db id
	raceCount, subraceCount := 0, 0

	// Pass 1: base races (no semicolon in title).
	for _, card := range cards {
		if hasTag(card.Tags, "PHB'14") && phb24[card.Title] {
			continue
		}
		if strings.Contains(card.Title, ";") {
			continue
		}

		props := parseContentsProps(card.Contents)
		source := getSource(card.Tags)

		var traits []string
		for _, line := range card.Contents {
			parts := splitContent(line)
			if len(parts) >= 2 && parts[0] == "description" {
				traits = append(traits, cleanText(parts[1]))
			}
		}

		res, err := tx.Exec(
			`INSERT OR IGNORE INTO races
			 (name, size, speed, ability_score_increases, languages, traits_summary, source)
			 VALUES (?,?,?,?,?,?,?)`,
			card.Title,
			props["Size"],
			props["Speed"],
			props["Ability Scores"],
			props["Languages"],
			strings.Join(traits, ", "),
			source,
		)
		if err != nil {
			log.Printf("SeedRaces: insert %q: %v", card.Title, err)
			continue
		}
		id, _ := res.LastInsertId()
		if id > 0 {
			raceIDs[card.Title] = id
			raceCount++
			insertRaceFeatures(tx, id, card.Contents, false)
		}
	}

	// Pass 2: subraces (semicolon in title).
	for _, card := range cards {
		if hasTag(card.Tags, "PHB'14") && phb24[card.Title] {
			continue
		}
		if !strings.Contains(card.Title, ";") {
			continue
		}

		halves := strings.SplitN(card.Title, ";", 2)
		parentName := strings.TrimSpace(halves[0])
		subName := strings.TrimSpace(halves[1])
		source := getSource(card.Tags)

		parentID := raceIDs[parentName]
		if parentID == 0 {
			// Parent was already in DB from a prior run.
			DB.QueryRow(`SELECT id FROM races WHERE name = ?`, parentName).Scan(&parentID)
		}
		if parentID == 0 {
			continue
		}

		var desc string
		for _, line := range card.Contents {
			parts := splitContent(line)
			if parts[0] == "text" && len(parts) >= 2 {
				desc = cleanText(parts[len(parts)-1])
				break
			}
		}

		res, err := tx.Exec(
			`INSERT OR IGNORE INTO subraces (race_id, name, description, source) VALUES (?,?,?,?)`,
			parentID, subName, desc, source,
		)
		if err != nil {
			log.Printf("SeedRaces: insert subrace %q: %v", subName, err)
			continue
		}
		subraceID, _ := res.LastInsertId()
		if subraceID > 0 {
			subraceCount++
			insertRaceFeatures(tx, subraceID, card.Contents, true)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("SeedRaces: commit: %v", err)
		return
	}
	log.Printf("SeedRaces: %d races, %d subraces", raceCount, subraceCount)
}

func insertRaceFeatures(tx *sql.Tx, parentID int64, contents []string, isSubrace bool) {
	table := "race_features"
	col := "race_id"
	if isSubrace {
		table = "subrace_features"
		col = "subrace_id"
	}
	for _, line := range contents {
		parts := splitContent(line)
		if len(parts) == 3 && parts[0] == "description" {
			name := cleanText(parts[1])
			desc := cleanText(parts[2])
			if name == "" || desc == "" {
				continue
			}
			tx.Exec(
				`INSERT OR IGNORE INTO `+table+` (`+col+`, name, description, level) VALUES (?,?,?,1)`,
				parentID, name, desc,
			)
		}
	}
}
