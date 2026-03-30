package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

type classJSON struct {
	Class           []classEntry           `json:"class"`
	Subclass        []subclassEntry        `json:"subclass"`
	ClassFeature    []classFeatureEntry    `json:"classFeature"`
	SubclassFeature []subclassFeatureEntry `json:"subclassFeature"`
}

type classEntry struct {
	Name        string   `json:"name"`
	Source      string   `json:"source"`
	Edition     string   `json:"edition"`
	ReprintedAs []string `json:"reprintedAs"`
	HD          struct {
		Faces int `json:"faces"`
	} `json:"hd"`
	Proficiency           []string `json:"proficiency"`
	SpellcastingAbility   string   `json:"spellcastingAbility"`
	StartingProficiencies struct {
		Armor   []interface{} `json:"armor"`
		Weapons []interface{} `json:"weapons"`
	} `json:"startingProficiencies"`
	ClassTableGroups []struct {
		Title                string  `json:"title"`
		RowsSpellProgression [][]int `json:"rowsSpellProgression"`
	} `json:"classTableGroups"`
}

type subclassEntry struct {
	Name      string `json:"name"`
	ShortName string `json:"shortName"`
	ClassName string `json:"className"`
	Source    string `json:"source"`
	Edition   string `json:"edition"`
}

type classFeatureEntry struct {
	Name        string        `json:"name"`
	ClassName   string        `json:"className"`
	ClassSource string        `json:"classSource"`
	Level       int           `json:"level"`
	Source      string        `json:"source"`
	Entries     []interface{} `json:"entries"`
}

type subclassFeatureEntry struct {
	Name              string        `json:"name"`
	SubclassShortName string        `json:"subclassShortName"`
	ClassName         string        `json:"className"`
	ClassSource       string        `json:"classSource"`
	Level             int           `json:"level"`
	Source            string        `json:"source"`
	Entries           []interface{} `json:"entries"`
}

var abilityAbbrevToFull = map[string]string{
	"str": "Strength", "dex": "Dexterity", "con": "Constitution",
	"int": "Intelligence", "wis": "Wisdom", "cha": "Charisma",
}

// planeshiftSources are third-party Plane Shift supplements excluded from seeding.
var planeshiftSources = map[string]bool{
	"PSA": true, "PSK": true, "PSZ": true,
	"PSX": true, "PSI": true, "PSG": true,
}

// canonicalShortName normalises shortName values so that 2024 XPHB reprints
// that received a new shortName still resolve to the same dedup key as their
// 2014 classic counterpart. Map is: XPHB shortName -> canonical (classic) shortName.
var canonicalShortName = map[string]string{
	// Fighter
	"Banneret": "Purple Dragon Knight (Banneret)",
	// Sorcerer
	"Wild Magic": "Wild",
	"Aberrant":   "Aberrant Mind",
	"Clockwork":  "Clockwork Soul",
	// Wizard
	"Abjurer":     "Abjuration",
	"Diviner":     "Divination",
	"Evoker":      "Evocation",
	"Illusionist": "Illusion",
	"Bladesinger": "Bladesinging",
}

// normaliseShortName returns the canonical shortName for dedup purposes.
func normaliseShortName(shortName string) string {
	if canonical, ok := canonicalShortName[shortName]; ok {
		return canonical
	}
	return shortName
}

