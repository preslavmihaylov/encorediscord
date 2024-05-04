CREATE TABLE discord_community_messages (
    id VARCHAR(255) PRIMARY KEY,
    discord_message_id VARCHAR(255) NOT NULL,
    CONSTRAINT uniq_discord_message_id UNIQUE(discord_message_id)
);
