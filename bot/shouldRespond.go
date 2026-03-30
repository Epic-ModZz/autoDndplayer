package bot

import (
	"log"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/bwmarrin/discordgo"
)

var allowedBots = map[string]bool{
	"431544605209788416": true, // Tupper
	"261302296103747584": true, // Avrae
}

var (
	cooldowns  = make(map[string]time.Time)
	cooldownMu sync.Mutex

	lastSpoke   = make(map[string]time.Time)
	lastSpokeMu sync.Mutex
)

// RecordBotSpoke tracks the last time the bot sent a message in a channel
// so recentlySpoke can gate question-response heuristics.
func RecordBotSpoke(channelID string) {
	lastSpokeMu.Lock()
	defer lastSpokeMu.Unlock()
	lastSpoke[channelID] = time.Now()
}

func recentlySpoke(channelID string) bool {
	lastSpokeMu.Lock()
	defer lastSpokeMu.Unlock()
	t, ok := lastSpoke[channelID]
	if !ok {
		return false
	}
	return time.Since(t) < 900*time.Second // 15 min — wider window for active scenes
}

// shouldRespond is the cheap heuristic gate. It decides whether a message is
// worth sending to the LLM classifier (shouldRespondWithLLM). Returning true
// here means "advance to the LLM", not "definitely respond".
//
// mode is passed in from botInit.go, which already has it from the cache.
// IC and QUEST channels are treated as active scenes where the character is a
// participant; OOC channels are more conservative.
func shouldRespond(s *discordgo.Session, m *discordgo.MessageCreate, mode ChannelMode) bool {
	log.Printf(
		"shouldRespond: mode=%s content=%q mentioned=%v reply=%v nameMatch=%v "+
			"question=%v action=%v substantive=%v cooldown=%v recentlySpoke=%v",
		mode,
		m.Content,
		isMentioned(s, m),
		isReplyToBot(s, m),
		CharacterName != "" && containsCharacterName(m.Content),
		isQuestion(m.Content),
		isRoleplayAction(m.Content),
		isSubstantive(m.Content),
		isOnCooldown(m.ChannelID),
		recentlySpoke(m.ChannelID),
	)

	// Own messages never trigger a response.
	if m.Author.ID == s.State.User.ID {
		return false
	}

	// Non-allowed bots are ignored entirely.
	if m.Author.Bot && !allowedBots[m.Author.ID] {
		return false
	}

	// ── Unambiguous signals — bypass every gate ───────────────────────────────
	// Direct pings and replies always advance to the LLM classifier regardless
	// of cooldown, channel mode, or message length.
	if isMentioned(s, m) || isReplyToBot(s, m) {
		return true
	}
	if !isPlayerAwake() {
		// Even asleep, a direct ping has a small chance of a response
		if (isMentioned(s, m) || isReplyToBot(s, m)) && rand.Intn(100) < 15 {
			return true
		}
		return false
	}
	// ── Absolute disqualifiers ────────────────────────────────────────────────
	// Apply these AFTER the unambiguous signals so a cooldown never swallows a
	// direct ping, and a 2-word name mention ("Hey Lyra") isn't thrown out
	// before we check for the character name.
	if isVeryShort(m.Content) { // < 2 words
		return false
	}
	if isOnCooldown(m.ChannelID) {
		return false
	}

	// ── Name match — always advance ───────────────────────────────────────────
	if CharacterName != "" && containsCharacterName(m.Content) {
		return true
	}

	// ── Channel-mode-aware heuristics ─────────────────────────────────────────
	switch mode {

	case ChannelModeIC, ChannelModeQuest:
		// The bot is a scene participant here, not just a rules answerer.
		// Be permissive — the LLM classifier is the real filter.

		if recentlySpoke(m.ChannelID) {
			// Active scene: questions, roleplay actions, or any substantive message
			// all warrant an LLM look. "Substantive" means 8+ words — long enough
			// to be addressing someone rather than just emoting to the void.
			if isQuestion(m.Content) || isRoleplayAction(m.Content) || isSubstantive(m.Content) {
				return true
			}
		} else {
			// Bot hasn't spoken in a while. Still respond to roleplay actions
			// (someone may be addressing the character) and direct questions.
			// Skip generic chatter.
			if isRoleplayAction(m.Content) || isQuestion(m.Content) {
				return true
			}
		}

	case ChannelModeOOC:
		// OOC: only advance on questions. The bot is a rules resource here,
		// not a conversationalist — it shouldn't chime into every social thread.
		if isQuestion(m.Content) {
			return true
		}

	default:
		// Unknown or unclassified channel: conservative fallback — same as the
		// old behaviour, but mode classification should fill this in quickly.
		if recentlySpoke(m.ChannelID) && isQuestion(m.Content) {
			return true
		}
	}

	// ── Random fallback ───────────────────────────────────────────────────────
	// Organic scene participation in IC/QUEST (~2%). Higher than before because
	// the name-match and action-detection paths above catch most intentional
	// interactions; this covers ambient presence.
	// OOC gets a much lower random chance — it would feel out of place.
	threshold := 0
	switch mode {
	case ChannelModeIC, ChannelModeQuest:
		threshold = 2
	case ChannelModeOOC:
		threshold = 0 // never random-fire in OOC
	}
	if threshold > 0 && rand.Intn(100) < threshold {
		setCooldown(m.ChannelID, 300*time.Second)
		return true
	}

	return false
}

