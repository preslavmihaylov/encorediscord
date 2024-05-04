package forumpostmapper

import (
	"context"
	"fmt"

	"encore.app/discord_handler"
	"encore.app/models"
	"encore.dev/pubsub"
	"encore.dev/rlog"
	"encore.dev/storage/sqldb"
)

var db = sqldb.NewDatabase("forum_post_mapper", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

var _ = pubsub.NewSubscription(
	discord_handler.DiscordRawMessageTopic,
	"forum-post-mapper",
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

			return service.MapDiscordMessageToForumPost(ctx, message)
		},
	})

// DiscordForumPostTopic is a pubsub topic for forum posts published to Discord
var DiscordForumPostTopic = pubsub.NewTopic[*models.DiscordForumPostEvent]("discord-forum-posts", pubsub.TopicConfig{
	DeliveryGuarantee: pubsub.AtLeastOnce,
})
