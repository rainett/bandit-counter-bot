PRAGMA journal_mode=WAL;

ALTER TABLE user_stats
ADD COLUMN username TEXT NOT NULL DEFAULT 'ноунейм';

UPDATE schema_version SET version = 2;
