ALTER TABLE chat_settings ADD COLUMN allow_user_settings INTEGER NOT NULL DEFAULT 0;
ALTER TABLE chat_settings ADD COLUMN allow_user_reset INTEGER NOT NULL DEFAULT 0;
