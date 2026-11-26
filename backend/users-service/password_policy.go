package main

import "time"

func PasswordCanBeChanged(changedAt, now time.Time, minimumAge time.Duration) bool {
	return !changedAt.IsZero() && !now.Before(changedAt.Add(minimumAge))
}

func PasswordExpired(changedAt, createdAt, now time.Time, maximumAge time.Duration) bool {
	if changedAt.IsZero() {
		changedAt = createdAt
	}
	return changedAt.IsZero() || !now.Before(changedAt.Add(maximumAge))
}
