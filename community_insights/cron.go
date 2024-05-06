package communityinsights

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"

	communitymessageindexer "encore.app/community_message_indexer"
	"encore.app/models"
	"encore.dev/cron"
)

const generalChannelID = "1086301297201909864"

var _ = cron.NewJob("fetch-hourly-messages", cron.JobConfig{
	Every:    1 * cron.Hour,
	Endpoint: FetchHourlyMessages,
})

type MessageDetails struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	AuthorID string `json:"authorId"`
}

type FetchMessagesResponse struct {
	Messages []MessageDetails `json:"messages"`
}

// encore:api private method=POST path=/fetch-hourly-messages
func FetchHourlyMessages(ctx context.Context) error {
	service, err := NewService()
	if err != nil {
		return fmt.Errorf("couldn't create service: %w", err)
	}

	return service.fetchHourlyMessages(ctx)
}

func (s *Service) fetchHourlyMessages(ctx context.Context) error {
	now := time.Now().Truncate(time.Hour)
	start := now
	end := now.Add(time.Hour)
	req := &communitymessageindexer.ListMessagesRequest{
		ChannelID: generalChannelID,
		Start:     start,
		End:       end,
	}

	resp, err := communitymessageindexer.ListMessages(ctx, req)
	if err != nil {
		return fmt.Errorf("error while trying to list messages: %w", err)
	}

	if err := s.addMessageCount(ctx, resp, start); err != nil {
		return fmt.Errorf("error while trying to add message count: %w", err)
	}

	if err := s.addMessageCountPerTopic(ctx, resp, start); err != nil {
		return fmt.Errorf("error while trying to add message count per topic: %w", err)
	}

	if err := s.addMessageSentiment(ctx, resp, start); err != nil {
		return fmt.Errorf("error while trying to add message sentiment: %w", err)
	}

	return nil
}

func (s *Service) addMessageCount(ctx context.Context, resp *communitymessageindexer.SearchMessagesResponse, start time.Time) error {
	countAsJson := fmt.Sprintf(`{"count": %d}`, len(resp.Messages))
	return addInsight(ctx, uuid.New().String(), "message_count", start, countAsJson)
}

func (s *Service) addMessageCountPerTopic(ctx context.Context, resp *communitymessageindexer.SearchMessagesResponse, start time.Time) error {
	topics := []string{"Question", "Feedback", "Bug Report", "Feature Request", "Other"}
	topicMessageCount := make(map[string]int)

	for _, topic := range topics {
		topicMessageCount[topic] = 0
	}

	for _, msg := range resp.Messages {
		topic, err := s.llmService.MatchMessageToTopic(ctx, msg, topics)
		if err != nil {
			return fmt.Errorf("error while trying to match message to topic: %w", err)
		}
		topicCount, ok := topicMessageCount[topic]
		if ok {
			topicMessageCount[topic] = topicCount + 1
		}
	}

	messageCountPerTopicJson, err := json.Marshal(topicMessageCount)
	if err != nil {
		return err
	}

	return addInsight(ctx, uuid.New().String(), "messages_count_per_topic", start, string(messageCountPerTopicJson))
}

func (s *Service) addMessageSentiment(ctx context.Context, resp *communitymessageindexer.SearchMessagesResponse, start time.Time) error {
	messageAuthors := lo.Map(resp.Messages, func(msg *models.DiscordRawMessage, _ int) string {
		return msg.AuthorID
	})

	authorsToSentimentStats := make(map[string]*models.MessageSentimentStats)
	for _, messageAuthor := range messageAuthors {
		authorsToSentimentStats[messageAuthor] = &models.MessageSentimentStats{
			Positive: 0,
			Neutral:  0,
			Negative: 0,
		}
	}

	for _, msg := range resp.Messages {
		sentiment, err := s.llmService.EvaluateMessageSentiment(ctx, msg)
		if err != nil {
			return fmt.Errorf("error while trying to evaluate sentiment: %w", err)
		}

		switch sentiment {
		case models.MessageSentimentPositive:
			authorsToSentimentStats[msg.AuthorID].Positive++
		case models.MessageSentimentNeutral:
			authorsToSentimentStats[msg.AuthorID].Neutral++
		case models.MessageSentimentNegative:
			authorsToSentimentStats[msg.AuthorID].Negative++
		}
	}

	jsonVal, err := json.Marshal(authorsToSentimentStats)
	if err != nil {
		return err
	}

	return addInsight(ctx, uuid.New().String(), "sentiment_per_user", start, string(jsonVal))
}
