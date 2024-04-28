package discord_handler

import (
	"encore.app/models"
	"encore.dev/pubsub"
)

// DiscordRawMessageTopic is the pubsub topic for raw inbound Discord messages.
var DiscordRawMessageTopic = pubsub.NewTopic[*models.DiscordRawMessage]("discord-messages", pubsub.TopicConfig{
	DeliveryGuarantee: pubsub.AtLeastOnce,
})
