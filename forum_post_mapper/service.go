package forumpostmapper

import (
	"context"
	"fmt"

	"encore.app/models"
	"encore.dev/rlog"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

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

func (s *Service) MapDiscordMessageToForumPost(ctx context.Context, message *models.DiscordRawMessage) error {
	forumPostChannel, err := s.discordClient.Channel(message.ChannelID)
	if err != nil {
		return fmt.Errorf("couldn't get discord channel: %w", err)
	}

	forumChannel, err := s.discordClient.Channel(forumPostChannel.ParentID)
	if err != nil {
		return fmt.Errorf("couldn't get discord channel: %w", err)
	}

	rlog.Info("channel ID",
		"channelId", message.ChannelID,
		"parentId", forumPostChannel.ParentID,
		"forumChannelType", forumChannel.Type,
		"forumPostChannel", forumPostChannel.Name)
	if forumChannel.Type != discordgo.ChannelTypeGuildForum {
		rlog.Warn("Ignoring message for non-forum channel", "channelName", forumPostChannel.Name)
		return nil
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("couldn't begin transaction: %w", err)
	}

	result, err := tx.Exec(ctx, `
		INSERT INTO discord_forum_posts (id, discord_id)
		VALUES ($1, $2)
		ON CONFLICT (discord_id) DO NOTHING
		RETURNING id, discord_id
	`, uuid.NewString(), forumPostChannel.ID)
	if err != nil {
		return fmt.Errorf("couldn't insert forum post: %w", err)
	} else if result.RowsAffected() == 0 {
		rlog.Info("Forum post already exists in database", "discordId", forumPostChannel.ID)
		return nil
	}

	// Note: There is no atomicity between db insert and pubsub publish.
	// To make it properly, we'll need to utilize the transactional outbox pattern,
	// which is out of scope for this hackathon.
	_, err = DiscordForumPostTopic.Publish(ctx, &models.DiscordForumPostEvent{
		ID:      forumPostChannel.ID,
		GuildID: forumPostChannel.GuildID,
	})
	if err != nil {
		return fmt.Errorf("couldn't publish forum post: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("couldn't commit transaction: %w", err)
	}

	rlog.Info("Successfully inserted & published forum post", "discordId", forumPostChannel.ID)
	return nil
}
