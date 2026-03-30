package bot

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
)

type ChannelMode int

const (
	ChannelModeUnknown ChannelMode = iota
	ChannelModeIC                  // casual in-character roleplay
	ChannelModeQuest               // active quest/hunt — wrap-up detection active, higher stakes
	ChannelModeOOC                 // out-of-character
)

func (m ChannelMode) String() string {
	switch m {
	case ChannelModeIC:
		return "IC"
	case ChannelModeQuest:
		return "QUEST"
	case ChannelModeOOC:
		return "OOC"
	default:
		return "UNKNOWN"
	}
}

var (
	channelModeCache   = make(map[string]ChannelMode)
	channelModeCacheMu sync.RWMutex
)

func classifyChannelWithLLM(s *discordgo.Session, channelID string) (ChannelMode, error) {
	channel, err := s.Channel(channelID)
	if err != nil {
		return ChannelModeUnknown, err
	}

	var parentName string
	if channel.ParentID != "" {
		parent, err := s.Channel(channel.ParentID)
		if err == nil {
			parentName = parent.Name
		}
	}

	messages, err := s.ChannelMessages(channelID, 15, "", "", "")
	if err != nil {
		return ChannelModeUnknown, err
	}

	var sample strings.Builder
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		sample.WriteString(fmt.Sprintf("%s: %s\n", m.Author.Username, m.Content))
	}

	prompt := fmt.Sprintf(`You are helping classify a Discord channel in a D&D westmarch server.

Channel name: #%s
Parent channel/category: %s

Sample of recent messages:
%s

Classify this channel as exactly one of the following:
- QUEST  (an active adventure, hunt, or mission — characters are out in the world doing something with stakes, combat is likely or ongoing, a DM is running a scene)
- IC     (casual in-character roleplay — characters talking in town, tavern scenes, downtime, social RP with no active quest)
- OOC    (out-of-character — game mechanics, character applications, rules questions, meta discussion, bot commands)
- UNKNOWN (cannot tell, mixed use, or no messages)

Reply with only QUEST, IC, OOC, or UNKNOWN.`,
		channel.Name,
		parentName,
		sample.String(),
	)

	response, err := Query(prompt, ClassifierConfig())
	if err != nil {
		return ChannelModeUnknown, err
	}

	switch strings.TrimSpace(strings.ToUpper(response)) {
	case "QUEST":
		return ChannelModeQuest, nil
	case "IC":
		return ChannelModeIC, nil
	case "OOC":
		return ChannelModeOOC, nil
	default:
		return ChannelModeUnknown, nil
	}
}

func getChannelMode(s *discordgo.Session, channelID string) ChannelMode {
	channelModeCacheMu.RLock()
	if mode, ok := channelModeCache[channelID]; ok {
		channelModeCacheMu.RUnlock()
		return mode
	}
	channelModeCacheMu.RUnlock()

	mode, err := classifyChannelWithLLM(s, channelID)
	if err != nil {
		log.Printf("failed to classify channel %s: %v", channelID, err)
		return ChannelModeUnknown
	}

	channelModeCacheMu.Lock()
	channelModeCache[channelID] = mode
	channelModeCacheMu.Unlock()

	return mode
}
