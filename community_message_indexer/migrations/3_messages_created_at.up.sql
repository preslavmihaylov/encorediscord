ALTER TABLE discord_messages
ADD COLUMN created_at TIMESTAMP NOT NULL DEFAULT now();
