package conversationalerter

import (
	"context"
	"fmt"
	"strings"
	"time"

	communitymessageindexer "encore.app/community_message_indexer"
	"encore.app/models"
	"encore.dev/cron"
	"encore.dev/rlog"
)

const cronTimeDuration = 10 * time.Minute
const conversationAlertsChannelID = "1234396668837892107"

var secrets struct {
	DiscordToken string
}

// Check latest messages & alert if any topics/keywords match criteria.
var _ = cron.NewJob("check-conversation-alerts", cron.JobConfig{
	Title:    "Check all conversation alerts",
	Endpoint: CheckConversationAlerts,
	Every:    10 * cron.Minute,
})

// CheckConversationAlerts checks for any messages in the last time window
// matching the defined conversation alerts.
//
//encore:api private method=POST path=/check-conversation-alerts
func CheckConversationAlerts(ctx context.Context) error {
	service, err := NewService()
	if err != nil {
		return fmt.Errorf("couldn't create service: %w", err)
	}

	return service.checkConversationAlerts(ctx)
}

func (s *Service) checkConversationAlerts(ctx context.Context) error {
	now := time.Now()
	rows, err := db.Query(ctx, "SELECT id, keywords, topics, channel_id FROM conversation_alerts")
	if err != nil {
		return fmt.Errorf("couldn't get conversation alerts: %w", err)
	}

	conversationAlerts, err := models.MapConversationAlertsFromSQLRows(rows)
	if err != nil {
		return fmt.Errorf("couldn't map conversation alerts: %w", err)
	}

	for _, alert := range conversationAlerts {
		matchingMessages, err := s.findMessagesMatchingTopic(ctx, alert, now)
		if err != nil {
			return fmt.Errorf("couldn't find messages matching topic: %w", err)
		}

		if len(matchingMessages) == 0 {
			rlog.Info("No messages found matching topics", "topics", alert.Topics)
			continue
		}

		discordMsgsStr := []string{}
		for _, message := range matchingMessages {
			discordMsg, err := s.discordClient.ChannelMessage(message.ChannelID, message.ID)
			if err != nil {
				return fmt.Errorf("couldn't get discord message: %w", err)
			}

			discordChannel, err := s.discordClient.Channel(message.ChannelID)
			if err != nil {
				return fmt.Errorf("couldn't get discord channel: %w", err)
			}

			discordMsgsStr = append(discordMsgsStr,
				fmt.Sprintf("[Link](https://discord.com/channels/%s/%s/%s)",
					discordChannel.GuildID, discordMsg.ChannelID, discordMsg.ID))
		}

		alertMsg := fmt.Sprintf("🔔 New messages matching topic(s) [%s] found:\n%s",
			strings.Join(alert.Topics, ", "), strings.Join(discordMsgsStr, "\n"))
		_, err = s.discordClient.ChannelMessageSend(conversationAlertsChannelID, alertMsg)
		if err != nil {
			return fmt.Errorf("couldn't send discord message: %w", err)
		}
	}

	for _, alert := range conversationAlerts {
		for _, keyword := range alert.Keywords {
			resp, err := communitymessageindexer.SearchMessages(ctx, &communitymessageindexer.SearchMessagesRequest{
				Start:      now.Add(-cronTimeDuration).UTC(),
				End:        now.UTC(),
				SearchTerm: keyword,
			})
			if err != nil {
				return fmt.Errorf("couldn't find messages matching topic: %w", err)
			}

			discordMsgsStr := []string{}
			matchingMessages := resp.Messages
			if len(matchingMessages) == 0 {
				rlog.Info("No messages found matching keyword", "keyword", keyword)
				continue
			}

			for _, message := range matchingMessages {
				discordMsg, err := s.discordClient.ChannelMessage(message.ChannelID, message.ID)
				if err != nil {
					return fmt.Errorf("couldn't get discord message: %w", err)
				}

				discordChannel, err := s.discordClient.Channel(message.ChannelID)
				if err != nil {
					return fmt.Errorf("couldn't get discord channel: %w", err)
				}

				discordMsgsStr = append(discordMsgsStr,
					fmt.Sprintf("[Link](https://discord.com/channels/%s/%s/%s)",
						discordChannel.GuildID, discordMsg.ChannelID, discordMsg.ID))
			}

			alertMsg := fmt.Sprintf("🔔 New messages matching keyword \"%s\" found:\n%s",
				keyword, strings.Join(discordMsgsStr, "\n"))
			_, err = s.discordClient.ChannelMessageSend(conversationAlertsChannelID, alertMsg)
			if err != nil {
				return fmt.Errorf("couldn't send discord message: %w", err)
			}
		}
	}

	return nil
}

func (s *Service) findMessagesMatchingTopic(
	ctx context.Context, alert *models.ConversationAlert, now time.Time,
) ([]*models.DiscordRawMessage, error) {
	generalChannelID := "1086301297201909864"
	resp, err := communitymessageindexer.ListMessages(ctx, &communitymessageindexer.ListMessagesRequest{
		ChannelID: generalChannelID,
		Start:     now.Add(-cronTimeDuration).UTC(),
		End:       now.UTC(),
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't get messages: %w", err)
	}

	rlog.Info("Listed messages", "messages", resp.Messages, "alert", alert)
	matchingMessages, err := s.llmService.FindMessagesMatchingTopic(ctx, resp.Messages, alert.Topics)
	if err != nil {
		return nil, fmt.Errorf("couldn't find messages matching topic: %w", err)
	}

	return matchingMessages, nil
}