// ── Heuristic helpers ─────────────────────────────────────────────────────────

// isVeryShort returns true for messages with fewer than 2 words.
// This is intentionally looser than the old isTooShort (4 words) so that
// short but direct messages like "Hey Lyra!" survive to the name check.
func isVeryShort(content string) bool {
	return len(strings.Fields(content)) < 2
}

// isRoleplayAction returns true if the message contains inline roleplay
// formatting — *action* or _action_ — the standard Discord RP convention.
// These frequently signal that something is happening in the scene that the
// character might react to even without being named explicitly.
var reRoleplayAction = regexp.MustCompile(`\*[^*\n]{2,}\*|_[^_\n]{2,}_`)

func isRoleplayAction(content string) bool {
	return reRoleplayAction.MatchString(content)
}

// isSubstantive returns true for messages long enough to be a real IC turn
// rather than a one-off emote or filler. Used in active scenes where the bot
// recently spoke, as a catch-all for long IC posts that don't happen to
// contain a question mark or asterisks.
func isSubstantive(content string) bool {
	return len(strings.Fields(content)) >= 8
}

// containsCharacterName checks for a whole-word, case-insensitive match of
// CharacterName in the message content. CharacterName is set in botInit.go.
func containsCharacterName(content string) bool {
	lower := strings.ToLower(content)
	name := strings.ToLower(CharacterName)
	idx := strings.Index(lower, name)
	if idx == -1 {
		return false
	}
	before := idx == 0 || !unicode.IsLetter(rune(lower[idx-1]))
	after := idx+len(name) >= len(lower) || !unicode.IsLetter(rune(lower[idx+len(name)]))
	return before && after
}

func isQuestion(content string) bool {
	trimmed := strings.TrimSpace(content)
	if strings.HasSuffix(trimmed, "?") {
		return true
	}
	lower := strings.ToLower(trimmed)
	questionStarters := []string{
		"who ", "what ", "where ", "when ", "why ", "how ",
		"is ", "are ", "do ", "does ", "can ", "will ",
		// Imperatives directed at the bot
		"tell ", "describe ", "explain ", "show ", "give ",
		"list ", "help ", "say ", "talk ",
	}
	for _, starter := range questionStarters {
		if strings.HasPrefix(lower, starter) {
			return true
		}
	}
	return false
}

func isMentioned(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	for _, user := range m.Mentions {
		if user.ID == s.State.User.ID {
			return true
		}
	}
	return false
}

func isReplyToBot(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	if m.ReferencedMessage == nil {
		return false
	}
	return m.ReferencedMessage.Author.ID == s.State.User.ID
}

func isOnCooldown(channelID string) bool {
	cooldownMu.Lock()
	defer cooldownMu.Unlock()
	return time.Now().Before(cooldowns[channelID])
}

func setCooldown(channelID string, duration time.Duration) {
	cooldownMu.Lock()
	defer cooldownMu.Unlock()
	cooldowns[channelID] = time.Now().Add(duration)
}

// isPlayerAwake returns false during hours when a real person in the player's
// timezone would typically be asleep. The bot won't respond at 3am local time.
// A small random exception (~5%) simulates the occasional insomniac session.
func isPlayerAwake() bool {
	if PlayerTimezone == "" {
		return true
	}

	loc, err := time.LoadLocation(PlayerTimezone)
	if err != nil {
		log.Printf("isPlayerAwake: invalid timezone %q: %v", PlayerTimezone, err)
		return true
	}

	playerAwakeMu.Lock()
	defer playerAwakeMu.Unlock()

	// Cached state is still valid — return it without re-rolling.
	if playerAwakeState != nil && time.Now().Before(playerAwakeUntil) {
		return *playerAwakeState
	}

	// State has expired — evaluate a new one.
	hour := time.Now().In(loc).Hour()
	var awake bool
	var stateDuration time.Duration

	if hour >= 0 && hour < 8 {
		// Sleep window. 5% chance they're up (insomniac / late night gaming).
		// If asleep, cache until 8am so they don't randomly wake up mid-night.
		awake = rand.Intn(100) < 5
		if awake {
			// They're up — but only for a short burst (20-45 min)
			stateDuration = time.Duration(20+rand.Intn(25)) * time.Minute
		} else {
			// Asleep until morning — calculate time until 8am
			now := time.Now().In(loc)
			tomorrow8am := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, loc)
			if now.Hour() >= 8 {
				tomorrow8am = tomorrow8am.Add(24 * time.Hour)
			}
			stateDuration = time.Until(tomorrow8am)
		}
	} else {
		// Waking hours — always awake, cache for 30-90 minutes
		awake = true
		stateDuration = time.Duration(30+rand.Intn(60)) * time.Minute
	}

	playerAwakeState = &awake
	playerAwakeUntil = time.Now().Add(stateDuration)
	log.Printf("isPlayerAwake: new state — awake=%v, valid for %v", awake, stateDuration.Round(time.Minute))
	return awake
}
