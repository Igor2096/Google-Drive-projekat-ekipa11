package main

import (
	"testing"
	"time"
)

func TestPasswordCanBeChangedAfterMinimumAge(t *testing.T) {
	now := time.Date(2026, time.November, 25, 12, 0, 0, 0, time.UTC)
	if PasswordCanBeChanged(now.Add(-23*time.Hour), now, 24*time.Hour) {
		t.Fatal("lozinka ne sme biti promenjena pre isteka 24 časa")
	}
	if !PasswordCanBeChanged(now.Add(-24*time.Hour), now, 24*time.Hour) {
		t.Fatal("lozinka mora biti promenljiva nakon 24 časa")
	}
}

func TestPasswordExpiredAfterMaximumAge(t *testing.T) {
	now := time.Date(2026, time.November, 25, 12, 0, 0, 0, time.UTC)
	if PasswordExpired(now.Add(-59*24*time.Hour), time.Time{}, now, 60*24*time.Hour) {
		t.Fatal("lozinka ne sme isteći pre 60 dana")
	}
	if !PasswordExpired(now.Add(-60*24*time.Hour), time.Time{}, now, 60*24*time.Hour) {
		t.Fatal("lozinka mora isteći nakon 60 dana")
	}
}
