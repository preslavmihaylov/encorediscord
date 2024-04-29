package conversationalerter

import (
	"context"
	"fmt"

	"encore.app/models"
)

type CreateConversationAlertRequest struct {
	Keywords  []string `json:"keywords"`
	Topics    []string `json:"topics"`
	ChannelID string   `json:"channel_id"`
}

// CreateConversationAlert creates a new conversation alert.
//
//encore:api private method=POST path=/conversation-alerts
func CreateConversationAlert(
	ctx context.Context, request *CreateConversationAlertRequest,
) (*models.ConversationAlert, error) {
	var conversationAlert models.ConversationAlert
	err := db.QueryRow(ctx, `
		INSERT INTO conversation_alerts (keywords, topics, channel_id)
		VALUES ($1, $2, $3)
		RETURNING id, keywords, topics, channel_id
	`, request.Keywords, request.Topics, request.ChannelID).
		Scan(&conversationAlert.ID, &conversationAlert.Keywords,
			&conversationAlert.Topics, &conversationAlert.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("couldn't create conversation alert: %w", err)
	}

	return &conversationAlert, nil
}
