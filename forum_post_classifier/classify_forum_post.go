package forumpostclassifier

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"

	forumpostmapper "encore.app/forum_post_mapper"
	"encore.app/models"
	"encore.app/packages/llmservice"
	"encore.app/packages/utils"
	"encore.dev/pubsub"
	"encore.dev/rlog"
	"github.com/bwmarrin/discordgo"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"github.com/samber/lo"
)

var secrets struct {
	DiscordToken   string
	PineconeApiKey string
}

const uniqForumPostsIndexName = "encorediscord-uniq-forum-posts"

// Service for classifying forum posts as duplicate or unique
type Service struct {
	llmService        *llmservice.Service
	discordClient     *discordgo.Session
	pineconeClient    *pinecone.Client
	pineconeIndexConn *pinecone.IndexConnection
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

	pineconeClient, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey: secrets.PineconeApiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't create pinecone client: %w", err)
	}

	pineconeIndexConn, err := utils.ConnectToVectorDBIndex(context.Background(), pineconeClient, uniqForumPostsIndexName)
	if err != nil {
		return nil, fmt.Errorf("couldn't connect to pinecone index: %w", err)
	}

	return &Service{
		llmService:        llmService,
		discordClient:     discordClient,
		pineconeClient:    pineconeClient,
		pineconeIndexConn: pineconeIndexConn,
	}, nil
}

var _ = pubsub.NewSubscription(
	forumpostmapper.DiscordForumPostTopic,
	"forum-post-classifier",
	pubsub.SubscriptionConfig[*models.DiscordForumPostEvent]{
		RetryPolicy: &pubsub.RetryPolicy{
			MaxRetries: 5,
		},
		Handler: func(ctx context.Context, forumPost *models.DiscordForumPostEvent) error {
			rlog.Info("Received discord forum post event", "forumPost", forumPost)
			service, err := initService()
			if err != nil {
				return fmt.Errorf("couldn't create service: %w", err)
			}

			return service.ClassifyDiscordForumPost(ctx, forumPost)
		},
	})

func (s *Service) ClassifyDiscordForumPost(ctx context.Context, forumPostEvt *models.DiscordForumPostEvent) error {
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
	)

	otherTag := lo.Filter(forumChannel.AvailableTags, func(tag discordgo.ForumTag, _ int) bool {
		return tag.Name == "Other"
	})[0]
	if len(forumPostChannel.AppliedTags) == 0 {
		return errors.New("forum post has no tags yet, will attempt retry...")
	} else if lo.Contains(forumPostChannel.AppliedTags, otherTag.ID) {
		rlog.Warn("Skipping classification for forum post with 'Other' tag")
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
	embeddings, err := s.llmService.CreateEmbeddings(
		ctx, []string{formatMessageForClassification(forumPostChannel.Name, firstMsgCleanContent)})
	if err != nil {
		return fmt.Errorf("couldn't create embeddings: %w", err)
	}

	msgEmbedding := embeddings[0]
	matches, err := s.SearchForSimilarMessages(ctx, msgEmbedding)
	if err != nil {
		return fmt.Errorf("couldn't search for similar messages: %w", err)
	}

	highConfidenceMatches := lo.Filter(matches, func(match *pinecone.ScoredVector, _ int) bool {
		return match.Score > 0.85
	})

	if len(highConfidenceMatches) == 0 {
		rlog.Info("Unique forum post detected, adding to forum posts index & publishing event")
		if err := s.upsertMessageAsVector(ctx, forumPostChannel.ID, embeddings[0], map[string]*structpb.Value{
			"forum_channel_id": structpb.NewStringValue(forumPostChannel.ID),
		}); err != nil {
			return fmt.Errorf("couldn't upsert message as vector: %w", err)
		}

		_, err = UniqueDiscordForumPostTopic.Publish(ctx, &models.DiscordForumPostEvent{
			ID:      forumPostChannel.ID,
			GuildID: forumPostChannel.GuildID,
		})
		if err != nil {
			return fmt.Errorf("couldn't publish unique forum post: %w", err)
		}
	} else {
		rlog.Info("Duplicate forum post detected, publishing event", "highConfidenceMatches", highConfidenceMatches)
		_, err = DuplicateDiscordForumPostTopic.Publish(ctx, &models.DuplicateDiscordForumPostEvent{
			ID: forumPostChannel.ID,
			DuplicateDiscordForumPostIDs: lo.Map(highConfidenceMatches, func(match *pinecone.ScoredVector, _ int) string {
				return match.Vector.Id
			}),
			GuildID: forumPostChannel.GuildID,
		})
		if err != nil {
			return fmt.Errorf("couldn't publish duplicate forum post: %w", err)
		}
	}

	return nil
}

func (s *Service) SearchForSimilarMessages(ctx context.Context, embedding []float32) ([]*pinecone.ScoredVector, error) {
	resp, err := s.pineconeIndexConn.QueryByVectorValues(&ctx, &pinecone.QueryByVectorValuesRequest{
		Vector:          embedding,
		TopK:            5,
		IncludeValues:   false,
		IncludeMetadata: false,
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't query vectors: %w", err)
	}

	return resp.Matches, nil
}

func (s *Service) upsertMessageAsVector(
	ctx context.Context, id string, embedding []float32, metadata map[string]*structpb.Value,
) error {
	vector := &pinecone.Vector{
		Id:     id,
		Values: embedding,
		Metadata: &pinecone.Metadata{
			Fields: metadata,
		},
	}
	_, err := s.pineconeIndexConn.UpsertVectors(&ctx, []*pinecone.Vector{vector})
	if err != nil {
		return fmt.Errorf("couldn't upsert vector: %w", err)
	}

	return nil
}

func formatMessageForClassification(title, message string) string {
	return fmt.Sprintf("Title: %s\n\nContents:\n%s", title, message)
}
