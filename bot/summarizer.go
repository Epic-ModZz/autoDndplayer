package bot

import (
	"fmt"
	"strings"
)

const (
	chunkSize     = 3000
	summaryBudget = 4000
)

// KnowledgeSummary partitions what the summarizer found into two pools.
// IC is what the character learned through in-fiction experience and can
// act on freely. OOC is what the player knows through out-of-character
// channels (OOC chat, DMs) — the character cannot reference it directly
// but it can inform strategy and awareness.
type KnowledgeSummary struct {
	IC  string // character knows this — safe to use in IC responses
	OOC string // player knows this, character does not — inform strategy only
}

// SummarizeDBResults takes raw query results, chunks them, runs each chunk
// through a small LLM, and returns a KnowledgeSummary partitioned by how
// each piece of knowledge was obtained.
func SummarizeDBResults(formattedResults string, job *MessageJob) (*KnowledgeSummary, error) {
	var context strings.Builder
	for i := len(job.Messages) - 1; i >= 0; i-- {
		m := job.Messages[i]
		context.WriteString(fmt.Sprintf("%s: %s\n", m.Author.Username, m.Content))
	}
	scene := fmt.Sprintf("%s\nLatest message from %s: \"%s\"",
		context.String(),
		job.Message.Author.Username,
		job.Message.Content,
	)

	chunks := chunkString(formattedResults, chunkSize)

	var icSummaries []string
	var oocSummaries []string
	config := SummarizerConfig()

	for i, chunk := range chunks {
		prompt := fmt.Sprintf(`You are helping a D&D bot distinguish between what its CHARACTER knows
and what the PLAYER (you, the system) knows from out-of-character sources.

Below is a chunk of raw database results (%d of %d) and the current scene.

## Knowledge Source Rules
- knowledge_source = 'ic' or discovered_ic = 1  →  the CHARACTER learned this in-fiction
- knowledge_source = 'ooc' or 'dm'  →  the PLAYER knows this but the CHARACTER does not
- Rows with no knowledge_source column are assumed IC

## Database Results
%s

## Current Scene
%s

## Your Task
Extract information relevant to the scene and sort it into two sections.
Use short bullet points only.
If a section has nothing relevant, write "nothing relevant" for that section.

## CHARACTER KNOWS (IC)
(things the character experienced in-fiction — safe to use in roleplay)

## PLAYER KNOWS ONLY (OOC)
(things learned via OOC channels or DMs — inform strategy but never reference directly in character)`,
			i+1, len(chunks), chunk, scene)

		response, err := Query(prompt, config)
		if err != nil {
			return nil, fmt.Errorf("summarizer failed on chunk %d: %w", i+1, err)
		}

		ic, ooc := splitKnowledgeSections(response)
		if ic != "" {
			icSummaries = append(icSummaries, ic)
		}
		if ooc != "" {
			oocSummaries = append(oocSummaries, ooc)
		}
	}

	result := &KnowledgeSummary{
		IC:  strings.Join(icSummaries, "\n\n"),
		OOC: strings.Join(oocSummaries, "\n\n"),
	}

	// Compress each section independently if over budget.
	if len(result.IC) > summaryBudget {
		compressed, err := compressSummary(result.IC, scene, config)
		if err != nil {
			return nil, err
		}
		result.IC = compressed
	}
	if len(result.OOC) > summaryBudget {
		compressed, err := compressSummary(result.OOC, scene, config)
		if err != nil {
			return nil, err
		}
		result.OOC = compressed
	}

	return result, nil
}

// splitKnowledgeSections parses the LLM's two-section output into IC and OOC strings.
// It looks for the section headers and splits on them.
func splitKnowledgeSections(response string) (ic, ooc string) {
	icMarker := "## CHARACTER KNOWS (IC)"
	oocMarker := "## PLAYER KNOWS ONLY (OOC)"

	icIdx := strings.Index(response, icMarker)
	oocIdx := strings.Index(response, oocMarker)

	if icIdx == -1 && oocIdx == -1 {
		// LLM didn't follow the format — treat the whole thing as IC
		// since that's the safer assumption.
		trimmed := strings.TrimSpace(response)
		if trimmed != "" && !strings.EqualFold(trimmed, "nothing relevant") {
			return trimmed, ""
		}
		return "", ""
	}

	if icIdx != -1 {
		start := icIdx + len(icMarker)
		end := len(response)
		if oocIdx != -1 && oocIdx > icIdx {
			end = oocIdx
		}
		ic = strings.TrimSpace(response[start:end])
		if strings.EqualFold(ic, "nothing relevant") {
			ic = ""
		}
	}

	if oocIdx != -1 {
		start := oocIdx + len(oocMarker)
		ooc = strings.TrimSpace(response[start:])
		if strings.EqualFold(ooc, "nothing relevant") {
			ooc = ""
		}
	}

	return ic, ooc
}

// compressSummary does a final single LLM pass to compress an oversized summary.
func compressSummary(summary string, scene string, config QueryConfig) (string, error) {
	prompt := fmt.Sprintf(`You are helping a D&D bot prepare a response.
The following summary is too long. Compress it, keeping only the most critical details for the scene.
Use short bullet points only.

## Summary To Compress
%s

## Current Scene
%s

## Compressed Summary`,
		summary, scene)

	response, err := Query(prompt, config)
	if err != nil {
		return "", fmt.Errorf("summary compression failed: %w", err)
	}
	return strings.TrimSpace(response), nil
}

// chunkString splits a string into chunks of at most size characters,
// preferring to split on newlines.
func chunkString(s string, size int) []string {
	var chunks []string
	lines := strings.Split(s, "\n")
	var current strings.Builder

	for _, line := range lines {
		if current.Len()+len(line)+1 > size && current.Len() > 0 {
			chunks = append(chunks, current.String())
			current.Reset()
		}
		current.WriteString(line)
		current.WriteString("\n")
	}

	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}

	return chunks
}
