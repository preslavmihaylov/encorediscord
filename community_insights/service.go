package communityinsights

import (
	"context"
	"encore.app/packages/llmservice"
	"encore.dev/storage/sqldb"
	"fmt"
	"time"
)

var db = sqldb.NewDatabase("community_insights", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

type Service struct {
	llmService *llmservice.Service
}

func NewService() (*Service, error) {
	llmService, err := llmservice.NewService()
	if err != nil {
		return nil, fmt.Errorf("couldn't create llm service: %w", err)
	}

	return &Service{llmService: llmService}, nil
}

func addInsight(ctx context.Context, id, messageType, value string, bucketTimestamp time.Time) error {
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
