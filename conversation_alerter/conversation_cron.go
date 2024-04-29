package conversationalerter

import (
	"context"
	"fmt"
	"strings"
	"time"

	messageindexer "encore.app/message_indexer"
	"encore.app/models"
	"encore.app/packages/llmservice"
	"encore.dev/cron"
	"encore.dev/rlog"
	"github.com/bwmarrin/discordgo"
	"github.com/samber/lo"
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

type Service struct {
	llmService    *llmservice.Service
	discordClient *discordgo.Session
}

func NewService() (*Service, error) {
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
	rows, err := db.Query(ctx, "SELECT id, topics, keywords, channel_id FROM conversation_alerts")
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

		discordMsgs := []*discordgo.Message{}
		for _, message := range matchingMessages {
			discordMsg, err := s.discordClient.ChannelMessage(message.ChannelID, message.ID)
			if err != nil {
				return fmt.Errorf("couldn't get discord message: %w", err)
			}

			discordMsgs = append(discordMsgs, discordMsg)
		}

		messagesStr := lo.Map(discordMsgs, func(msg *discordgo.Message, _ int) string {
			return fmt.Sprintf(" - [Link](https://discord.com/channels/%s/%s/%s)", msg.GuildID, msg.ChannelID, msg.ID)
		})

		alertMsg := fmt.Sprintf("🔔 New messages matching topic(s) [%s] found:\n%s",
			strings.Join(alert.Topics, ", "), strings.Join(messagesStr, "\n"))
		_, err = s.discordClient.ChannelMessageSend(conversationAlertsChannelID, alertMsg)
		if err != nil {
			return fmt.Errorf("couldn't send discord message: %w", err)
		}
	}

	// TODO: alert for keywords

	return nil
}

func (s *Service) findMessagesMatchingTopic(
	ctx context.Context, alert *models.ConversationAlert, now time.Time,
) ([]*models.DiscordRawMessage, error) {
	generalChannelID := "1086301297201909864"
	resp, err := messageindexer.ListMessages(ctx, &messageindexer.ListMessagesRequest{
		ChannelID: generalChannelID,
		Start:     now.Add(-cronTimeDuration).UTC(),
		End:       now.UTC(),
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't get messages: %w", err)
	}

	rlog.Info("Listed messages", "messages", resp.Messages)
	matchingMessages, err := s.llmService.FindMessagesMatchingTopic(ctx, resp.Messages, alert.Topics)
	if err != nil {
		return nil, fmt.Errorf("couldn't find messages matching topic: %w", err)
	}

	return matchingMessages, nil
}
