CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE discord_messages (
    id VARCHAR(255) PRIMARY KEY,
    interaction_type int,
    channel_id VARCHAR(255),
    guild_id VARCHAR(255),
    author_id VARCHAR(255),
    content TEXT,
    clean_content TEXT
);

CREATE TABLE discord_messages_search (
    id VARCHAR(255) PRIMARY KEY,
    content_normalized TEXT,

    FOREIGN KEY (id) REFERENCES discord_messages(id)
);
