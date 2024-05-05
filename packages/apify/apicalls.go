package apify

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type StartWebScrapeHTTPResponse struct {
	Data struct {
		ID               string `json:"id"`
		DefaultDataSetID string `json:"defaultDatasetId"`
	} `json:"data"`
}

type CheckWebScrapeStatusHTTPResponse struct {
	Data struct {
		Status string `json:"status"`
	} `json:"data"`
}

type WebScrapeHTTPResponse struct {
	URL      string `json:"url"`
	Metadata struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	} `json:"metadata"`
	Text     string `json:"text"`
	Markdown string `json:"markdown"`
}

//go:embed start_webscrape_request.json
var startWebScrapeRequestBody string

func (s *Service) startWebScrapeJob(url string) (*StartWebScrapeHTTPResponse, error) {
	input := fmt.Sprintf(startWebScrapeRequestBody, url)
	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("https://api.apify.com/v2/acts/aYG0l9s7dbB7j3gbS/runs?token=%s", s.apiToken),
		bytes.NewBuffer([]byte(input)))
	if err != nil {
		return nil, fmt.Errorf("Error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response: %w", err)
	}

	var respBody StartWebScrapeHTTPResponse
	if err := json.Unmarshal(body, &respBody); err != nil {
		return nil, fmt.Errorf("Error unmarshalling response: %w", err)
	}

	return &respBody, nil
}

func (s *Service) checkWebScrapeStatus(webScrapeJobID string) (*CheckWebScrapeStatusHTTPResponse, error) {
	resp, err := http.DefaultClient.Get(
		fmt.Sprintf("https://api.apify.com/v2/actor-runs/%s?token=%s", webScrapeJobID, s.apiToken))
	if err != nil {
		return nil, fmt.Errorf("Error sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response: %w", err)
	}

	var respBody CheckWebScrapeStatusHTTPResponse
	if err := json.Unmarshal(body, &respBody); err != nil {
		return nil, fmt.Errorf("Error unmarshalling response: %w", err)
	}

	return &respBody, nil
}

func (s *Service) getWebScrapeResults(webScrapeDatasetID string) ([]*WebScrapeHTTPResponse, error) {
	resp, err := http.DefaultClient.Get(
		fmt.Sprintf("https://api.apify.com/v2/datasets/%s/items?token=%s", webScrapeDatasetID, s.apiToken))
	if err != nil {
		return nil, fmt.Errorf("Error sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response: %w", err)
	}

	var respBody []*WebScrapeHTTPResponse
	if err := json.Unmarshal(body, &respBody); err != nil {
		return nil, fmt.Errorf("Error unmarshalling response: %w", err)
	}

	return respBody, nil
}
