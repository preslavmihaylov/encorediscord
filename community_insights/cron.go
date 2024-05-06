package communityinsights

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"time"

	communitymessageindexer "encore.app/community_message_indexer"
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
		return err
	}

	if err := s.addCountMessages(ctx, resp, start); err != nil {
		return err
	}

	if err := s.addMessageCountPerTopic(ctx, resp, start); err != nil {
		return err
	}

	return nil
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
			continue
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

	return addInsight(ctx, uuid.New().String(), "messages_count_per_topic", string(messageCountPerTopicJson), start)
}

func (s *Service) addCountMessages(ctx context.Context, resp *communitymessageindexer.SearchMessagesResponse, start time.Time) error {
	countAsJson := fmt.Sprintf(`{"count": %d}`, len(resp.Messages))
	return addInsight(ctx, uuid.New().String(), "message_count", countAsJson, start)
}
