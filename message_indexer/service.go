package messageindexer

import (
	"context"
	"fmt"
	"strings"

	"encore.app/discord_handler"
	"encore.app/models"
	"encore.dev/pubsub"
	"encore.dev/rlog"
	"encore.dev/storage/sqldb"
	"github.com/bbalet/stopwords"
)

var db = sqldb.NewDatabase("discord_messages", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

var _ = pubsub.NewSubscription(
	discord_handler.DiscordRawMessageTopic,
	"message-writer",
	pubsub.SubscriptionConfig[*models.DiscordRawMessage]{
		Handler: func(ctx context.Context, message *models.DiscordRawMessage) error {
			return persistDiscordMessage(ctx, message)
		},
	})

func persistDiscordMessage(ctx context.Context, message *models.DiscordRawMessage) error {
	// open transaction
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("couldn't begin transaction: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO discord_messages 
		(id, interaction_type, channel_id, guild_id, author_id, content, clean_content) 
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, message.ID, message.InteractionType, message.ChannelID,
		message.GuildID, message.AuthorID, message.Content, message.CleanContent)
	if err != nil {
		return fmt.Errorf("couldn't insert discord message: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO discord_messages_search
		(id, content_normalized)
		VALUES ($1, $2)
	`, message.ID, normalizeText(message.CleanContent))
	if err != nil {
		return fmt.Errorf("couldn't insert discord message search: %w", err)
	}

	// commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("couldn't commit transaction: %w", err)
	}

	rlog.Info("Successfully persisted discord message", "messageID", message.ID)
	return nil
}

func normalizeText(text string) string {
	s := strings.ReplaceAll(text, "!", "")
	s = strings.ReplaceAll(s, "?", "")
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, ";", "")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	s = strings.ReplaceAll(s, "[", "")
	s = strings.ReplaceAll(s, "]", "")
	s = strings.ReplaceAll(s, "{", "")
	s = strings.ReplaceAll(s, "}", "")
	s = strings.ReplaceAll(s, "<", "")
	s = strings.ReplaceAll(s, ">", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "'", "")
	// s = strings.ReplaceAll(s, " ", "")
	s = stopwords.CleanString(s, "en", true)

	return strings.ToLower(s)
}
