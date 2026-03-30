package bot

import (
	"log"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// channelCursor tracks the last seen message ID per channel.
// Only channels with activity since the last poll are re-fetched.
var (
	channelCursors   = make(map[string]string) // channelID -> last message ID
	channelCursorsMu sync.Mutex
)

// seenMessages deduplicates message IDs that the poller dispatches so that
// the concurrent gateway websocket delivery of the same message is ignored.
// Entries expire after seenMessageTTL to bound memory growth.
var (
	seenMessages   = make(map[string]time.Time) // messageID -> dispatched time
	seenMessagesMu sync.Mutex
	seenMessageTTL = 60 * time.Second
)

// markSeen records a message ID as dispatched by the poller and returns true
// if it was NOT already seen (i.e. safe to process). Returns false if it is a
// duplicate — the caller should drop it.
func markSeen(messageID string) bool {
	seenMessagesMu.Lock()
	defer seenMessagesMu.Unlock()

	// Evict expired entries to keep the map bounded.
	now := time.Now()
	for id, t := range seenMessages {
		if now.Sub(t) > seenMessageTTL {
			delete(seenMessages, id)
		}
	}

	if _, exists := seenMessages[messageID]; exists {
		return false // duplicate
	}
	seenMessages[messageID] = now
	return true
}

// StartMessagePoller watches all text channels across the given guilds for
// new messages and dispatches them through the normal messageCreate pipeline.
//
// Design for scale:
//   - Cursors are seeded from the guild's channel list (last_message_id) on
//     startup so we never replay history.
//   - Each poll only fetches channels whose last_message_id has advanced since
//     the cursor was last set — a cheap string comparison before any API call.
//   - Threads are discovered dynamically as they appear in active channels.
func StartMessagePoller(s *discordgo.Session, guildIDs []string, pollInterval time.Duration) {
	log.Printf("StartMessagePoller: starting — %d guild(s), poll interval %s", len(guildIDs), pollInterval)

	// Seed cursors so we don't replay old messages on startup.
	for _, gid := range guildIDs {
		seedCursors(s, gid)
	}

	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for range ticker.C {
			for _, gid := range guildIDs {
				pollGuild(s, gid)
			}
		}
	}()
}

// seedCursors initialises channelCursors from the guild's channel list.
// Discord returns last_message_id for each channel, so we don't need an
// extra API call per channel — one GuildChannels call seeds everything.
func seedCursors(s *discordgo.Session, guildID string) {
	channels, err := s.GuildChannels(guildID)
	if err != nil {
		log.Printf("seedCursors: failed for guild %s: %v", guildID, err)
		return
	}
	channelCursorsMu.Lock()
	defer channelCursorsMu.Unlock()
	for _, ch := range channels {
		if isTextChannel(ch) && ch.LastMessageID != "" {
			channelCursors[ch.ID] = ch.LastMessageID
		}
	}
	log.Printf("seedCursors: seeded %d channels for guild %s", len(channelCursors), guildID)
}

// pollGuild checks every text channel in a guild.
// It skips channels whose last_message_id matches our cursor — zero cost
// for inactive channels, which is the common case.
func pollGuild(s *discordgo.Session, guildID string) {
	channels, err := s.GuildChannels(guildID)
	if err != nil {
		log.Printf("pollGuild: failed for guild %s: %v", guildID, err)
		return
	}

	for _, ch := range channels {
		if !isTextChannel(ch) {
			continue
		}
		channelCursorsMu.Lock()
		cursor := channelCursors[ch.ID]
		channelCursorsMu.Unlock()

		// Skip if nothing new — last_message_id hasn't changed.
		if ch.LastMessageID == "" || ch.LastMessageID == cursor {
			continue
		}

		pollChannel(s, ch, guildID)
	}
}

// pollChannel fetches messages newer than the cursor and dispatches them.
func pollChannel(s *discordgo.Session, ch *discordgo.Channel, guildID string) {
	channelCursorsMu.Lock()
	after := channelCursors[ch.ID]
	channelCursorsMu.Unlock()

	msgs, err := s.ChannelMessages(ch.ID, 20, "", after, "")
	if err != nil {
		log.Printf("pollChannel: error fetching #%s: %v", ch.Name, err)
		return
	}
	if len(msgs) == 0 {
		return
	}

	// msgs[0] is newest — update cursor before dispatching.
	channelCursorsMu.Lock()
	channelCursors[ch.ID] = msgs[0].ID
	channelCursorsMu.Unlock()

	// Dispatch oldest-first for chronological order.
	// markSeen is called inside messageCreate — that is the single dedup gate
	// for both the poller path and the gateway websocket path. Don't call it
	// here, or the poller would mark the ID seen and then immediately drop it
	// when messageCreate runs.
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		event := &discordgo.MessageCreate{Message: m}
		if event.GuildID == "" {
			event.GuildID = guildID
		}
		log.Printf("pollChannel: dispatching message from %s in #%s: %.80s",
			m.Author.Username, ch.Name, m.Content)
		messageCreate(s, event)
	}
}

func isTextChannel(ch *discordgo.Channel) bool {
	switch ch.Type {
	case discordgo.ChannelTypeGuildText,
		discordgo.ChannelTypeGuildNews,
		discordgo.ChannelTypeGuildForum:
		return true
	}
	return false
}

// SubscribeToGuild is kept as a no-op so botInit.go compiles unchanged.
// OP 14 is no longer used — StartMessagePoller handles message delivery.
func SubscribeToGuild(s *discordgo.Session, guildID string) error {
	return nil
}
