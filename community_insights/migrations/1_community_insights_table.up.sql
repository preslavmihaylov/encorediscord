CREATE TABLE community_insights (
    id VARCHAR(255) PRIMARY KEY,
    type VARCHAR(255),
    timestamp TIMESTAMP,
    value JSON
);
