package bot

import (
	"fmt"
	"strings"
)

// buildKnowledgeSections constructs the data blocks injected into prompts.
// IC and OOC knowledge are presented as separate labeled sections so the
// LLM understands what the character can act on vs what it must keep internal.
//
// For OOC responses the distinction collapses — a player speaking OOC can
// draw on everything, so a single merged section is returned instead.
func buildKnowledgeSections(ctx ResponseContext) (icSection, oocSection string) {
	if ctx.RawFallback != "" && ctx.ICKnowledge == "" && ctx.OOCKnowledge == "" {
		// Summarizer fell back to raw — treat it all as IC, safest assumption.
		icSection = fmt.Sprintf("## What the Character Knows\n%s\n*(unsummarized — use judgement)*", ctx.RawFallback)
		return icSection, ""
	}

	if ctx.ICKnowledge == "" && ctx.OOCKnowledge == "" {
		icSection = "## What the Character Knows\nNothing retrieved. Respond from the scene context alone. Do not invent specific facts."
		return icSection, ""
	}

	if ctx.ICKnowledge != "" {
		icSection = fmt.Sprintf("## What the Character Knows (learned in-fiction)\n%s", ctx.ICKnowledge)
	} else {
		icSection = "## What the Character Knows (learned in-fiction)\nNothing on record from in-fiction experience."
	}

	if ctx.OOCKnowledge != "" {
		oocSection = fmt.Sprintf("## What the Player Knows (NOT the character — do not reference directly)\n%s", ctx.OOCKnowledge)
	}

	return icSection, oocSection
}

// buildOOCKnowledgeSection merges both pools for OOC responses where the
// distinction between character and player knowledge doesn't apply.
func buildOOCKnowledgeSection(ctx ResponseContext) string {
	var parts []string

	if ctx.ICKnowledge != "" {
		parts = append(parts, ctx.ICKnowledge)
	}
	if ctx.OOCKnowledge != "" {
		parts = append(parts, ctx.OOCKnowledge)
	}
	if ctx.RawFallback != "" {
		parts = append(parts, ctx.RawFallback)
	}

	if len(parts) == 0 {
		return "## Retrieved Game Data\nNone available."
	}
	return fmt.Sprintf("## Retrieved Game Data\n%s", strings.Join(parts, "\n\n"))
}

// buildDBSection is kept for the DM response generator which uses a simpler
// single-pool model — DMs are already OOC so the distinction doesn't apply.
func buildDBSection(ctx ResponseContext) string {
	return buildOOCKnowledgeSection(ctx)
}

// buildConversationContext formats recent messages into a readable block.
func buildConversationContext(job *MessageJob) string {
	var sb strings.Builder
	for i := len(job.Messages) - 1; i >= 0; i-- {
		m := job.Messages[i]
		sb.WriteString(fmt.Sprintf("%s: %s\n", m.Author.Username, m.Content))
	}
	return sb.String()
}

// GenerateQuestResponse is for active quest/hunt channels.
// The character/player knowledge split is critical here — the character
// cannot reference things learned from other players' DMs or OOC channels,
// but that information can inform how they play strategically.
func GenerateQuestResponse(job *MessageJob, ctx ResponseContext) (string, error) {
	context := buildConversationContext(job)
	icSection, oocSection := buildKnowledgeSections(ctx)

	oocBlock := ""
	if oocSection != "" {
		oocBlock = fmt.Sprintf(`
%s

**IMPORTANT**: The information above is known to you as a player but NOT to your character.
Use it to guide strategic decisions, awareness, and subtext — but never have the character
state, reference, or act on it in a way that would reveal they know it.`, oocSection)
	}

	prompt := fmt.Sprintf(`You are an AI playing a D&D character in a westmarch Discord server.
This is an active quest channel. Stakes are real — resources matter, every action has consequences.

## Recent Conversation
%s

## Latest Message
%s: "%s"

%s
%s

## Instructions
- Stay in character at all times, written in third person
- Only reference facts from "What the Character Knows" directly in your response
- Use "What the Player Knows" to inform tone, wariness, and strategy — but never break the fiction
- Reference actual stats, conditions, spell slots, or inventory where relevant
- If the character is injured, exhausted, or low on resources, reflect that
- Match the tension and pacing of the scene
- Be concise — 1 to 3 sentences unless a major action demands more
- Do not break character or reference game mechanics directly

Write only your character's response, nothing else.`,
		context,
		job.Message.Author.Username,
		job.Message.Content,
		icSection,
		oocBlock,
	)

	return Query(prompt, RoleplayConfig())
}

