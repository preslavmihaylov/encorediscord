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

func MapConversationAlertsFromSQLRows(rows *sqldb.Rows) ([]*ConversationAlert, error) {
	var cvs []*ConversationAlert
	for rows.Next() {
		var cv ConversationAlert
		err := rows.Scan(
			&cv.ID, &cv.Keywords, &cv.Topics, &cv.ChannelID)
		if err != nil {
			return nil, fmt.Errorf("couldn't scan message: %w", err)
		}

		cvs = append(cvs, &cv)
	}

	return cvs, nil
}

func MapWebScrapeJobsFromSQLRows(rows *sqldb.Rows) ([]*WebScrapeJob, error) {
	var wsjs []*WebScrapeJob
	for rows.Next() {
		var wsj WebScrapeJob
		err := rows.Scan(&wsj.ID, &wsj.ResultID, &wsj.Status)
		if err != nil {
			return nil, fmt.Errorf("couldn't scan message: %w", err)
		}

		wsjs = append(wsjs, &wsj)
	}

	return wsjs, nil
}
