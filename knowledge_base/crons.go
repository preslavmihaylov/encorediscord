package knowledgebase

import (
	"context"
	"fmt"

	"encore.dev/cron"
)

var _ = cron.NewJob("start-knowledge-base-scraping", cron.JobConfig{
	Title:    "Start Knowledge Base Scraping",
	Endpoint: StartKnowledgeBaseScrapingCron,
	Every:    24 * cron.Hour,
})

// StartKnowledgeBaseScraping starts the web scraping process for knowledge base articles.
//
//encore:api private method=POST path=/start-scraping
func StartKnowledgeBaseScrapingCron(ctx context.Context) error {
	svc, err := initService()
	if err != nil {
		return fmt.Errorf("couldn't create knowledge base service: %w", err)
	}

	return svc.StartKnowledgeBaseScraping(ctx)
}

var _ = cron.NewJob("check-knowledge-base-scraping", cron.JobConfig{
	Title:    "Start Knowledge Base Scraping",
	Endpoint: CheckAndUpsertKnowledgeBaseResultsCron,
	Every:    1 * cron.Minute,
})

// CheckAndUpsertKnowledgeBaseResults checks the status of the web scraping process and
// upserts the results into the knowledge base database.
//
//encore:api private method=POST path=/process-results
func CheckAndUpsertKnowledgeBaseResultsCron(ctx context.Context) error {
	svc, err := initService()
	if err != nil {
		return fmt.Errorf("couldn't create knowledge base service: %w", err)
	}

	return svc.CheckAndUpsertKnowledgeBaseResults(ctx)
}
