package repository

import (
	"bandit-counter-bot/internal/domain"
	"database/sql"
	"errors"
)

type UserStatsRepo struct {
	db *sql.DB
}

func NewUserStatsRepo(db *sql.DB) *UserStatsRepo {
	return &UserStatsRepo{db: db}
}

func (r *UserStatsRepo) Spin(chatId int64, userId int64, username string, winDelta int64, delta int64) error {
	return r.executeUpdate(`
		INSERT OR IGNORE INTO user_stats (chat_id, user_id, username) VALUES (?, ?, ?);
		UPDATE user_stats SET spins = spins + 1, wins = wins + ?, balance = balance + ?
		WHERE chat_id = ? AND user_id = ?`,
		chatId, userId, username,
		winDelta, delta,
		chatId, userId,
	)
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
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return stats, err
		}
		return stats, err
	}
	return stats, nil
}

func (r *UserStatsRepo) executeUpdate(query string, args ...interface{}) error {
	_, err := r.db.Exec(query, args...)
	if err != nil {
		return err
	}
	return nil
}

func (r *UserStatsRepo) GetRichStats(chatId int64) ([]domain.RatingStats, error) {
	rows, err := r.db.Query(`
		SELECT
			us.username,
			us.balance,
			us.spins,
			us.wins,
			(
				SELECT COUNT(DISTINCT balance) + 1
				FROM user_stats
				WHERE chat_id = us.chat_id
				  AND balance > us.balance
			) AS rank
		FROM user_stats us
		WHERE us.chat_id = ?
		ORDER BY us.balance DESC`, chatId)
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
	return res, nil
}

func (r *UserStatsRepo) GetDebtorsStats(chatId int64) ([]domain.RatingStats, error) {
	rows, err := r.db.Query(`
		SELECT
			us.username,
			us.balance,
			us.spins,
			us.wins,
			(
				SELECT COUNT(DISTINCT balance) + 1
				FROM user_stats
				WHERE chat_id = us.chat_id
				  AND balance < us.balance
			) AS rank
		FROM user_stats us
		WHERE us.chat_id = ?
		ORDER BY us.balance ASC
	`, chatId)
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
	return res, nil
}
