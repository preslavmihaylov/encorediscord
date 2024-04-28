package discord_handler

import (
	"encore.dev/pubsub"
	"github.com/bwmarrin/discordgo"
)

type DiscordRawMessageEvent struct {
	InteractionType discordgo.InteractionType `json:"type"`
	ChannelID       string                    `json:"channelId"`
	GuildID         string                    `json:"guildId"`
	ID              string                    `json:"id"`
	Content         string                    `json:"content"`
	CleanContent    string                    `json:"cleanContent"`
	AuthorID        string                    `json:"authorId"`
}

// DiscordRawMessageTopic is the pubsub topic for raw inbound Discord messages.
var DiscordRawMessageTopic = pubsub.NewTopic[*DiscordRawMessageEvent]("discord-messages", pubsub.TopicConfig{
	DeliveryGuarantee: pubsub.AtLeastOnce,
})

