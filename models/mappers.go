package models

import (
	"fmt"

	"encore.dev/storage/sqldb"
)

func MapDiscordRawMessageFromSQLRow(row *sqldb.Row) (*DiscordRawMessage, error) {
	var message DiscordRawMessage
	err := row.Scan(
		&message.ID, &message.InteractionType, &message.ChannelID,
		&message.GuildID, &message.AuthorID, &message.Content, &message.CleanContent)
	if err != nil {
		return nil, fmt.Errorf("couldn't scan message: %w", err)
	}

	return &message, nil
}

func MapDiscordRawMessagesFromSQLRows(rows *sqldb.Rows) ([]*DiscordRawMessage, error) {
	var messages []*DiscordRawMessage
	for rows.Next() {
		var message DiscordRawMessage
		err := rows.Scan(
			&message.ID, &message.InteractionType, &message.ChannelID,
			&message.GuildID, &message.AuthorID, &message.Content, &message.CleanContent)
		if err != nil {
			return nil, fmt.Errorf("couldn't scan message: %w", err)
		}

		messages = append(messages, &message)
	}

	return messages, nil
}
