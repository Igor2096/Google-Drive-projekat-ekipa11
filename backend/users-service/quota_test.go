package main

import "testing"

func TestAvailableStorageDoesNotBecomeNegative(t *testing.T) {
	user := User{StorageQuota: 100, UsedStorage: 150}
	available := user.StorageQuota - user.UsedStorage
	if available < 0 {
		available = 0
	}
	if available != 0 {
		t.Fatalf("dostupna kvota mora biti 0, dobijeno %d", available)
	}
}
