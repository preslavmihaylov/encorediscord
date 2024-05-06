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

//encore:api private
func FetchHourlyMessages(ctx context.Context) error {
	now := time.Now().Truncate(time.Hour)
	start := now.Add(-1 * time.Hour)
	req := &communitymessageindexer.ListMessagesRequest{
		ChannelID: generalChannelID,
		Start:     start,
		End:       now,
	}

	resp, err := communitymessageindexer.ListMessages(ctx, req)
	if err != nil {
		return err
	}

	countAsJson := fmt.Sprintf(`{"count": %d}`, len(resp.Messages))
	err = addInsight(ctx, uuid.New().String(), "message_count", countAsJson, start)
	if err != nil {
		return err
	}

	return nil
}
