package bot

import (
	dbpkg "PCL/db/SQL_CharStats"
	"log"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	BotToken         string
	CharacterName    string
	PlayerName       string
	PlayerAge        string
	PlayerJob        string
	PlayerLocation   string
	PlayerDetails    string
	PlayerTimezone   string
	playerAwakeState *bool
	playerAwakeUntil time.Time
	playerAwakeMu    sync.Mutex
	messageQueue     chan *MessageJob
)

func Run() {
	log.Println("Run() started — handlers being registered")
	messageQueue = make(chan *MessageJob, 100)
	go messageWorker()

	discord, err := discordgo.New(BotToken)
	if err != nil {
		log.Fatal("Error creating session: ", err)
	}

	discord.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36"
	discord.Identify.Properties = discordgo.IdentifyProperties{
		OS:                "Windows",
		Browser:           "Chrome",
		Device:            "",
		SystemLocale:      "en-US",
		BrowserVersion:    "123.0.0.0",
		OSVersion:         "",
		Referrer:          "",
		ReferringDomain:   "",
		ReleaseChannel:    "stable",
		ClientBuildNumber: 359213,
	}

	discord.Identify.Intents = 0
	discord.Identify.Capabilities = 14333
	discord.Identify.GuildSubscriptions = true
	discord.Identify.LargeThreshold = 250
	discord.Identify.Presence = discordgo.GatewayStatusUpdate{
		Status: "online",
	}
	discord.ShouldReconnectOnError = true

	discord.AddHandler(messageCreate)
	discord.AddHandler(channelCreate)
	discord.AddHandler(threadCreate)

	discord.AddHandler(func(s *discordgo.Session, e *discordgo.Disconnect) {
		log.Println("Disconnected from gateway")
	})
	discord.AddHandler(func(s *discordgo.Session, e *discordgo.Connect) {
		log.Println("Connected to gateway")
	})
	discord.AddHandler(func(s *discordgo.Session, e *discordgo.Event) {
		if e.Type != "READY" {
			log.Printf("Raw event: %s | %s", e.Type, string(e.RawData))
		}
	})

	discord.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Println("READY received")
	})

	// READY_SUPPLEMENTAL fires after READY and signals the session is fully
	// established. We read guild IDs directly from s.State which discordgo
	// populates synchronously before any handler fires — no race condition.
	discord.AddHandler(func(s *discordgo.Session, e *discordgo.Event) {
		if e.Type != "READY_SUPPLEMENTAL" {
			return
		}

		s.State.RLock()
		var ids []string
		for _, g := range s.State.Guilds {
			ids = append(ids, g.ID)
		}
		s.State.RUnlock()

		if len(ids) == 0 {
			return
		}

		log.Println("READY_SUPPLEMENTAL — starting message poller")
		go StartMessagePoller(s, ids, 5*time.Second)
	})

	log.Println("All handlers registered — opening connection")
	err = discord.Open()
	if err != nil {
		log.Fatal("Error opening connection: ", err)
	}
	log.Printf("Logged in as: %s#%s", discord.State.User.Username, discord.State.User.Discriminator)
	log.Println("Bot is running.")
	select {}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	log.Printf("messageCreate fired: %s: %s", m.Author.Username, m.Content)
	if m.Author.ID == s.State.User.ID {
		log.Println("Skipping: own message")
		return
	}

	// Deduplicate against the polling pipeline. The poller calls markSeen
	// when it dispatches a message. If the gateway also delivers the same
	// message ID, markSeen returns false here and we drop the duplicate.
	// Messages that arrive only via the gateway (e.g. during a poller gap)
	// will be new to markSeen and will return true — processed normally.
	if !markSeen(m.ID) {
		log.Printf("messageCreate: dropping duplicate gateway event for message %s", m.ID)
		return
	}

	if !m.Author.Bot {
		go upsertDiscordUser(m)
	}

	if m.GuildID == "" {
		log.Println("Routing to DM handler")
		HandleDMCommand(s, m)
		return
	}

	if m.Author.Bot && allowedBots[m.Author.ID] {
		NotifyXPListener(m.ChannelID, m.Message)
		return
	}

	channelModeCacheMu.RLock()
	mode, exists := channelModeCache[m.ChannelID]
	channelModeCacheMu.RUnlock()

	log.Printf("Channel %s mode: %s, exists: %v", m.ChannelID, mode, exists)

	if !exists || mode == ChannelModeUnknown {
		// Classify synchronously so this message isn't lost. The LLM call
		// takes a few seconds but the alternative is silently dropping the
		// message — the cursor has already advanced past it so the poller
		// will never retry it.
		log.Printf("Channel %s not yet classified — classifying now (blocking)", m.ChannelID)
		newMode, err := classifyChannelWithLLM(s, m.ChannelID)
		if err != nil {
			log.Printf("failed to classify channel %s: %v — dropping message", m.ChannelID, err)
			return
		}
		mode = newMode
		log.Printf("Channel %s classified as %s", m.ChannelID, mode)

		if mode == ChannelModeUnknown {
			// Don't cache UNKNOWN — the classifier may have had no messages to
			// work with (e.g. called before seedCursors finished). Leave the
			// cache empty so the next message retries classification fresh.
			log.Printf("Channel %s returned UNKNOWN — not caching, dropping this message", m.ChannelID)
			return
		}

		channelModeCacheMu.Lock()
		channelModeCache[m.ChannelID] = mode
		channelModeCacheMu.Unlock()
	}

	if mode == ChannelModeQuest && ContainsWrapUpLanguage(m.Content) {
		go func() {
			messages, err := s.ChannelMessages(m.ChannelID, 10, "", "", "")
			if err != nil {
				log.Println("Failed to fetch messages for wrap-up check:", err)
				return
			}
			job := &MessageJob{
				Session:  s,
				Message:  m,
				Messages: messages,
				Mode:     mode,
			}
			HandleWrapUp(s, job)
		}()
	}

	if !shouldRespond(s, m, mode) {
		return
	}

	messages, err := s.ChannelMessages(m.ChannelID, 10, "", "", "")
	if err != nil {
		log.Printf("failed to fetch messages for channel %s: %v", m.ChannelID, err)
		return
	}

	job := &MessageJob{
		Session:  s,
		Message:  m,
		Messages: messages,
		Mode:     mode,
	}

	// If the bot was recently active in this channel and is named directly,
	// that's enough — don't second-guess it with the classifier.
	// The LLM classifier is valuable for ambiguous IC scenes; for OOC
	// conversations where the bot's name appears mid-thread it adds latency
	// and is occasionally wrong.
	if containsCharacterName(m.Content) && recentlySpoke(m.ChannelID) {
		log.Println("Name match + recently spoke — bypassing LLM classifier")
	} else {
		respond, err := shouldRespondWithLLM(job)
		if err != nil {
			log.Println("LLM classifier error:", err)
			return
		}
		if !respond {
			return
		}
	}

	select {
	case messageQueue <- job:
	default:
		log.Println("Queue full, dropping message from:", m.Author.Username)
	}
}

