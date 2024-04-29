package messageindexer

import (
	"context"
	"fmt"
	"time"

	"encore.app/models"
)

type SearchMessagesResponse struct {
	Messages []*models.DiscordRawMessage `json:"messages"`
}

// SearchMessages searches for messages in the database.
//
//encore:api private method=GET path=/search-messages/*searchTerm
func SearchMessages(ctx context.Context, searchTerm string) (*SearchMessagesResponse, error) {
	rows, err := db.Query(ctx, `
		SELECT 
			dm.id, dm.interaction_type, dm.channel_id, dm.guild_id, 
			dm.author_id, dm.content, dm.clean_content
		FROM discord_messages_search dms 
		JOIN discord_messages dm ON dms.id = dm.id
		WHERE $1 % ANY(STRING_TO_ARRAY(dms.content_normalized, ' '))
	`, searchTerm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages, err := models.MapDiscordRawMessagesFromSQLRows(rows)
	if err != nil {
		return nil, fmt.Errorf("couldn't map messages: %w", err)
	}

	return &SearchMessagesResponse{Messages: messages}, nil
}

type GetMessagesInTimeRangeRequest struct {
	Start time.Time `query:"start"`
	End   time.Time `query:"end"`
}

// ListMessages searches for messages in the specified time range.
//
//encore:api private method=GET path=/messages
func ListMessages(
	ctx context.Context, request *GetMessagesInTimeRangeRequest,
) (*SearchMessagesResponse, error) {
	rows, err := db.Query(ctx, `
		SELECT 
			id, interaction_type, channel_id, guild_id, 
			author_id, content, clean_content
		FROM discord_messages
		WHERE created_at BETWEEN $1 AND $2
	`, request.Start, request.End)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages, err := models.MapDiscordRawMessagesFromSQLRows(rows)
	if err != nil {
		return nil, fmt.Errorf("couldn't map messages: %w", err)
	}

	return &SearchMessagesResponse{Messages: messages}, nil
}
