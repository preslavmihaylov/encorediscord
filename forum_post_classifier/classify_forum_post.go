package forumpostclassifier

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
	"github.com/pinecone-io/go-pinecone/pinecone"
)

var secrets struct {
	DiscordToken   string
	PineconeApiKey string
}

// #support
const forumChannelID = "1233297799366311977"
const indexName = "encorediscord-uniq-forum-posts"

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

	pineconeIndexConn, err := connectToVectorDBIndex(context.Background(), pineconeClient)
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

	// forumChannel, err := s.discordClient.Channel(forumPostChannel.ParentID)
	// if err != nil {
	// 	return fmt.Errorf("couldn't get discord channel: %w", err)
	// }

	rlog.Info("Handling discord forum post",
		"forumPostChannelId", forumPostChannel.ID,
	)

	messages, err := s.discordClient.ChannelMessages(forumPostChannel.ID, 100, "", "", "")
	if err != nil {
		return fmt.Errorf("couldn't get messages in forum post: %w", err)
	} else if len(messages) == 0 {
		rlog.Warn("No messages found in forum post")
		return errors.New("no messages found in forum post, will attempt retry...")
	}

	firstMessage := messages[len(messages)-1]
	firstMsgCleanContent := firstMessage.ContentWithMentionsReplaced()
	rlog.Info("DEBUG", "title", forumPostChannel.Name, "content", firstMsgCleanContent)

	return nil
}

func connectToVectorDBIndex(ctx context.Context, pineconeClient *pinecone.Client) (*pinecone.IndexConnection, error) {
	index, err := createOrGetVectorDBIndex(ctx, pineconeClient)
	if err != nil {
		return nil, err
	}

	indexConn, err := pineconeClient.Index(index.Host)
	if err != nil {
		return nil, fmt.Errorf("couldn't connect to index: %w", err)
	}

	return indexConn, nil
}

func createOrGetVectorDBIndex(
	ctx context.Context, pineconeClient *pinecone.Client,
) (*pinecone.Index, error) {
	indices, err := pineconeClient.ListIndexes(ctx)
	if err != nil {
		panic("Error listing indexes: " + err.Error())
	}

	for _, index := range indices {
		if index.Name == indexName {
			return index, nil
		}
	}

	index, err := pineconeClient.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
		Name:      indexName,
		Dimension: 3072,
		Metric:    "cosine",
		Cloud:     "aws",
		// this is the only region supported for the default tier
		Region: "us-east-1",
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't create index: %w", err)
	}

	return index, nil
}

func formatMessageForClassification(title, message string) string {
	return fmt.Sprintf("Title: %s\n\nContents:\n%s", title, message)
}