func upsertDiscordUser(m *discordgo.MessageCreate) {
	if dbpkg.DB == nil {
		return
	}

	displayName := m.Author.GlobalName
	if displayName == "" {
		displayName = m.Author.Username
	}

	_, err := dbpkg.DB.Exec(`
		INSERT INTO discord_users
			(discord_user_id, username, display_name, is_dm, is_bot, first_seen_at, last_seen_at)
		VALUES (?, ?, ?, 0, 0, datetime('now'), datetime('now'))
		ON CONFLICT(discord_user_id) DO UPDATE SET
			username      = excluded.username,
			display_name  = excluded.display_name,
			last_seen_at  = datetime('now')`,
		m.Author.ID,
		m.Author.Username,
		displayName,
	)
	if err != nil {
		log.Printf("upsertDiscordUser failed for %s: %v", m.Author.Username, err)
	}
}

func channelCreate(s *discordgo.Session, c *discordgo.ChannelCreate) {
	mode := getChannelMode(s, c.ID)
	log.Printf("New channel #%s classified as %s", c.Name, mode)
}

func threadCreate(s *discordgo.Session, t *discordgo.ThreadCreate) {
	mode := getChannelMode(s, t.ID)
	log.Printf("New thread #%s classified as %s", t.Name, mode)
}
