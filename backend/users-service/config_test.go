package main

import "testing"

func TestPositiveInt64Env(t *testing.T) {
	t.Setenv("DEFAULT_STORAGE_QUOTA_BYTES", "2048")
	value, err := positiveInt64Env("DEFAULT_STORAGE_QUOTA_BYTES", 1024)
	if err != nil || value != 2048 {
		t.Fatalf("dobijeno value=%d, err=%v", value, err)
	}

	t.Setenv("DEFAULT_STORAGE_QUOTA_BYTES", "0")
	if _, err := positiveInt64Env("DEFAULT_STORAGE_QUOTA_BYTES", 1024); err == nil {
		t.Fatal("nulta kvota mora biti odbijena")
	}
}
