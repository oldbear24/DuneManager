package discord

import (
	"fmt"
	"sort"

	"github.com/bwmarrin/discordgo"
)

// Channel is a guild text channel with its category name resolved.
type Channel struct {
	ID           string
	Name         string
	CategoryName string // empty if uncategorised
	Position     int
}

// ListChannels returns all text channels for the guild, sorted by category
// name then channel position within the category.
func ListChannels(token, guildID string) ([]Channel, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("discord session: %w", err)
	}
	raw, err := dg.GuildChannels(guildID)
	if err != nil {
		return nil, fmt.Errorf("list guild channels: %w", err)
	}

	// Map category IDs to names.
	categories := map[string]string{}
	for _, ch := range raw {
		if ch.Type == discordgo.ChannelTypeGuildCategory {
			categories[ch.ID] = ch.Name
		}
	}

	var result []Channel
	for _, ch := range raw {
		if ch.Type != discordgo.ChannelTypeGuildText && ch.Type != discordgo.ChannelTypeGuildNews {
			continue
		}
		catName := ""
		if ch.ParentID != "" {
			catName = categories[ch.ParentID]
		}
		result = append(result, Channel{
			ID:           ch.ID,
			Name:         ch.Name,
			CategoryName: catName,
			Position:     ch.Position,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].CategoryName != result[j].CategoryName {
			return result[i].CategoryName < result[j].CategoryName
		}
		return result[i].Position < result[j].Position
	})
	return result, nil
}
