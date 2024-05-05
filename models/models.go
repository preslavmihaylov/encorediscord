package models

import (
	"encore.app/packages/apify"
	"github.com/bwmarrin/discordgo"
)

type DiscordRawMessage struct {
	ID              string                    `json:"id"`
	InteractionType discordgo.InteractionType `json:"type"`
	ChannelID       string                    `json:"channelId"`
	GuildID         string                    `json:"guildId"`
	AuthorID        string                    `json:"authorId"`
	Content         string                    `json:"content"`
	CleanContent    string                    `json:"cleanContent"`
}

type DiscordForumPostEvent struct {
	ID      string `json:"id"`
	GuildID string `json:"guildId"`
}

type DuplicateDiscordForumPostEvent struct {
	ID                           string   `json:"id"`
	DuplicateDiscordForumPostIDs []string `json:"duplicateDiscordForumPostIds"`
	GuildID                      string   `json:"guildId"`
}

type DiscordCommunityMessageEvent struct {
	ID              string                    `json:"id"`
	InteractionType discordgo.InteractionType `json:"type"`
	ChannelID       string                    `json:"channelId"`
	GuildID         string                    `json:"guildId"`
	AuthorID        string                    `json:"authorId"`
	Content         string                    `json:"content"`
	CleanContent    string                    `json:"cleanContent"`
}

type ConversationAlert struct {
	ID        string   `json:"id"`
	Keywords  []string `json:"keywords"`
	Topics    []string `json:"topics"`
	ChannelID string   `json:"channel"`
}

type KnowledgeBaseArticle struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

type WebScrapeJob struct {
	ID       string                `json:"id"`
	ResultID string                `json:"resultId"`
	Status   apify.WebScrapeStatus `json:"status"`
}
