package communitymessagemapper

import (
	"context"
	"fmt"

	"encore.app/models"
	"encore.dev/rlog"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

// #general
const generalChannelID = "1086301297201909864"

var secrets struct {
	DiscordToken string
}

// Service for tagging forum posts based on their content
type Service struct {
	discordClient *discordgo.Session
}

func initService() (*Service, error) {
	discordClient, err := discordgo.New("Bot " + secrets.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("couldn't create discord client: %w", err)
	}

	return &Service{discordClient: discordClient}, nil
}

func (s *Service) MapDiscordMessageToCommunityMessage(ctx context.Context, message *models.DiscordRawMessage) error {
	rlog.Info("Handling discord raw message",
		"channelId", message.ChannelID)
	if message.ChannelID != generalChannelID {
		rlog.Warn("Ignoring message for non-general channel")
		return nil
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("couldn't begin transaction: %w", err)
	}

	result, err := tx.Exec(ctx, `
		INSERT INTO discord_community_messages (id, discord_message_id)
		VALUES ($1, $2)
		ON CONFLICT (discord_message_id) DO NOTHING
		RETURNING id, discord_message_id
	`, uuid.NewString(), message.ID)
	if err != nil {
		return fmt.Errorf("couldn't insert forum post: %w", err)
	} else if result.RowsAffected() == 0 {
		rlog.Info("Discord message already exists in db", "messageId", message.ID)
		return nil
	}

	// Note: There is no atomicity between db insert and pubsub publish.
	// To make it properly, we'll need to utilize the transactional outbox pattern,
	// which is out of scope for this hackathon.
	_, err = DiscordCommunityMessageTopic.Publish(ctx, &models.DiscordCommunityMessageEvent{
		ID:              message.ID,
		InteractionType: message.InteractionType,
		ChannelID:       message.ChannelID,
		GuildID:         message.GuildID,
		AuthorID:        message.AuthorID,
		Content:         message.Content,
		CleanContent:    message.CleanContent,
	})
	if err != nil {
		return fmt.Errorf("couldn't publish community message: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("couldn't commit transaction: %w", err)
	}

	rlog.Info("Successfully inserted & published community message")
	return nil
}
