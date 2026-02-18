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

func (r *UserStatsRepo) Spin(chatId int64, userId int64, username string, win bool, winAmount int64) error {
	var balanceDelta int64
	var winDelta int64
	var winFlag int64
	if win {
		balanceDelta = winAmount
		winDelta = 1
		winFlag = 1
	} else {
		balanceDelta = -1
		winDelta = 0
		winFlag = 0
	}

	return r.executeUpdate(`
		INSERT INTO user_stats (chat_id, user_id, username, spins, wins, balance,
			current_streak, max_streak, current_loss_streak, max_loss_streak)
		VALUES (?, ?, ?, 1, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(chat_id, user_id) DO UPDATE SET
			username = excluded.username,
			spins = spins + 1,
			wins = wins + excluded.wins,
			balance = balance + excluded.balance,
			current_streak = CASE WHEN ? = 1 THEN current_streak + 1 ELSE 0 END,
			max_streak = CASE WHEN ? = 1 THEN MAX(max_streak, current_streak + 1) ELSE max_streak END,
			current_loss_streak = CASE WHEN ? = 0 THEN current_loss_streak + 1 ELSE 0 END,
			max_loss_streak = CASE WHEN ? = 0 THEN MAX(max_loss_streak, current_loss_streak + 1) ELSE max_loss_streak END`,
		chatId, userId, username, winDelta, balanceDelta,
		winFlag, winFlag, 1-winFlag, 1-winFlag,
		winFlag, winFlag, winFlag, winFlag,
	)
}

func (r *UserStatsRepo) GetPersonalStats(chatId int64, userId int64) (domain.PersonalStats, error) {
	var stats domain.PersonalStats
	err := r.db.QueryRow(`
		WITH ranked AS (
			SELECT user_id, spins, wins, balance,
			       current_streak, max_streak, current_loss_streak, max_loss_streak,
			       DENSE_RANK() OVER (ORDER BY balance DESC) AS rank
			FROM user_stats WHERE chat_id = ?
		)
		SELECT spins, wins, balance, current_streak, max_streak,
		       current_loss_streak, max_loss_streak, rank
		FROM ranked WHERE user_id = ?`,
		chatId, userId).Scan(&stats.Spins, &stats.Wins, &stats.Balance,
		&stats.CurrentStreak, &stats.MaxStreak,
		&stats.CurrentLossStreak, &stats.MaxLossStreak, &stats.Rank)
	if err != nil {
		return stats, err
	}
	if stats.Spins > 0 {
		stats.Luck = float64(stats.Wins) / float64(stats.Spins) * 100
	}
	return stats, nil
}

func (r *UserStatsRepo) GetRichStats(chatId int64) ([]domain.RatingStats, error) {
	rows, err := r.db.Query(`
		SELECT username, spins, wins, balance,
		       DENSE_RANK() OVER (ORDER BY balance DESC) AS rank
		FROM user_stats
		WHERE chat_id = ?
		ORDER BY balance DESC`, chatId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []domain.RatingStats
	for rows.Next() {
		var s domain.RatingStats
		if err := rows.Scan(&s.Username, &s.Spins, &s.Wins, &s.Balance, &s.Rank); err != nil {
			return nil, err
		}
		res = append(res, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func (r *UserStatsRepo) GetDebtorsStats(chatId int64) ([]domain.RatingStats, error) {
	rows, err := r.db.Query(`
		SELECT username, spins, wins, balance,
		       DENSE_RANK() OVER (ORDER BY balance ASC) AS rank
		FROM user_stats
		WHERE chat_id = ?
		ORDER BY balance ASC`, chatId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []domain.RatingStats
	for rows.Next() {
		var s domain.RatingStats
		if err := rows.Scan(&s.Username, &s.Spins, &s.Wins, &s.Balance, &s.Rank); err != nil {
			return nil, err
		}
		res = append(res, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func (r *UserStatsRepo) GetLuckyStats(chatId int64) ([]domain.RatingStats, error) {
	rows, err := r.db.Query(`
		SELECT username, spins, wins, balance,
		       CASE WHEN spins > 0 THEN CAST(wins AS REAL) / spins * 100 ELSE 0 END AS luck,
		       DENSE_RANK() OVER (ORDER BY CASE WHEN spins > 0 THEN CAST(wins AS REAL) / spins ELSE 0 END DESC) AS rank
		FROM user_stats
		WHERE chat_id = ?
		ORDER BY luck DESC`, chatId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []domain.RatingStats
	for rows.Next() {
		var s domain.RatingStats
		if err := rows.Scan(&s.Username, &s.Spins, &s.Wins, &s.Balance, &s.Luck, &s.Rank); err != nil {
			return nil, err
		}
		res = append(res, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func (r *UserStatsRepo) GetStreakStats(chatId int64) ([]domain.RatingStats, error) {
	rows, err := r.db.Query(`
		SELECT username, spins, wins, max_streak, max_loss_streak,
		       DENSE_RANK() OVER (ORDER BY max_streak DESC) AS rank
		FROM user_stats
		WHERE chat_id = ?
		ORDER BY max_streak DESC`, chatId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []domain.RatingStats
	for rows.Next() {
		var s domain.RatingStats
		if err := rows.Scan(&s.Username, &s.Spins, &s.Wins, &s.MaxStreak, &s.MaxLossStreak, &s.Rank); err != nil {
			return nil, err
		}
		res = append(res, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func (r *UserStatsRepo) ResetChat(chatId int64) error {
	return r.executeUpdate(`DELETE FROM user_stats WHERE chat_id = ?`, chatId)
}

func (r *UserStatsRepo) executeUpdate(query string, args ...interface{}) error {
	_, err := r.db.Exec(query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	return nil
}
