PRAGMA journal_mode=WAL;

CREATE TABLE IF NOT EXISTS user_stats (
    chat_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,

    spins INTEGER NOT NULL DEFAULT 0,
    wins INTEGER NOT NULL DEFAULT 0,
    balance INTEGER NOT NULL DEFAULT 0,

    PRIMARY KEY (chat_id, user_id)
);

CREATE INDEX IF NOT EXISTS user_stats_chat_balance_idx
ON user_stats(chat_id, balance DESC);

INSERT OR IGNORE INTO schema_version(version) VALUES (0);
UPDATE schema_version SET version = 1;
