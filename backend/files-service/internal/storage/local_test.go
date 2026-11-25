package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPutGetDelete(t *testing.T) {
	s, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	key := "507f1f77bcf86cd799439011"
	data := []byte("sadrzaj datoteke")

	if err := s.Put(ctx, key, bytes.NewReader(data), int64(len(data)), "text/plain"); err != nil {
		t.Fatal(err)
	}

	r, err := s.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(r)
	r.Close()
	if !bytes.Equal(got, data) {
		t.Fatalf("procitano %q, ocekivano %q", got, data)
	}

	if err := s.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ocekivano ErrNotFound, dobijeno %v", err)
	}
	// brisanje nepostojeceg kljuca nije greska
	if err := s.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
}

func TestPutOverwrites(t *testing.T) {
	s, _ := NewLocal(t.TempDir())
	ctx, key := context.Background(), "507f1f77bcf86cd799439011"

	_ = s.Put(ctx, key, strings.NewReader("prva verzija"), 12, "")
	if err := s.Put(ctx, key, strings.NewReader("druga"), 5, ""); err != nil {
		t.Fatal(err)
	}
	r, _ := s.Get(ctx, key)
	got, _ := io.ReadAll(r)
	r.Close()
	if string(got) != "druga" {
		t.Fatalf("dobijeno %q", got)
	}
}

func TestInvalidKeys(t *testing.T) {
	s, _ := NewLocal(t.TempDir())
	ctx := context.Background()
	for _, key := range []string{"", "../../etc/passwd", "a/b", `a\b`, "..", "kljuc sa razmakom", strings.Repeat("a", 200)} {
		if err := s.Put(ctx, key, strings.NewReader("x"), 1, ""); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("kljuc %q je trebalo da bude odbijen, dobijeno %v", key, err)
		}
	}
}

// Ako deklarisana velicina ne odgovara stvarnoj, upis se odbacuje
// i na disku ne sme da ostane nista.
func TestSizeMismatchLeavesNothing(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewLocal(dir)
	key := "507f1f77bcf86cd799439011"

	if err := s.Put(context.Background(), key, strings.NewReader("samo 5"), 999, ""); err == nil {
		t.Fatal("ocekivana greska zbog neslaganja velicine")
	}
	if _, err := os.Stat(filepath.Join(dir, key[:2], key)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("fajl je ostao na disku iako upis nije uspeo")
	}
}
