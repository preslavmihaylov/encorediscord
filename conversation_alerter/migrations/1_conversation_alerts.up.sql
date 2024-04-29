CREATE TABLE conversation_alerts (
    id SERIAL PRIMARY KEY,
    topics TEXT[],
    keywords TEXT[],
    channel_id VARCHAR(255)
);

