ALTER TABLE community_insights ADD CONSTRAINT type_timestamp_unique UNIQUE (type, timestamp);
