package forumposttagger

import (
	"context"
	"errors"
	"fmt"

	forumpostmapper "encore.app/forum_post_mapper"
	"encore.app/models"
	"encore.app/packages/llmservice"
	"encore.dev/pubsub"
	"encore.dev/rlog"
	"github.com/bwmarrin/discordgo"
	"github.com/samber/lo"
)

var secrets struct {
	DiscordToken string
}

// #support
const forumChannelID = "1233297799366311977"

// Service for tagging forum posts based on their content
type Service struct {
	llmService    *llmservice.Service
	discordClient *discordgo.Session
}

func initService() (*Service, error) {
	llmService, err := llmservice.NewService()
	if err != nil {
		return nil, fmt.Errorf("couldn't create llm service: %w", err)
	}

	discordClient, err := discordgo.New("Bot " + secrets.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("couldn't create discord client: %w", err)
	}

	return &Service{llmService: llmService, discordClient: discordClient}, nil
}

var _ = pubsub.NewSubscription(
	forumpostmapper.DiscordForumPostTopic,
	"forum-post-tagger",
	pubsub.SubscriptionConfig[*models.DiscordForumPostEvent]{
		RetryPolicy: &pubsub.RetryPolicy{
			MaxRetries: 5,
		},
		Handler: func(ctx context.Context, forumPost *models.DiscordForumPostEvent) error {
			rlog.Info("Received raw discord message", "discordMessage", forumPost)
			service, err := initService()
			if err != nil {
				return fmt.Errorf("couldn't create service: %w", err)
			}

			return service.TriageDiscordForumPost(ctx, forumPost)
		},
	})

func (s *Service) TriageDiscordForumPost(ctx context.Context, forumPostEvt *models.DiscordForumPostEvent) error {
	forumPostChannel, err := s.discordClient.Channel(forumPostEvt.ID)
	if err != nil {
		return fmt.Errorf("couldn't get discord channel: %w", err)
	}

	forumChannel, err := s.discordClient.Channel(forumPostChannel.ParentID)
	if err != nil {
		return fmt.Errorf("couldn't get discord channel: %w", err)
	}

	rlog.Info("Handling discord forum post",
		"forumPostChannelId", forumPostChannel.ID,
		"forumId", forumChannel.ID)
	if len(forumPostChannel.AppliedTags) > 0 {
		rlog.Info("Not setting tags for forum post which already has ones")
		return nil
	}

	messages, err := s.discordClient.ChannelMessages(forumPostChannel.ID, 100, "", "", "")
	if err != nil {
		return fmt.Errorf("couldn't get messages in forum post: %w", err)
	} else if len(messages) == 0 {
		rlog.Warn("No messages found in forum post")
		return errors.New("no messages found in forum post, will attempt retry...")
	}

	firstMessage := messages[len(messages)-1]
	firstMsgCleanContent := firstMessage.ContentWithMentionsReplaced()
	tagsStr := lo.Map(forumChannel.AvailableTags, func(tag discordgo.ForumTag, _ int) string {
		return tag.Name
	})

	llmDerivedTags, err := s.llmService.DetermineForumPostTags(ctx, tagsStr, forumPostChannel.Name, firstMsgCleanContent)
	if err != nil {
		return fmt.Errorf("couldn't determine forum post tags: %w", err)
	}

	// always apply the "Other" if nothing matches
	if len(llmDerivedTags) == 0 {
		llmDerivedTags = append(llmDerivedTags, "Other")
	}

	tagsToApply := lo.Filter(forumChannel.AvailableTags, func(tag discordgo.ForumTag, i int) bool {
		return lo.Contains(llmDerivedTags, tag.Name)
	})
	tagIds := lo.Map(tagsToApply, func(tag discordgo.ForumTag, i int) string {
		return tag.ID
	})

	tagIdsToApply := lo.Filter(tagIds, func(tag string, i int) bool {
		return i < 5
	})

	_, err = s.discordClient.ChannelEdit(forumPostChannel.ID, &discordgo.ChannelEdit{
		AppliedTags: lo.ToPtr(tagIdsToApply),
	})
	if err != nil {
		return fmt.Errorf("couldn't set tags for forum post: %w", err)
	}

	return nil
}
