package discord

import (
	"fmt"
	"sort"

	"github.com/bwmarrin/discordgo"
)

type Role struct {
	ID   string
	Name string
}

func ListRoles(token, guildID string) ([]Role, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("discord session: %w", err)
	}
	roles, err := dg.GuildRoles(guildID)
	if err != nil {
		return nil, fmt.Errorf("list guild roles: %w", err)
	}
	sort.Slice(roles, func(i, j int) bool {
		if roles[i].Position != roles[j].Position {
			return roles[i].Position > roles[j].Position
		}
		return roles[i].Name < roles[j].Name
	})
	out := make([]Role, 0, len(roles))
	for _, role := range roles {
		if role.ID == guildID {
			continue
		}
		out = append(out, Role{ID: role.ID, Name: role.Name})
	}
	return out, nil
}
