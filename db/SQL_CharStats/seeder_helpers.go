package db

import (
	"log"
	"os"
	"regexp"
	"strings"
)

// dataDir is the path to the RawData/Data directory, relative to the working
// directory the binary is run from (the project root).
const dataDir = "db/SQL_CharStats/tableStructs/RawData/Data"

// readDataFile reads a JSON seed file from the data directory.
func readDataFile(name string) ([]byte, error) {
	return os.ReadFile(dataDir + "/" + name)
}

// reToolsTag strips 5etools template tags like {@damage 2d8} → "2d8"
var reToolsTag = regexp.MustCompile(`\{@\w+\s*([^|}{}]*)[^}]*\}`)

// reHTML strips HTML tags
var reHTML = regexp.MustCompile(`<[^>]+>`)

// cleanText strips 5etools markup and HTML from a string.
func cleanText(s string) string {
	s = reToolsTag.ReplaceAllStringFunc(s, func(m string) string {
		sub := reToolsTag.FindStringSubmatch(m)
		if len(sub) > 1 {
			return strings.TrimSpace(sub[1])
		}
		return ""
	})
	s = reHTML.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// hasTag returns true if tag is in the tags slice.
func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// getSource returns the first source-like tag, skipping generic type tags.
func getSource(tags []string) string {
	skip := map[string]bool{
		"race": true, "feat": true, "boon": true, "spell": true,
		"creature": true, "background": true, "optional feature": true,
	}
	for _, t := range tags {
		if !skip[t] {
			return t
		}
	}
	return "Unknown"
}

// splitContent splits a pipe-delimited content line into up to 3 parts.
// "property | Armor class | 16" → ["property", "Armor class", "16"]
func splitContent(line string) []string {
	parts := strings.SplitN(line, " | ", 3)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// parseContentsProps extracts a map of all "property" lines from a contents array.
func parseContentsProps(contents []string) map[string]string {
	m := make(map[string]string)
	for _, line := range contents {
		parts := splitContent(line)
		if len(parts) == 3 && parts[0] == "property" {
			m[parts[1]] = cleanText(parts[2])
		}
	}
	return m
}

// flattenEntries recursively flattens a 5etools entries []interface{} to plain text.
func flattenEntries(entries []interface{}) string {
	var parts []string
	for _, e := range entries {
		switch v := e.(type) {
		case string:
			if s := cleanText(v); s != "" {
				parts = append(parts, s)
			}
		case map[string]interface{}:
			name, _ := v["name"].(string)
			for _, key := range []string{"entries", "items"} {
				if sub, ok := v[key].([]interface{}); ok {
					if s := flattenEntries(sub); s != "" {
						if name != "" {
							parts = append(parts, name+": "+s)
						} else {
							parts = append(parts, s)
						}
					}
					break
				}
			}
		}
	}
	return strings.Join(parts, " ")
}

// truncate shortens s to at most n bytes, breaking on a space where possible.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if idx := strings.LastIndex(s[:n], " "); idx > n/2 {
		return s[:idx]
	}
	return s[:n]
}

// SeedReferenceData seeds all immutable reference tables.
// Checks if races is already populated and skips entirely if so —
// safe to call on every startup.
func SeedReferenceData() {
	var count int
	if err := DB.QueryRow("SELECT COUNT(*) FROM races").Scan(&count); err != nil || count > 0 {
		if count > 0 {
			log.Println("Reference data already seeded, skipping")
		}
		return
	}

	log.Println("Seeding reference data...")
	SeedRaces()
	SeedClasses()
	SeedFeats()
	SeedBackgrounds()
	SeedSpells()
	SeedMonsters()
	log.Println("Reference data seeding complete")
}
