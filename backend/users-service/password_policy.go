package main

import "time"

func PasswordCanBeChanged(changedAt, now time.Time, minimumAge time.Duration) bool {
	return !changedAt.IsZero() && !now.Before(changedAt.Add(minimumAge))
}
