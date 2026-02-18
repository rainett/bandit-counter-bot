package repository

import (
	"bandit-counter-bot/internal/domain"
	"database/sql"
	"fmt"
)

type UserStatsRepo struct {
	db *sql.DB
}

func NewUserStatsRepo(db *sql.DB) *UserStatsRepo {
	return &UserStatsRepo{db: db}
}

func (r *UserStatsRepo) Spin(chatId int64, userId int64, username string, winDelta int64, delta int64) error {
	_, err := r.db.Exec(`
		INSERT INTO user_stats (chat_id, user_id, username, spins, wins, balance)
		VALUES (?, ?, ?, 1, ?, ?)
		ON CONFLICT(chat_id, user_id) DO UPDATE SET
			username = excluded.username,
			spins = spins + 1,
			wins = wins + excluded.wins,
			balance = balance + excluded.balance`,
		chatId, userId, username, winDelta, delta,
	)
	return err
}

func (r *UserStatsRepo) GetPersonalStats(chatId int64, userId int64) (domain.PersonalStats, error) {
	var stats domain.PersonalStats
	err := r.db.QueryRow(`
		SELECT spins,
		       wins,
		       balance,
		       (SELECT count(DISTINCT balance) + 1
		        FROM user_stats
		        WHERE chat_id = ?
		          AND balance > us.balance) AS rank
		FROM user_stats us
		WHERE chat_id = ?
		  AND user_id = ?`,
		chatId, chatId, userId).Scan(&stats.Spins, &stats.Wins, &stats.Balance, &stats.Rank)
	return stats, err
}

func (r *UserStatsRepo) GetRichStats(chatId int64) ([]domain.RatingStats, error) {
	return r.getRatingStats(chatId, ">", "DESC")
}

func (r *UserStatsRepo) GetDebtorsStats(chatId int64) ([]domain.RatingStats, error) {
	return r.getRatingStats(chatId, "<", "ASC")
}

func (r *UserStatsRepo) getRatingStats(chatId int64, rankOp string, orderDir string) ([]domain.RatingStats, error) {
	query := fmt.Sprintf(`
		SELECT
			us.username,
			us.balance,
			us.spins,
			us.wins,
			(
				SELECT COUNT(DISTINCT balance) + 1
				FROM user_stats
				WHERE chat_id = us.chat_id
				  AND balance %s us.balance
			) AS rank
		FROM user_stats us
		WHERE us.chat_id = ?
		ORDER BY us.balance %s`, rankOp, orderDir)

	rows, err := r.db.Query(query, chatId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []domain.RatingStats
	for rows.Next() {
		var stats domain.RatingStats
		if err := rows.Scan(&stats.Username, &stats.Balance, &stats.Spins, &stats.Wins, &stats.Rank); err != nil {
			return nil, err
		}
		res = append(res, stats)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}
