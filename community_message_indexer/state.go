package communitymessageindexer

import (
	"context"

	communitymessagemapper "encore.app/community_message_mapper"
	"encore.app/models"
	"encore.dev/pubsub"
	"encore.dev/storage/sqldb"
)

var db = sqldb.NewDatabase("discord_messages", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

var _ = pubsub.NewSubscription(
	communitymessagemapper.DiscordCommunityMessageTopic,
	"community-message-indexer",
	pubsub.SubscriptionConfig[*models.DiscordCommunityMessageEvent]{
		Handler: func(ctx context.Context, message *models.DiscordCommunityMessageEvent) error {
			return persistDiscordMessage(ctx, message)
		},
	})
