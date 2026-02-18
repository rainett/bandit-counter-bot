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
		INSERT INTO chat_settings (chat_id, prize_values) VALUES (?, ?)
		ON CONFLICT(chat_id) DO UPDATE SET prize_values = excluded.prize_values`,
		chatId, values)
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

func (r *SettingsRepo) GetWinAmount(chatId int64) (int64, error) {
	var amount int64
	err := r.db.QueryRow(`SELECT win_amount FROM chat_settings WHERE chat_id = ?`,
		chatId).Scan(&amount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 64, nil
		}
		return 0, err
	}
	return amount, nil
}

func (r *SettingsRepo) UpdateWinAmount(amount int64, chatId int64) error {
	_, err := r.db.Exec(`
		INSERT INTO chat_settings (chat_id, win_amount) VALUES (?, ?)
		ON CONFLICT(chat_id) DO UPDATE SET win_amount = excluded.win_amount`,
		chatId, amount)
	return err
}

func (r *SettingsRepo) GetPrizeMode(chatId int64) (string, error) {
	prizeValues, err := r.GetPrizeValues(chatId)
	if err != nil {
		return "", err
	}
	return prizeValuesToMode(prizeValues), nil
}

func prizeValuesToMode(values []int) string {
	if len(values) == 1 && values[0] == 43 {
		return "lemons"
	}
	if len(values) == 4 {
		return "three_in_a_row"
	}
	return "classic"
}
