package bot

import (
	dbpkg "PCL/db/SQL_CharStats"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// avraeXPListener holds a filtered channel waiting for Avrae's !xp response.
type avraeXPListener struct {
	ch    chan *discordgo.Message
	match func(*discordgo.Message) bool
}

var (
	xpListeners   = make(map[string]*avraeXPListener)
	xpListenersMu sync.Mutex
)

// XPParseResult is what the LLM returns from the wrap-up call.
type XPParseResult struct {
	QuestOver bool `json:"quest_over"`
	XP        int  `json:"xp"`
	Gold      int  `json:"gold"`
}

var (
	wrapUpRegex  = regexp.MustCompile(`(?i)\b(\d+)\s*(xp|experience|gold|gp|g)\b`)
	levelUpRegex = regexp.MustCompile(`(?i)levels up to (\d+)`)
	xpEmbedRegex = regexp.MustCompile(`(?i)\b(xp|experience|exp)\b`)
)

// isXPResponse returns true if the message looks like Avrae's reply to !xp.
func isXPResponse(m *discordgo.Message) bool {
	if len(m.Embeds) == 0 {
		return false
	}
	for _, embed := range m.Embeds {
		combined := embed.Title + " " + embed.Description
		if xpEmbedRegex.MatchString(combined) {
			return true
		}
		for _, field := range embed.Fields {
			if xpEmbedRegex.MatchString(field.Name + " " + field.Value) {
				return true
			}
		}
	}
	return false
}

// ContainsWrapUpLanguage is the cheap regex gate run on every IC message.
func ContainsWrapUpLanguage(content string) bool {
	return wrapUpRegex.MatchString(content)
}

// ParseWrapUp sends a single LLM call to check if the quest is over
// and parse XP and gold amounts from the message context.
func ParseWrapUp(job *MessageJob) (*XPParseResult, error) {
	var context strings.Builder
	for i := len(job.Messages) - 1; i >= 0; i-- {
		m := job.Messages[i]
		context.WriteString(fmt.Sprintf("%s: %s\n", m.Author.Username, m.Content))
	}

	prompt := fmt.Sprintf(`You are monitoring a D&D westmarch Discord server.
Based on the recent messages below, answer the following:
1. Has the current quest or hunt concluded? Look for signals like:
   - DM describing the party returning to town
   - Loot or XP being distributed
   - Quest wrap-up narration
   - Players saying goodbye or going their separate ways
2. How much XP was awarded in total? (0 if none)
3. How much gold was awarded in total? (0 if none)
 
Recent messages:
%s
 
Latest message from %s: "%s"
 
Reply with ONLY a JSON object in this exact format, no other text:
{"quest_over": true/false, "xp": number, "gold": number}`,
		context.String(),
		job.Message.Author.Username,
		job.Message.Content,
	)

	config := ClassifierConfig()
	config.NumCtx = 4096

	response, err := Query(prompt, config)
	if err != nil {
		return nil, fmt.Errorf("wrap-up LLM call failed: %w", err)
	}

	cleaned := strings.TrimSpace(response)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var result XPParseResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("failed to parse wrap-up response: %w", err)
	}

	return &result, nil
}

// RegisterXPListener registers a listener that only fires for messages
// passing the provided predicate.
func RegisterXPListener(channelID string, match func(*discordgo.Message) bool) *avraeXPListener {
	listener := &avraeXPListener{
		ch:    make(chan *discordgo.Message, 1),
		match: match,
	}
	xpListenersMu.Lock()
	xpListeners[channelID] = listener
	xpListenersMu.Unlock()
	return listener
}

// DeregisterXPListener removes the listener for a channel.
func DeregisterXPListener(channelID string) {
	xpListenersMu.Lock()
	delete(xpListeners, channelID)
	xpListenersMu.Unlock()
}

// NotifyXPListener forwards a message to the listener only if it passes
// the registered predicate.
func NotifyXPListener(channelID string, m *discordgo.Message) {
	xpListenersMu.Lock()
	listener, ok := xpListeners[channelID]
	xpListenersMu.Unlock()
	if !ok {
		return
	}
	if listener.match != nil && !listener.match(m) {
		return
	}
	select {
	case listener.ch <- m:
	default:
	}
}

// ParseLevelUp checks Avrae's XP embed for a level-up notification.
func ParseLevelUp(m *discordgo.Message) (bool, int) {
	for _, embed := range m.Embeds {
		for _, text := range []string{embed.Description, embed.Title} {
			matches := levelUpRegex.FindStringSubmatch(text)
			if len(matches) > 1 {
				if level, err := strconv.Atoi(matches[1]); err == nil {
					return true, level
				}
			}
		}
	}
	return false, 0
}

// HandleWrapUp is the main orchestrator — called when wrap-up language is detected.
func HandleWrapUp(s *discordgo.Session, job *MessageJob) {
	result, err := ParseWrapUp(job)
	if err != nil {
		log.Println("ParseWrapUp error:", err)
		return
	}
	if !result.QuestOver {
		return
	}

	channelID := job.Message.ChannelID

	if result.XP > 0 {
		listener := RegisterXPListener(channelID, isXPResponse)
		defer DeregisterXPListener(channelID)

		_, err := s.ChannelMessageSend(channelID, fmt.Sprintf("!xp +%d", result.XP))
		if err != nil {
			log.Println("Failed to post !xp command:", err)
			return
		}

		var avraeMsg *discordgo.Message
		select {
		case avraeMsg = <-listener.ch:
		case <-time.After(15 * time.Second):
			log.Println("Timed out waiting for Avrae XP response")
			return
		}

		leveledUp, newLevel := ParseLevelUp(avraeMsg)
		if leveledUp {
			log.Printf("Level up detected! New level: %d", newLevel)

			// Store the pending level-up record as before, then kick off
			// AI deliberation in the background so we don't block the wrap-up.
			if err := StorePendingLevelUp(newLevel); err != nil {
				log.Println("Failed to store pending level up:", err)
			}
			go HandleLevelUp(s, newLevel, channelID)
		}
	}

	if result.Gold > 0 {
		_, err := s.ChannelMessageSend(channelID, fmt.Sprintf("!g +%d", result.Gold))
		if err != nil {
			log.Println("Failed to post !g command:", err)
		}
	}
}

// StorePendingLevelUp stores the pending level-up marker in the DB.
// The actual AI decisions are stored separately in character_levelup_decisions.
func StorePendingLevelUp(newLevel int) error {
	return dbpkg.UpsertRow("character_pending_levelup", map[string]interface{}{
		"character_id": 1,
		"new_level":    newLevel,
		"pending":      true,
	}, "character_id")
}
