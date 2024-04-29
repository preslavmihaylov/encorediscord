package models

import (
	"github.com/bwmarrin/discordgo"
)

type DiscordRawMessage struct {
	ID              string                    `json:"id"`
	InteractionType discordgo.InteractionType `json:"type"`
	ChannelID       string                    `json:"channelId"`
	GuildID         string                    `json:"guildId"`
	AuthorID        string                    `json:"authorId"`
	Content         string                    `json:"content"`
	CleanContent    string                    `json:"cleanContent"`
}

type ConversationAlert struct {
	ID        string   `json:"id"`
	Keywords  []string `json:"keywords"`
	Topics    []string `json:"topics"`
	ChannelID string   `json:"channel"`
}
