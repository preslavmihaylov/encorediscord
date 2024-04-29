CREATE TABLE conversation_alerts (
    ID SERIAL PRIMARY KEY,
    topics TEXT[],
    keywords TEXT[],
    channel_id VARCHAR(255)
);

