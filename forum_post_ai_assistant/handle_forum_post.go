package forumpostaiassistant

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	forumpostclassifier "encore.app/forum_post_classifier"
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

// #support
const forumChannelID = "1233297799366311977"
const indexName = "knowledge-base-index"

// Service for sending automated messages for forum posts
// based on a knowledge base we've built
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

	pineconeIndexConn, err := utils.ConnectToVectorDBIndex(context.Background(), pineconeClient, indexName)
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
	forumpostclassifier.UniqueDiscordForumPostTopic,
	"forum-post-ai-assistant",
	pubsub.SubscriptionConfig[*models.DiscordForumPostEvent]{
		RetryPolicy: &pubsub.RetryPolicy{
			MaxRetries: 5,
		},
		AckDeadline: time.Minute * 5,
		Handler: func(ctx context.Context, forumPost *models.DiscordForumPostEvent) error {
			rlog.Info("Received discord forum post event", "forumPost", forumPost)
			service, err := initService()
			if err != nil {
				return fmt.Errorf("couldn't create service: %w", err)
			}

			return service.HandleDiscordForumPost(ctx, forumPost)
		},
	})

func (s *Service) HandleDiscordForumPost(ctx context.Context, forumPostEvt *models.DiscordForumPostEvent) error {
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
	userForumPostContents := formatMessageForAIAssistant(forumPostChannel.Name, firstMsgCleanContent)
	matches, err := s.SearchForSimilarMessages(
		ctx, userForumPostContents)
	if err != nil {
		return fmt.Errorf("couldn't search for similar messages: %w", err)
	}

	highConfidenceMatches := lo.Filter(matches, func(match *pinecone.ScoredVector, _ int) bool {
		return match.Score > 0.3
	})
	if len(highConfidenceMatches) == 0 {
		rlog.Warn("No high confidence knowledge base articles found for forum post")
		return nil
	}

	topMatches := lo.Filter(matches, func(match *pinecone.ScoredVector, i int) bool {
		return i < 3
	})

	matchedArticles := lo.Map(topMatches,
		func(match *pinecone.ScoredVector, _ int) *models.KnowledgeBaseArticle {
			metadata := match.Vector.Metadata.AsMap()
			return &models.KnowledgeBaseArticle{
				ID:   match.Vector.Id,
				Text: metadata["text"].(string),
				URL:  metadata["url"].(string),
			}
		})

	aiAssistantAnswer, err := s.llmService.AnswerForumPost(ctx, userForumPostContents, matchedArticles)
	if err != nil {
		return fmt.Errorf("couldn't answer forum post: %w", err)
	} else if aiAssistantAnswer == "" {
		rlog.Warn("AI didn't provide an answer for the forum post")
		return nil
	}

	_, err = s.discordClient.ChannelMessageSend(
		forumPostChannel.ID, attachSourcesToAIAssistantAnswer(aiAssistantAnswer, matchedArticles))
	if err != nil {
		return fmt.Errorf("couldn't send message to discord channel: %w", err)
	}

	rlog.Info("Sent AI assistant answer to discord channel")
	return nil
}

func (s *Service) SearchForSimilarMessages(ctx context.Context, msg string) ([]*pinecone.ScoredVector, error) {
	embeddings, err := s.llmService.CreateEmbeddings(ctx, []string{msg})
	if err != nil {
		return nil, fmt.Errorf("couldn't create embeddings: %w", err)
	}

	msgEmbedding := embeddings[0]
	resp, err := s.pineconeIndexConn.QueryByVectorValues(&ctx, &pinecone.QueryByVectorValuesRequest{
		Vector:          msgEmbedding,
		TopK:            3,
		IncludeValues:   false,
		IncludeMetadata: true,
	})
	if err != nil {
		panic("Error querying vectors: " + err.Error())
	}

	return resp.Matches, nil
}

func attachSourcesToAIAssistantAnswer(aiAssistantAnswer string, sources []*models.KnowledgeBaseArticle) string {
	return fmt.Sprintf("%s\n\n---\nSources:\n%s", aiAssistantAnswer,
		strings.Join(lo.Map(sources, func(source *models.KnowledgeBaseArticle, _ int) string {
			return fmt.Sprintf("* %s", source.URL)
		}), "\n"))
}

func formatMessageForAIAssistant(title, message string) string {
	return fmt.Sprintf("Title: %s\n\nContents:\n%s", title, message)
}
