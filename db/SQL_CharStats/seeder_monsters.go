package db

import (
	"database/sql"
	"encoding/json"
	"log"
	"regexp"
	"strconv"
	"strings"
)

var (
	reCR = regexp.MustCompile(`^([\d/]+)`)
	reXP = regexp.MustCompile(`XP ([\d,]+)`)
	reHP = regexp.MustCompile(`^(\d+)`)
)

// SeedMonsters inserts all monsters from monsters.json.
// Version rule: MM'25 preferred over MM'14 for the same title.
func SeedMonsters() {
	data, err := readDataFile("monsters.json")
	if err != nil {
		log.Printf("SeedMonsters: read file: %v", err)
		return
	}
	var cards []raceCard
	if err := json.Unmarshal(data, &cards); err != nil {
		log.Printf("SeedMonsters: parse failed: %v", err)
		return
	}

	mm25 := make(map[string]bool)
	for _, c := range cards {
		if hasTag(c.Tags, "MM'25") {
			mm25[c.Title] = true
		}
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Printf("SeedMonsters: begin tx: %v", err)
		return
	}
	defer tx.Rollback()

	count := 0

	for _, card := range cards {
		if hasTag(card.Tags, "MM'14") && mm25[card.Title] {
			continue
		}

		source := getSource(card.Tags)
		size, monsterType, alignment := parseSubtitle(card.Contents)
		props := parseContentsProps(card.Contents)

		ac := parseFirstInt(props["Armor class"])
		hp := parseFirstInt(props["Hit points"])
		cr := parseCR(props["Challenge"])
		xp := parseXP(props["Challenge"])
		speed := props["Speed"]

		str, dex, con, intel, wis, cha := parseStats(card.Contents)

		res, err := tx.Exec(
			`INSERT OR IGNORE INTO monsters
			 (name, size, type, alignment, ac, hp, speed, str, dex, con, int, wis, cha, cr, xp, source)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			card.Title, size, monsterType, alignment,
			ac, hp, speed, str, dex, con, intel, wis, cha,
			cr, xp, source,
		)
		if err != nil {
			log.Printf("SeedMonsters: insert %q: %v", card.Title, err)
			continue
		}
		id, _ := res.LastInsertId()
		if id == 0 {
			continue
		}
		count++
		insertMonsterContent(tx, id, card.Contents)
	}

	if err := tx.Commit(); err != nil {
		log.Printf("SeedMonsters: commit: %v", err)
		return
	}
	log.Printf("SeedMonsters: %d monsters", count)
}

// insertMonsterContent parses the contents array and inserts traits, actions,
// and legendary actions using the provided transaction.
func insertMonsterContent(tx *sql.Tx, monsterID int64, contents []string) {
	section := "trait"

	for _, line := range contents {
		parts := splitContent(line)
		if len(parts) < 2 {
			continue
		}

		switch parts[0] {
		case "section":
			s := strings.ToLower(cleanText(parts[len(parts)-1]))
			switch {
			case strings.Contains(s, "legendary"):
				section = "legendary"
			case strings.Contains(s, "action"), strings.Contains(s, "reaction"), strings.Contains(s, "bonus"):
				section = "action"
			default:
				section = "trait"
			}

		case "description":
			if len(parts) < 3 {
				continue
			}
			name := cleanText(parts[1])
			desc := truncate(cleanText(parts[2]), 1500)
			if name == "" {
				continue
			}

			switch section {
			case "action":
				actionType := "action"
				if strings.Contains(strings.ToLower(name), "reaction") {
					actionType = "reaction"
				} else if strings.Contains(strings.ToLower(line), "bonus") {
					actionType = "bonus_action"
				}
				tx.Exec(
					`INSERT OR IGNORE INTO monster_actions
					 (monster_id, name, action_type, attack_bonus, hit_dice, damage_type, description)
					 VALUES (?,?,?,0,'','',?)`,
					monsterID, name, actionType, desc,
				)
			case "legendary":
				tx.Exec(
					`INSERT OR IGNORE INTO monster_actions
					 (monster_id, name, action_type, attack_bonus, hit_dice, damage_type, description)
					 VALUES (?,?,'legendary',0,'','',?)`,
					monsterID, name, desc,
				)
			default:
				tx.Exec(
					`INSERT OR IGNORE INTO monster_traits (monster_id, name, description) VALUES (?,?,?)`,
					monsterID, name, desc,
				)
			}
		}
	}
}

// parseSubtitle extracts size, type, and alignment from the subtitle line.
// "Medium Elemental, Neutral" → "Medium", "Elemental", "Neutral"
func parseSubtitle(contents []string) (size, monsterType, alignment string) {
	for _, line := range contents {
		parts := splitContent(line)
		if len(parts) >= 2 && parts[0] == "subtitle" {
			text := cleanText(parts[len(parts)-1])
			halves := strings.SplitN(text, ",", 2)
			if len(halves) == 2 {
				alignment = strings.TrimSpace(halves[1])
			}
			words := strings.SplitN(strings.TrimSpace(halves[0]), " ", 2)
			if len(words) == 2 {
				size = words[0]
				monsterType = words[1]
			} else {
				monsterType = strings.TrimSpace(halves[0])
			}
			return
		}
	}
	return
}

// parseStats finds the dndstats line and returns the six ability scores.
// "dndstats | 10 | 16 | 12 | 13 | 17 | 12"
func parseStats(contents []string) (str, dex, con, intel, wis, cha int) {
	for _, line := range contents {
		raw := strings.Split(line, " | ")
		if len(raw) >= 7 && strings.TrimSpace(raw[0]) == "dndstats" {
			str, _ = strconv.Atoi(strings.TrimSpace(raw[1]))
			dex, _ = strconv.Atoi(strings.TrimSpace(raw[2]))
			con, _ = strconv.Atoi(strings.TrimSpace(raw[3]))
			intel, _ = strconv.Atoi(strings.TrimSpace(raw[4]))
			wis, _ = strconv.Atoi(strings.TrimSpace(raw[5]))
			cha, _ = strconv.Atoi(strings.TrimSpace(raw[6]))
			return
		}
	}
	return
}

// parseCR extracts the CR fraction/number from a Challenge property value.
// "4 (XP 1,100; PB +2)" → "4"
func parseCR(s string) string {
	if m := reCR.FindString(s); m != "" {
		return m
	}
	return "0"
}

// parseXP extracts the XP integer from a Challenge property value.
// "4 (XP 1,100; PB +2)" → 1100
func parseXP(s string) int {
	m := reXP.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	return n
}

// parseFirstInt extracts the leading integer from a property value string.
// "66 (12d8 + 12)" → 66,  "16 (natural armor)" → 16
func parseFirstInt(s string) int {
	if m := reHP.FindString(strings.TrimSpace(s)); m != "" {
		n, _ := strconv.Atoi(m)
		return n
	}
	return 0
}
