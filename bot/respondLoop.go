package bot

import (
	"log"
	"sync"

	"github.com/bwmarrin/discordgo"
)

// characterMu serializes the full read→generate pipeline for the character.
//
// Without this, two IC channels processing simultaneously would both read the
// same DB snapshot and produce responses that contradict each other — one
// might reference full spell slots while the other already spent them.
//
// Holding this lock for the duration of processMessage means one channel
// waits while the other finishes. Messages queue up naturally via messageQueue.
//
// When multi-character support is added, replace this with a
// map[int]*sync.Mutex keyed by character_id, initialized lazily.
var characterMu sync.Mutex

// ResponseContext carries whatever data survived the pipeline stages.
type ResponseContext struct {
	// ICKnowledge is what the CHARACTER learned through in-fiction experience.
	// Safe to reference directly in IC responses.
	ICKnowledge string

	// OOCKnowledge is what the PLAYER knows through OOC channels or DMs.
	// The character cannot reference this directly but it can inform strategy.
	OOCKnowledge string

	// RawFallback holds unsummarized query results used when the summarizer
	// returned empty but the queries did return rows.
	RawFallback string

	// ContextOnly is true when GatherInfo produced no SQL — the LLM decided
	// no DB data was needed and the response should rely on conversation context.
	ContextOnly bool
}

func messageWorker() {
	for job := range messageQueue {
		processMessage(job)
	}
}

func processMessage(job *MessageJob) {
	// Serialize per-character: only one channel's pipeline runs at a time.
	// This prevents two concurrent channels from reading stale DB state and
	// generating contradictory responses.
	characterMu.Lock()
	defer characterMu.Unlock()

	log.Printf("processMessage: start — channel=%s mode=%s", job.Message.ChannelID, job.Mode)
	job.Session.ChannelTyping(job.Message.ChannelID)

	ctx := ResponseContext{}

	// Stage 1: gather context.
	// For OOC channels, GatherInfo returns pre-formatted DB rows directly —
	// no SQL fences, no RunQueries needed. For IC/QUEST it returns LLM-generated
	// SQL that still needs to be executed.
	log.Println("processMessage: calling GatherInfo")
	gatherResult, err := GatherInfo(job)
	if err != nil {
		log.Println("GatherInfo error:", err)
		return
	}
	log.Printf("processMessage: GatherInfo done — sql_len=%d", len(gatherResult))
	if gatherResult == "" {
		log.Println("GatherInfo returned nothing — responding from context only")
		ctx.ContextOnly = true
	}

	// Stage 2: for OOC, GatherInfo already ran the queries and returned plain
	// formatted results. Skip RunQueries entirely and treat the output as the
	// raw fallback — the OOC response generator uses it directly.
	// For IC/QUEST, run the generated SQL through the normal pipeline.
	var formattedResults string
	if !ctx.ContextOnly {
		if job.Mode == ChannelModeOOC {
			// Pre-formatted rows from gatherOOCCharacterContext — use as-is.
			formattedResults = gatherResult
		} else {
			job.Session.ChannelTyping(job.Message.ChannelID)
			queryResults, err := RunQueries(gatherResult)
			if err != nil {
				log.Println("RunQueries error (falling back to context-only):", err)
				ctx.ContextOnly = true
			} else {
				formattedResults = FormatResultsForLLM(queryResults)
				if formattedResults == "" {
					log.Println("RunQueries returned no rows — DB may not be seeded yet, falling back to context-only")
					ctx.ContextOnly = true
				}
			}
		}
	}

	// Stage 3: summarize raw results, partitioned by knowledge source.
	// OOC skips the summarizer — the data is already compact and the
	// IC/OOC knowledge split doesn't apply in an out-of-character channel.
	if !ctx.ContextOnly {
		if job.Mode == ChannelModeOOC {
			ctx.RawFallback = formattedResults
		} else {
			log.Println("processMessage: running SummarizeDBResults")
			job.Session.ChannelTyping(job.Message.ChannelID)
			knowledge, err := SummarizeDBResults(formattedResults, job)
			log.Println("processMessage: SummarizeDBResults done")
			if err != nil {
				log.Println("SummarizeDBResults error (using raw results):", err)
				ctx.RawFallback = formattedResults
			} else if knowledge.IC == "" && knowledge.OOC == "" {
				log.Println("Summarizer found nothing relevant — falling back to raw results")
				ctx.RawFallback = formattedResults
			} else {
				ctx.ICKnowledge = knowledge.IC
				ctx.OOCKnowledge = knowledge.OOC
			}
		}
	}

	// Stage 4: route to the correct response generator.
	// Mode is already known from the job — no cache re-read needed.
	job.Session.ChannelTyping(job.Message.ChannelID)

	if job.Mode == ChannelModeUnknown {
		log.Println("Channel mode unknown — dropping message until channel is classified")
		return
	}

	var response string
	switch job.Mode {
	case ChannelModeQuest:
		response, err = GenerateQuestResponse(job, ctx)
	case ChannelModeIC:
		response, err = GenerateICResponse(job, ctx)
	default:
		response, err = GenerateOOCResponse(job, ctx)
	}
	if err != nil {
		log.Println("GenerateResponse error:", err)
		return
	}

	// Stage 5: send the response.
	_, err = job.Session.ChannelMessageSend(job.Message.ChannelID, response)
	if err != nil {
		log.Println("Error sending message:", err)
		return
	}

	RecordBotSpoke(job.Message.ChannelID)

	// Stage 6: write memory — fire and forget after the response is sent.
	// Pass IC knowledge as the "already known" baseline so the memory writer
	// only persists things that genuinely changed or are new.
	dbContext := ctx.ICKnowledge
	if dbContext == "" {
		dbContext = ctx.RawFallback
	}
	go WriteMemory(job, response, dbContext)
}

// MessageJob is the unit of work passed through the pipeline.
// Mode is set at enqueue time so processMessage never needs to re-read the cache.
type MessageJob struct {
	Session  *discordgo.Session
	Message  *discordgo.MessageCreate
	Messages []*discordgo.Message
	Mode     ChannelMode
}
