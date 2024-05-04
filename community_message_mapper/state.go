package communitymessagemapper

import (
	"context"
	"fmt"

	"encore.app/discord_handler"
	"encore.app/models"
	"encore.dev/pubsub"
	"encore.dev/rlog"
	"encore.dev/storage/sqldb"
)

var db = sqldb.NewDatabase("community_message_mapper", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

var _ = pubsub.NewSubscription(
	discord_handler.DiscordRawMessageTopic,
	"community-message-mapper",
	pubsub.SubscriptionConfig[*models.DiscordRawMessage]{
		RetryPolicy: &pubsub.RetryPolicy{
			MaxRetries: 5,
		},
		Handler: func(ctx context.Context, message *models.DiscordRawMessage) error {
			rlog.Info("Received raw discord message", "discordMessage", message)
			service, err := initService()
			if err != nil {
				return fmt.Errorf("couldn't create service: %w", err)
			}

			return service.MapDiscordMessageToCommunityMessage(ctx, message)
		},
	})

// DiscordCommunityMessageTopic is a pubsub topic for community messages published to Discord
var DiscordCommunityMessageTopic = pubsub.NewTopic[*models.DiscordCommunityMessageEvent]("discord-community-messages", pubsub.TopicConfig{
	DeliveryGuarantee: pubsub.AtLeastOnce,
})
