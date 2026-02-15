CREATE TABLE IF NOT EXISTS chat_settings(
    chat_id INTEGER PRIMARY KEY,
    prize_values TEXT NOT NULL DEFAULT '[64]'
);

UPDATE schema_version SET version = 3;