package communityinsights

import (
	"context"
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

//encore:api private
func FetchHourlyMessages(ctx context.Context) (*FetchMessagesResponse, error) {
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
		return nil, err
	}

	countAsJson := fmt.Sprintf(`{"count": %d}`, len(resp.Messages))
	err = addInsight(ctx, uuid.New().String(), "message_count", countAsJson, start)
	if err != nil {
		return nil, err
	}

	var messages []MessageDetails
	for _, msg := range resp.Messages {
		messages = append(messages, MessageDetails{
			ID:       msg.ID,
			Content:  msg.Content,
			AuthorID: msg.AuthorID,
		})
	}

	return &FetchMessagesResponse{Messages: messages}, nil
}
