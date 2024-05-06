package communityinsights

import (
	"context"
	"fmt"
	"time"

	"encore.app/packages/llmservice"
	"encore.dev/storage/sqldb"
	"github.com/bwmarrin/discordgo"
)

var db = sqldb.NewDatabase("community_insights", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

//encore:service
type Service struct {
	llmService    *llmservice.Service
	discordClient *discordgo.Session
}

var secrets struct {
	DiscordToken string
}

func initService() (*Service, error) {
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

func addInsight(ctx context.Context, id, messageType string, bucketTimestamp time.Time, value string) error {
	_, err := db.Exec(ctx,
		`INSERT INTO community_insights (id, type, timestamp, value) 
         VALUES ($1, $2, $3, $4)
         ON CONFLICT (type, timestamp) 
         DO UPDATE SET value = EXCLUDED.value`,
		id, messageType, bucketTimestamp, value)
	if err != nil {
		return fmt.Errorf("error while trying to add a community insight: %w", err)
	}

	return nil
}
