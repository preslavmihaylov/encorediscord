-- create table discord_forum_posts with id and discord_id
CREATE TABLE discord_forum_posts (
    id VARCHAR(255) PRIMARY KEY,
    discord_id VARCHAR(255) NOT NULL,
    CONSTRAINT uniq_discord_id UNIQUE(discord_id)
);
