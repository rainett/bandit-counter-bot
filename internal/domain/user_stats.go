package domain

type PersonalStats struct {
	Spins             int64
	Wins              int64
	Balance           int64
	Rank              int64
	CurrentStreak     int64
	MaxStreak         int64
	CurrentLossStreak int64
	MaxLossStreak     int64
	Luck              float64
}

type RatingStats struct {
	Username          string
	Spins             int64
	Wins              int64
	Balance           int64
	Rank              int64
	CurrentStreak     int64
	MaxStreak         int64
	CurrentLossStreak int64
	MaxLossStreak     int64
	Luck              float64
}
