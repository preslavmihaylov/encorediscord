package communityinsights

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

type MessageCountRequest struct {
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

// encore:api public path=/get-message-counts
func GetMessageCounts(ctx context.Context, req *MessageCountRequest) (*MessageCountResponse, error) {
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
func GetMessageCountsPerTopic(ctx context.Context, req *MessageCountRequest) (*MessageCountPerTopicResponse, error) {
	if req.Hours > 24 {
		return nil, errors.New("please provide a duration less than or equal to 24 hours")
	}

	endTime := time.Now().UTC().Truncate(time.Hour)
	startTime := endTime.Add(time.Duration(-int(req.Hours)) * time.Hour)

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

	var results []TimeCountPerTopic
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

		results = append(results, TimeCountPerTopic{
			Timestamp:   hour,
			TopicCounts: topicCounts,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.Before(results[j].Timestamp)
	})

	return &MessageCountPerTopicResponse{TimeMessageCountPerTopic: results}, nil
}
