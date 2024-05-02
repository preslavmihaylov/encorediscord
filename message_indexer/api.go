package messageindexer

import (
	"context"
	"fmt"
	"time"

	"encore.app/models"
)

type SearchMessagesRequest struct {
	Start      time.Time `query:"start"`
	End        time.Time `query:"end"`
	SearchTerm string    `query:"search_term"`
}

type SearchMessagesResponse struct {
	Messages []*models.DiscordRawMessage `json:"messages"`
}

// SearchMessages searches for messages in the database.
//
//encore:api private method=GET path=/search-messages
func SearchMessages(ctx context.Context, req *SearchMessagesRequest) (*SearchMessagesResponse, error) {
	rows, err := db.Query(ctx, `
		SELECT 
			dm.id, dm.interaction_type, dm.channel_id, dm.guild_id, 
			dm.author_id, dm.content, dm.clean_content
		FROM discord_messages_search dms 
		JOIN discord_messages dm ON dms.id = dm.id
		WHERE dm.created_at BETWEEN $1 AND $2 
		  AND $3 % ANY(STRING_TO_ARRAY(dms.content_normalized, ' '))
	`, req.Start, req.End, req.SearchTerm)
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

type ListMessagesRequest struct {
	ChannelID string    `query:"channel_id"`
	Start     time.Time `query:"start"`
	End       time.Time `query:"end"`
}

// ListMessages searches for messages in the specified time range.
//
//encore:api private method=GET path=/messages
func ListMessages(
	ctx context.Context, request *ListMessagesRequest,
) (*SearchMessagesResponse, error) {
	rows, err := db.Query(ctx, `
		SELECT 
			id, interaction_type, channel_id, guild_id, 
			author_id, content, clean_content
		FROM discord_messages
		WHERE created_at BETWEEN $1 AND $2 AND channel_id = $3
	`, request.Start, request.End, request.ChannelID)
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
