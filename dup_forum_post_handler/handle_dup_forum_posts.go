package dupforumposthandler

import (
	"context"
	"fmt"
	"strings"

	forumpostclassifier "encore.app/forum_post_classifier"
	"encore.app/models"
	"encore.dev/pubsub"
	"encore.dev/rlog"
	"github.com/bwmarrin/discordgo"
	"github.com/samber/lo"
)

var secrets struct {
	DiscordToken   string
	PineconeApiKey string
}

// #support
const forumChannelID = "1233297799366311977"

// Service for sending automated messages for duplicate forum posts
type Service struct {
	discordClient *discordgo.Session
}

func initService() (*Service, error) {
	discordClient, err := discordgo.New("Bot " + secrets.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("couldn't create discord client: %w", err)
	}

	return &Service{
		discordClient: discordClient,
	}, nil
}

var _ = pubsub.NewSubscription(
	forumpostclassifier.DuplicateDiscordForumPostTopic,
	"dup-forum-post-handler",
	pubsub.SubscriptionConfig[*models.DuplicateDiscordForumPostEvent]{
		RetryPolicy: &pubsub.RetryPolicy{
			MaxRetries: 5,
		},
		Handler: func(ctx context.Context, forumPost *models.DuplicateDiscordForumPostEvent) error {
			rlog.Info("Received duplicate discord forum post event", "forumPost", forumPost)
			service, err := initService()
			if err != nil {
				return fmt.Errorf("couldn't create service: %w", err)
			}

			return service.HandleDuplicateDiscordForumPost(ctx, forumPost)
		},
	})

func (s *Service) HandleDuplicateDiscordForumPost(ctx context.Context, forumPostEvt *models.DuplicateDiscordForumPostEvent) error {
	forumPostChannel, err := s.discordClient.Channel(forumPostEvt.ID)
	if err != nil {
		return fmt.Errorf("couldn't get discord channel: %w", err)
	}

	forumPostsStr := strings.Join(lo.Map(forumPostEvt.DuplicateDiscordForumPostIDs, func(id string, _ int) string {
		return fmt.Sprintf(" * <#%s>", id)
	}), "\n")
	_, err = s.discordClient.ChannelMessageSend(
		forumPostChannel.ID,
		fmt.Sprintf(
			"This forum post seems to be duplicate. Please take a look at the following posts instead:\n%s",
			forumPostsStr))
	if err != nil {
		return fmt.Errorf("couldn't send message to discord channel: %w", err)
	}

	return nil
}
