package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
)

type SettingsRepo struct {
	db *sql.DB
}

func NewSettingsRepo(db *sql.DB) *SettingsRepo {
	return &SettingsRepo{db: db}
}

func (r *SettingsRepo) UpdatePrizeValues(values string, chatId int64) error {
	_, err := r.db.Exec(`
		INSERT OR IGNORE INTO chat_settings (chat_id) VALUES (?);
		UPDATE chat_settings SET prize_values = ? WHERE chat_id = ?;`,
		chatId, values, chatId)
	return err
}

func (r *SettingsRepo) GetPrizeValues(chatId int64) ([]int, error) {
	defaultValue := []int{64}
	var raw string
	err := r.db.QueryRow(`SELECT prize_values FROM chat_settings WHERE chat_id = ?`,
		chatId).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return defaultValue, nil
		}
		return nil, err
	}
	var values []int
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		log.Println("invalid prize_values json for chat:", chatId)
		return defaultValue, nil
	}
	if len(values) == 0 {
		return defaultValue, nil
	}
	return values, nil
}