func SeedClasses() {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		log.Printf("SeedClasses: readdir: %v", err)
		return
	}

	var (
		allClasses       []classEntry
		allSubclasses    []subclassEntry
		allClassFeats    []classFeatureEntry
		allSubclassFeats []subclassFeatureEntry
	)

	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "class-") {
			continue
		}
		data, err := os.ReadFile(dataDir + "/" + e.Name())
		if err != nil {
			log.Printf("SeedClasses: read %s: %v", e.Name(), err)
			continue
		}
		var cj classJSON
		if err := json.Unmarshal(data, &cj); err != nil {
			log.Printf("SeedClasses: parse %s: %v", e.Name(), err)
			continue
		}
		allClasses = append(allClasses, cj.Class...)
		allSubclasses = append(allSubclasses, cj.Subclass...)
		allClassFeats = append(allClassFeats, cj.ClassFeature...)
		allSubclassFeats = append(allSubclassFeats, cj.SubclassFeature...)
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Printf("SeedClasses: begin tx: %v", err)
		return
	}
	defer tx.Rollback()

	classIDs := make(map[string]int64)
	classCount := 0
	preferredClassSrc := bestClassSource(allClasses)

	for _, c := range allClasses {
		// Edition is "classic" or "one" for class entries, never empty.
		// Skip whichever edition is not preferred (XPHB wins when available).
		if preferredClassSrc[c.Name] != c.Source {
			continue
		}
		res, err := tx.Exec(
			`INSERT OR IGNORE INTO classes
			 (name, hit_die, primary_ability, saving_throws,
			  spellcasting_ability, armor_proficiencies, weapon_proficiencies)
			 VALUES (?,?,?,?,?,?,?)`,
			c.Name, c.HD.Faces, c.SpellcastingAbility,
			joinAbilities(c.Proficiency), c.SpellcastingAbility,
			joinInterfaceStrings(c.StartingProficiencies.Armor),
			joinInterfaceStrings(c.StartingProficiencies.Weapons),
		)
		if err != nil {
			log.Printf("SeedClasses: insert class %q: %v", c.Name, err)
			continue
		}
		id, _ := res.LastInsertId()
		if id == 0 {
			continue
		}
		classIDs[c.Name+"|"+c.Source] = id
		classCount++
		if err := insertClassSpellSlots(tx, id, c); err != nil {
			log.Printf("SeedClasses: spell slots for %q: %v", c.Name, err)
		}
	}

	subclassIDs := make(map[string]int64)
	subclassCount := 0
	prefSubSrc := bestSubclassSource(allSubclasses)

	for _, sc := range allSubclasses {
		if planeshiftSources[sc.Source] {
			continue
		}
		if sc.Edition == "" || prefSubSrc[sc.ClassName+"|"+normaliseShortName(sc.ShortName)] != sc.Source {
			continue
		}
		classID := resolveClassID(classIDs, sc.ClassName)
		if classID == 0 {
			continue
		}
		res, err := tx.Exec(
			`INSERT OR IGNORE INTO subclasses (class_id, name, description, source) VALUES (?,?,?,?)`,
			classID, sc.Name, "", sc.Source,
		)
		if err != nil {
			log.Printf("SeedClasses: insert subclass %q: %v", sc.Name, err)
			continue
		}
		id, _ := res.LastInsertId()
		if id == 0 {
			continue
		}
		subclassIDs[sc.ClassName+"|"+normaliseShortName(sc.ShortName)] = id
		subclassCount++
	}

	featCount := 0
	prefFeatSrc := bestFeatureSource(allClassFeats)

	for _, f := range allClassFeats {
		if prefFeatSrc[f.ClassName] != f.ClassSource {
			continue
		}
		classID := resolveClassID(classIDs, f.ClassName)
		if classID == 0 {
			continue
		}
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO class_features (class_id, name, level, description) VALUES (?,?,?,?)`,
			classID, f.Name, f.Level, truncate(flattenEntries(f.Entries), 2000),
		); err == nil {
			featCount++
		}
	}

	subFeatCount := 0
	prefSubFeatSrc := bestSubFeatureSource(allSubclassFeats)

	for _, f := range allSubclassFeats {
		if planeshiftSources[f.Source] {
			continue
		}
		key := f.ClassName + "|" + f.SubclassShortName
		if prefSubFeatSrc[key] != f.ClassSource {
			continue
		}
		scID := subclassIDs[key]
		if scID == 0 {
			continue
		}
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO subclass_features (subclass_id, name, level, description) VALUES (?,?,?,?)`,
			scID, f.Name, f.Level, truncate(flattenEntries(f.Entries), 2000),
		); err == nil {
			subFeatCount++
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("SeedClasses: commit: %v", err)
		return
	}
	log.Printf("SeedClasses: %d classes, %d subclasses, %d class features, %d subclass features",
		classCount, subclassCount, featCount, subFeatCount)
}

