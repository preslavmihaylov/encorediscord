package communityinsights

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"encore.app/models"
	"encore.dev/rlog"
	"github.com/samber/lo"
)

type MetricDurationRequest struct {
	Hours uint `json:"hours"`
}

type TimeCountPair struct {
	Timestamp time.Time `json:"timestamp"`
	Count     int       `json:"count"`
}

type MessageCountResponse struct {
	TimeCounts []TimeCountPair `json:"timeCounts"`
}

type TimeCountPerTopic struct {
	Timestamp   time.Time      `json:"timestamp"`
	TopicCounts map[string]int `json:"topicCounts"`
}

type MessageCountPerTopicResponse struct {
	TimeMessageCountPerTopic []TimeCountPerTopic `json:"timeMessageCountPerTopic"`
}

type UserSentimentResponse struct {
	PositiveSentiments map[string]float32 `json:"positiveSentiments"`
	NegativeSentiments map[string]float32 `json:"negativeSentiments"`
}

// encore:api public path=/get-message-counts
func (s *Service) GetMessageCounts(ctx context.Context, req *MetricDurationRequest) (*MessageCountResponse, error) {
	if req.Hours > 24 {
		return nil, errors.New("please provide a duration less than or equal to 24 hours")
	}

	endTime := time.Now().UTC().Truncate(time.Hour)
	startTime := endTime.Add(time.Duration(-int(req.Hours)) * time.Hour)

	hourlyCounts := make(map[time.Time]int)
	for t := startTime; !t.After(endTime); t = t.Add(time.Hour) {
		hourlyCounts[t] = 0
	}

	query := `
		SELECT date_trunc('hour', timestamp) AS hour, COALESCE(SUM((value->>'count')::INTEGER), 0) AS count
		FROM community_insights
		WHERE type = 'message_count' AND timestamp BETWEEN $1 AND $2
		GROUP BY hour
		ORDER BY hour
	`

	rows, err := db.Query(ctx, query, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var hour time.Time
		var count int
		if err := rows.Scan(&hour, &count); err != nil {
			return nil, err
		}
		_, ok := hourlyCounts[hour]
		if ok {
			hourlyCounts[hour] = count
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	var timeCounts []TimeCountPair
	for timestamp, count := range hourlyCounts {
		timeCounts = append(timeCounts, TimeCountPair{Timestamp: timestamp, Count: count})
	}

	sort.Slice(timeCounts, func(i, j int) bool {
		return timeCounts[i].Timestamp.Before(timeCounts[j].Timestamp)
	})

	return &MessageCountResponse{TimeCounts: timeCounts}, nil
}

// encore:api public path=/get-message-counts-per-topic
func (s *Service) GetMessageCountsPerTopic(ctx context.Context, req *MetricDurationRequest) (*MessageCountPerTopicResponse, error) {
	if req.Hours > 24 {
		return nil, errors.New("please provide a duration less than or equal to 24 hours")
	}

	endTime := time.Now().UTC().Truncate(time.Hour)
	startTime := endTime.Add(time.Duration(-int(req.Hours)) * time.Hour)

	results := initializeTimeCountPerTopic(startTime, endTime)
	query := `
		SELECT date_trunc('hour', timestamp) AS hour, value
		FROM community_insights
		WHERE type = 'messages_count_per_topic' AND timestamp BETWEEN $1 AND $2
		ORDER BY hour
	`

	rows, err := db.Query(ctx, query, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var hour time.Time
		var valueStr string
		if err := rows.Scan(&hour, &valueStr); err != nil {
			return nil, err
		}

		var topicCounts map[string]int
		if err := json.Unmarshal([]byte(valueStr), &topicCounts); err != nil {
			return nil, fmt.Errorf("unmarshal error: %v", err)
		}

		results[hour] = TimeCountPerTopic{
			TopicCounts: topicCounts,
			Timestamp:   hour,
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	sortedResults := sortResults(results)
	return &MessageCountPerTopicResponse{TimeMessageCountPerTopic: sortedResults}, nil
}

// encore:api public path=/get-user-sentiment
func (s *Service) GetUserSentiment(ctx context.Context, req *MetricDurationRequest) (*UserSentimentResponse, error) {
	if req.Hours > 24 {
		return nil, errors.New("please provide a duration less than or equal to 24 hours")
	}

	endTime := time.Now().UTC().Truncate(time.Hour)
	startTime := endTime.Add(time.Duration(-int(req.Hours)) * time.Hour)
	query := `
	 		SELECT date_trunc('hour', timestamp) AS hour, value
	 		FROM community_insights
	 		WHERE type = 'sentiment_per_user' AND timestamp BETWEEN $1 AND $2
	 		ORDER BY hour
	 	`

	rows, err := db.Query(ctx, query, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	sentimentsPerUser := map[string][]float32{}
	for rows.Next() {
		var hour time.Time
		var valueStr string
		if err := rows.Scan(&hour, &valueStr); err != nil {
			return nil, err
		}

		authorsToSentimentStats := make(map[string]*models.MessageSentimentStats)
		if err := json.Unmarshal([]byte(valueStr), &authorsToSentimentStats); err != nil {
			return nil, fmt.Errorf("unmarshal error: %v", err)
		}

		for author, stats := range authorsToSentimentStats {
			if _, ok := sentimentsPerUser[author]; !ok {
				sentimentsPerUser[author] = []float32{}
			}

			sentiment := (float32(stats.Positive)*1.0 + float32(stats.Negative)*(-1.0)) /
				float32(stats.Positive+stats.Negative+stats.Neutral)
			sentimentsPerUser[author] = append(sentimentsPerUser[author], sentiment)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %v", err)
	}

	positiveSentiments := map[string]float32{}
	negativeSentiments := map[string]float32{}
	for userID, sentiments := range sentimentsPerUser {
		avgSentiment := lo.Sum(sentiments) / float32(len(sentiments))
		if math.Abs(float64(avgSentiment)) < 0.2 {
			rlog.Info("Skipping user with neutral sentiment", "user_id", userID)
			continue
		}

		discordUser, err := s.discordClient.User(userID)
		if err != nil {
			return nil, fmt.Errorf("error while trying to get user: %w", err)
		}

		if avgSentiment > 0 {
			positiveSentiments[discordUser.Username] = avgSentiment
		} else {
			negativeSentiments[discordUser.Username] = -avgSentiment
		}
	}

	return &UserSentimentResponse{
		PositiveSentiments: positiveSentiments,
		NegativeSentiments: negativeSentiments,
	}, nil
}

func initializeTimeCountPerTopic(start, end time.Time) map[time.Time]TimeCountPerTopic {
	results := make(map[time.Time]TimeCountPerTopic)
	topics := []string{"Feature Request", "Feedback", "Other", "Question", "Bug Report"}
	for t := start; !t.After(end); t = t.Add(time.Hour) {
		topicCounts := make(map[string]int)
		for _, topic := range topics {
			topicCounts[topic] = 0
		}
		results[t] = TimeCountPerTopic{
			Timestamp:   t,
			TopicCounts: topicCounts,
		}
	}
	return results
}

func sortResults(results map[time.Time]TimeCountPerTopic) []TimeCountPerTopic {
	var sortedResults []TimeCountPerTopic
	for _, result := range results {
		sortedResults = append(sortedResults, result)
	}
	sort.Slice(sortedResults, func(i, j int) bool {
		return sortedResults[i].Timestamp.Before(sortedResults[j].Timestamp)
	})
	return sortedResults
}
