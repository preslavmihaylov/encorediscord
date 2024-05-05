package apify

import (
	"github.com/samber/lo"
)

// Service is a service for interacting with the Apify API for scraping web content.
type Service struct {
	apiToken string
}

func NewService(apiToken string) *Service {
	return &Service{apiToken: apiToken}
}

type StartWebScrapeResult struct {
	WebScrapeID string
	ResultID    string
}

// make enum Status RUNNING, SUCCEEDED
type WebScrapeStatus string

const (
	WebScrapeStatusSucceeded WebScrapeStatus = "SUCCEEDED"
	WebScrapeStatusRunning   WebScrapeStatus = "RUNNING"
	WebScrapeStatusUnknown   WebScrapeStatus = "UNKNOWN"
)

type CheckWebScrapeStatusResult struct {
	Status WebScrapeStatus
}

type WebScrapeResult struct {
	URL         string
	Title       string
	Description string
	Text        string
	Markdown    string
}

func (s *Service) StartWebScrapeJob(url string) (*StartWebScrapeResult, error) {
	resp, err := s.startWebScrapeJob(url)
	if err != nil {
		return nil, err
	}

	return &StartWebScrapeResult{
		WebScrapeID: resp.Data.ID,
		ResultID:    resp.Data.DefaultDataSetID,
	}, nil
}

func (s *Service) CheckWebScrapeStatus(webScrapeID string) (*CheckWebScrapeStatusResult, error) {
	resp, err := s.checkWebScrapeStatus(webScrapeID)
	if err != nil {
		return nil, err
	}

	status := WebScrapeStatusUnknown
	switch resp.Data.Status {
	case "SUCCEEDED":
		status = WebScrapeStatusSucceeded
	case "RUNNING":
		status = WebScrapeStatusRunning
	}

	return &CheckWebScrapeStatusResult{
		Status: status,
	}, nil
}

func (s *Service) GetWebScrapeResults(webScrapeResultID string) ([]*WebScrapeResult, error) {
	resp, err := s.getWebScrapeResults(webScrapeResultID)
	if err != nil {
		return nil, err
	}

	return lo.Map(resp, func(r *WebScrapeHTTPResponse, _ int) *WebScrapeResult {
		return &WebScrapeResult{
			URL:         r.URL,
			Title:       r.Metadata.Title,
			Description: r.Metadata.Description,
			Text:        r.Text,
			Markdown:    r.Markdown,
		}
	}), nil
}