func insertClassSpellSlots(tx *sql.Tx, classID int64, c classEntry) error {
	for _, tg := range c.ClassTableGroups {
		if !strings.Contains(tg.Title, "Slot") || tg.RowsSpellProgression == nil {
			continue
		}
		for level, row := range tg.RowsSpellProgression {
			slots := padSlots(row, 9)
			if _, err := tx.Exec(
				`INSERT OR IGNORE INTO single_class_spell_slots
				 (class_id, level, slot_level_1, slot_level_2, slot_level_3,
				  slot_level_4, slot_level_5, slot_level_6, slot_level_7,
				  slot_level_8, slot_level_9)
				 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
				classID, level+1,
				slots[0], slots[1], slots[2], slots[3], slots[4],
				slots[5], slots[6], slots[7], slots[8],
			); err != nil {
				return fmt.Errorf("level %d: %w", level+1, err)
			}
		}
		break
	}
	return nil
}

func bestClassSource(classes []classEntry) map[string]string {
	m := make(map[string]string)
	for _, c := range classes {
		if c.Edition == "" {
			continue
		}
		if m[c.Name] == "" || c.Source == "XPHB" {
			m[c.Name] = c.Source
		}
	}
	return m
}

// bestSubclassSource returns the preferred source per subclass, keyed by
// className+"|"+shortName so renamed 2024 reprints (e.g. "Fiend Patron"
// replacing "The Fiend") share a key and XPHB wins over classic.
func bestSubclassSource(scs []subclassEntry) map[string]string {
	m := make(map[string]string)
	// Pass 1: edition-tagged entries — XPHB beats classic.
	for _, sc := range scs {
		if sc.Edition == "" || planeshiftSources[sc.Source] {
			continue
		}
		key := sc.ClassName + "|" + normaliseShortName(sc.ShortName)
		if m[key] == "" || sc.Source == "XPHB" {
			m[key] = sc.Source
		}
	}
	// Pass 2: edition="" fallbacks for subclasses never reprinted in XPHB.
	for _, sc := range scs {
		if sc.Edition != "" || planeshiftSources[sc.Source] {
			continue
		}
		key := sc.ClassName + "|" + normaliseShortName(sc.ShortName)
		if m[key] == "" {
			m[key] = sc.Source
		}
	}
	return m
}

func bestFeatureSource(feats []classFeatureEntry) map[string]string {
	m := make(map[string]string)
	for _, f := range feats {
		if m[f.ClassName] == "" || f.ClassSource == "XPHB" {
			m[f.ClassName] = f.ClassSource
		}
	}
	return m
}

func bestSubFeatureSource(feats []subclassFeatureEntry) map[string]string {
	m := make(map[string]string)
	for _, f := range feats {
		key := f.ClassName + "|" + f.SubclassShortName
		if m[key] == "" || f.ClassSource == "XPHB" {
			m[key] = f.ClassSource
		}
	}
	return m
}

func resolveClassID(classIDs map[string]int64, className string) int64 {
	for _, src := range []string{"XPHB", "PHB"} {
		if id := classIDs[className+"|"+src]; id != 0 {
			return id
		}
	}
	for k, id := range classIDs {
		if strings.HasPrefix(k, className+"|") {
			return id
		}
	}
	return 0
}

func joinAbilities(abbrevs []string) string {
	var out []string
	for _, a := range abbrevs {
		if full, ok := abilityAbbrevToFull[a]; ok {
			out = append(out, full)
		} else {
			out = append(out, a)
		}
	}
	return strings.Join(out, ", ")
}

func joinInterfaceStrings(items []interface{}) string {
	var parts []string
	for _, item := range items {
		switch v := item.(type) {
		case string:
			parts = append(parts, v)
		case map[string]interface{}:
			_ = v
			parts = append(parts, "choice")
		}
	}
	return strings.Join(parts, ", ")
}

func padSlots(row []int, n int) []int {
	out := make([]int, n)
	copy(out, row)
	return out
}
