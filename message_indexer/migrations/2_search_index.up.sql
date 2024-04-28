create index if not exists 
idx_gin_content_normalized on discord_messages_search using gin (content_normalized gin_trgm_ops);