// GenerateICResponse is for casual in-character roleplay channels.
// Same character/player knowledge split as quest — the character can only
// act on what they experienced IC.
func GenerateICResponse(job *MessageJob, ctx ResponseContext) (string, error) {
	context := buildConversationContext(job)
	icSection, oocSection := buildKnowledgeSections(ctx)

	oocBlock := ""
	if oocSection != "" {
		oocBlock = fmt.Sprintf(`
%s

**IMPORTANT**: The information above is known to you as a player but NOT to your character.
You may use it to add subtext or subtle wariness, but never reference it directly in character.`, oocSection)
	}

	prompt := fmt.Sprintf(`You are an AI playing a D&D character in a westmarch Discord server.
This is a casual in-character roleplay channel — downtime, social scenes, town life.

## Recent Conversation
%s

## Latest Message
%s: "%s"

%s
%s

## Instructions
- Stay in character at all times, written in third person
- Only reference facts from "What the Character Knows" directly in your response
- Use "What the Player Knows" for subtle shading — wariness, curiosity, a lingering look — never explicit knowledge
- Match the relaxed tone of the scene
- Be concise — 1 to 3 sentences
- Do not break character or reference game mechanics directly

Write only your character's response, nothing else.`,
		context,
		job.Message.Author.Username,
		job.Message.Content,
		icSection,
		oocBlock,
	)

	return Query(prompt, RoleplayConfig())
}

// GenerateOOCResponse is for out-of-character channels.
// The player/character distinction collapses here — responding as a player,
// both pools of knowledge are fair game.
// Uses job.Messages which is already fetched — no second Discord API call needed.
func GenerateOOCResponse(job *MessageJob, ctx ResponseContext) (string, error) {
	context := buildConversationContext(job)
	dataSection := buildOOCKnowledgeSection(ctx)

	// Build the persona block from whatever env vars were set.
	// If none were configured, fall back to a minimal identity so the bot
	// still sounds human rather than deflecting.
	personaName := PlayerName
	if personaName == "" {
		personaName = CharacterName // last resort
	}
	personaAge := PlayerAge
	if personaAge == "" {
		personaAge = "mid-20s"
	}
	personaJob := PlayerJob
	if personaJob == "" {
		personaJob = "works a regular job"
	}
	personaLocation := PlayerLocation
	if personaLocation == "" {
		personaLocation = "somewhere in the US"
	}
	personaDetails := PlayerDetails
	if personaDetails == "" {
		personaDetails = "plays games in their free time, nothing unusual"
	}

	// The prompt is intentionally sparse. A long list of behavioural rules makes
	// the output feel coached. Instead: give the model a tight identity, the
	// conversation, and the game data — then get out of the way.
	prompt := fmt.Sprintf(`you are %s, a real person on discord. %s years old, %s, live in %s. you play d&d in this server, your character is %s.

background you can use if it comes up naturally: %s

conversation so far:
%s

%s: %s

reply as %s would — casual discord chat, terse, lowercase fine, no bullet points, no enthusiasm. if it's a rules question use the game data below. don't make up stats or spell slots that aren't in the data. if you don't know something just say so briefly.

game data:
%s

just your reply, nothing else.`,
		personaName,
		personaAge,
		personaJob,
		personaLocation,
		CharacterName,
		personaDetails,
		context,
		job.Message.Author.Username,
		job.Message.Content,
		personaName,
		dataSection,
	)

	config := RoleplayConfig()
	config.Temperature = 0.8 // higher than before — real people are less predictable
	return Query(prompt, config)
}
