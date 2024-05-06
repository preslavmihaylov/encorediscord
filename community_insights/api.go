package communityinsights

import (
	"context"
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

// encore:api public path=/get-message-counts
func GetMessageCounts(ctx context.Context, req *MessageCountRequest) (*MessageCountResponse, error) {
	endTime := time.Now().Truncate(time.Hour)
	startTime := endTime.Add(time.Duration(-int(req.Hours)) * time.Hour)

	query := `
		SELECT date_trunc('hour', timestamp) AS hour, COALESCE(SUM((value->>'count')::INTEGER), 0) AS count
		FROM community_insights
		WHERE type = 'message_count' AND timestamp BETWEEN $1 AND $2
		GROUP BY hour
		ORDER BY hour DESC
	`

	rows, err := db.Query(ctx, query, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var timeCounts []TimeCountPair
	for rows.Next() {
		var pair TimeCountPair
		err = rows.Scan(&pair.Timestamp, &pair.Count)
		if err != nil {
			return nil, err
		}
		timeCounts = append(timeCounts, pair)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &MessageCountResponse{TimeCounts: timeCounts}, nil
}
