package conversationalerter

import (
	"fmt"

	"encore.app/packages/llmservice"
	"encore.dev/storage/sqldb"
	"github.com/bwmarrin/discordgo"
)

var db = sqldb.NewDatabase("conversation_alerter", sqldb.DatabaseConfig{
	Migrations: "./migrations",
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
