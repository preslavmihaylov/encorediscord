package communityinsights

import (
	"context"
	"encore.dev/storage/sqldb"
	"fmt"
	"time"
)

var db = sqldb.NewDatabase("community_insights", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

func addInsight(ctx context.Context, id, messageType, value string, bucketTimestamp time.Time) error {
	_, err := db.Exec(ctx,
		`INSERT INTO community_insights (id, type, timestamp, value) VALUES ($1, $2, $3, $4)`,
		id, messageType, bucketTimestamp, value)
	if err != nil {
		return fmt.Errorf("error while trying to insert a community insight: %w", err)
	}

	return nil
}
