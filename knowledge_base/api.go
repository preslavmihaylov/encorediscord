package knowledgebase

import (
	"context"
	"encoding/base64"
	"fmt"

	"encore.app/models"
	"encore.app/packages/apify"
	"encore.app/packages/llmservice"
	"encore.app/packages/utils"
	"encore.dev/rlog"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/structpb"
)

const knowledgeBaseUrl = "https://encore.dev/docs"
const knowledgeBaseIndexName = "knowledge-base-index"

var secrets struct {
	ApifyApiKey    string
	PineconeApiKey string
}

//encore:service
type Service struct {
	apifyService      *apify.Service
	llmService        *llmservice.Service
	pineconeClient    *pinecone.Client
	pineconeIndexConn *pinecone.IndexConnection
}

func initService() (*Service, error) {
	llmService, err := llmservice.NewService()
	if err != nil {
		return nil, fmt.Errorf("couldn't create llm service: %w", err)
	}

	pineconeClient, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey: secrets.PineconeApiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't create pinecone client: %w", err)
	}

	pineconeIndexConn, err := utils.ConnectToVectorDBIndex(
		context.Background(), pineconeClient, knowledgeBaseIndexName)
	if err != nil {
		return nil, fmt.Errorf("couldn't connect to pinecone index: %w", err)
	}

	return &Service{
		llmService:        llmService,
		pineconeClient:    pineconeClient,
		pineconeIndexConn: pineconeIndexConn,
		apifyService:      apify.NewService(secrets.ApifyApiKey),
	}, nil
}

type RelevantKnowledgeBaseArticlesResponse struct {
	Articles []*models.KnowledgeBaseArticle `json:"articles"`
}

// FindRelevantKnowledgeBaseArticles finds relevant knowledge base articles based on a query.
//
//encore:api public method=GET path=/knowledge-base/*query
func (s *Service) FindRelevantKnowledgeBaseArticles(ctx context.Context, query string) (*RelevantKnowledgeBaseArticlesResponse, error) {
	embeddings, err := s.llmService.CreateEmbeddings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("failed to create embeddings: %w", err)
	}

	resp, err := s.pineconeIndexConn.QueryByVectorValues(&ctx, &pinecone.QueryByVectorValuesRequest{
		Vector:          embeddings[0],
		TopK:            3,
		IncludeValues:   false,
		IncludeMetadata: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query pinecone index: %w", err)
	}

	highConfidenceMatches := lo.Filter(resp.Matches, func(match *pinecone.ScoredVector, _ int) bool {
		return match.Score > 0.3
	})
	if len(highConfidenceMatches) == 0 {
		rlog.Warn("No high confidence knowledge base articles found for query", "query", query)
		return &RelevantKnowledgeBaseArticlesResponse{
			Articles: []*models.KnowledgeBaseArticle{},
		}, nil
	}

	articles := lo.Map(highConfidenceMatches,
		func(match *pinecone.ScoredVector, i int) *models.KnowledgeBaseArticle {
			metadata := match.Vector.Metadata.AsMap()
			return &models.KnowledgeBaseArticle{
				ID:    match.Vector.Id,
				Text:  metadata["text"].(string),
				Title: metadata["title"].(string),
				URL:   metadata["url"].(string),
			}
		})

	return &RelevantKnowledgeBaseArticlesResponse{
		Articles: articles,
	}, nil
}

func (s *Service) StartKnowledgeBaseScraping(ctx context.Context) error {
	startWebScrapeResult, err := s.apifyService.StartWebScrapeJob(knowledgeBaseUrl)
	if err != nil {
		return fmt.Errorf("failed to start web scrape: %w", err)
	}

	_, err = db.Exec(ctx, "INSERT INTO web_scrape_jobs (id, result_id, status) VALUES ($1, $2, $3)",
		startWebScrapeResult.WebScrapeID, startWebScrapeResult.ResultID, apify.WebScrapeStatusRunning)
	if err != nil {
		return fmt.Errorf("failed to insert web scrape job: %w", err)
	}

	return nil
}

