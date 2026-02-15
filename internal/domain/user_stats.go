package domain

type PersonalStats struct {
	Spins   int64
	Wins    int64
	Balance int64
	Rank    int64
}

type RatingStats struct {
	Username string
	Spins    int64
	Wins     int64
	Balance  int64
	Rank     int64
}