func (s *Service) CheckAndUpsertKnowledgeBaseResults(ctx context.Context) error {
	rows, err := db.Query(ctx, "SELECT id, result_id, status FROM web_scrape_jobs WHERE STATUS = 'RUNNING'")
	if err != nil {
		return fmt.Errorf("failed to query web scrape jobs: %w", err)
	}

	webScrapeJobs, err := models.MapWebScrapeJobsFromSQLRows(rows)
	if err != nil {
		return fmt.Errorf("failed to map web scrape jobs: %w", err)
	}

	rlog.Info(fmt.Sprintf("Found %d web scrape jobs", len(webScrapeJobs)))

	// any failures around here are fine as calls are idempotent & will eventually succeed on retry
	for _, webScrapeJob := range webScrapeJobs {
		webScrapeStatusResult, err := s.apifyService.CheckWebScrapeStatus(webScrapeJob.ID)
		if err != nil {
			return fmt.Errorf("failed to get web scrape result: %w", err)
		}

		if webScrapeStatusResult.Status != apify.WebScrapeStatusSucceeded {
			rlog.Info("Web scrape job is still running", "job_id", webScrapeJob.ID)
			continue
		}

		webScrapeResults, err := s.apifyService.GetWebScrapeResults(webScrapeJob.ResultID)
		if err != nil {
			return fmt.Errorf("failed to get web scrape results: %w", err)
		}

		err = s.upsertKnowledgeBaseArticlesAsVectors(ctx, webScrapeResults)
		if err != nil {
			return fmt.Errorf("failed to upsert knowledge base articles: %w", err)
		}

		_, err = db.Exec(ctx,
			"UPDATE web_scrape_jobs SET status = 'SUCCEEDED' WHERE id = $1", webScrapeJob.ID)
		if err != nil {
			return fmt.Errorf("failed to update web scrape job: %w", err)
		}
	}

	return nil
}

func (s *Service) upsertKnowledgeBaseArticlesAsVectors(
	ctx context.Context, articles []*apify.WebScrapeResult,
) error {
	articleTexts := lo.Map(articles, func(article *apify.WebScrapeResult, i int) string {
		return article.Text
	})

	embeddings, err := s.llmService.CreateEmbeddings(ctx, articleTexts)
	if err != nil {
		return fmt.Errorf("failed to create embeddings: %w", err)
	}

	vectors := lo.Map(embeddings, func(embedding []float32, i int) *pinecone.Vector {
		articleId := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("encore-%s", articles[i].URL)))
		articleUrl := structpb.NewStringValue(articles[i].URL)
		articleTitle := structpb.NewStringValue(articles[i].Title)
		articleText := structpb.NewStringValue(articles[i].Markdown)
		return &pinecone.Vector{
			Id:     articleId,
			Values: embedding,
			Metadata: &pinecone.Metadata{
				Fields: map[string]*structpb.Value{
					"url":   articleUrl,
					"title": articleTitle,
					"text":  articleText,
				},
			},
		}
	})

	_, err = s.pineconeIndexConn.UpsertVectors(&ctx, vectors)
	if err != nil {
		return fmt.Errorf("failed to upsert vectors: %w", err)
	}

	return nil
}

func connectToVectorDBIndex(ctx context.Context, pineconeClient *pinecone.Client) (*pinecone.IndexConnection, error) {
	index, err := createOrGetVectorDBIndex(ctx, pineconeClient)
	if err != nil {
		return nil, err
	}

	indexConn, err := pineconeClient.Index(index.Host)
	if err != nil {
		return nil, fmt.Errorf("couldn't connect to index: %w", err)
	}

	return indexConn, nil
}

func createOrGetVectorDBIndex(
	ctx context.Context, pineconeClient *pinecone.Client,
) (*pinecone.Index, error) {
	indices, err := pineconeClient.ListIndexes(ctx)
	if err != nil {
		panic("Error listing indexes: " + err.Error())
	}

	for _, index := range indices {
		if index.Name == knowledgeBaseIndexName {
			return index, nil
		}
	}

	index, err := pineconeClient.CreateServerlessIndex(ctx, &pinecone.CreateServerlessIndexRequest{
		Name:      knowledgeBaseIndexName,
		Dimension: 3072,
		Metric:    "cosine",
		Cloud:     "aws",
		// this is the only region supported for the default tier
		Region: "us-east-1",
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't create index: %w", err)
	}

	return index, nil
}
