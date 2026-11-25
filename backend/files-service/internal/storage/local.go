package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Local cuva sadrzaj datoteka na disku, ispod basePath.
// U Docker-u je basePath montiran volume, da sadrzaj prezivi restart kontejnera.
type Local struct {
	basePath string
}

func NewLocal(basePath string) (*Local, error) {
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("ne mogu da napravim %s: %w", basePath, err)
	}
	return &Local{basePath: basePath}, nil
}

// path deli datoteke u podfoldere po prva dva znaka kljuca, da se u jednom
// folderu ne nakupi na desetine hiljada fajlova.
func (l *Local) path(key string) string {
	return filepath.Join(l.basePath, key[:2], key)
}

func (l *Local) Put(ctx context.Context, key string, r io.Reader, size int64, _ string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	dest := l.path(key)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	// Prvo se pise u privremeni fajl pa se preimenuje: ako upis pukne na pola,
	// na mestu pravog fajla nikad ne ostane nepotpun sadrzaj.
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".upload-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName) // ako je rename uspeo, ovo ne radi nista
	}()

	written, err := io.Copy(tmp, r)
	if err != nil {
		return err
	}
	if size >= 0 && written != size {
		return fmt.Errorf("ocekivano %d bajtova, upisano %d", size, written)
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.Rename(tmpName, dest)
}

func (l *Local) Get(_ context.Context, key string) (io.ReadCloser, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	f, err := os.Open(l.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return f, err
}

func (l *Local) Delete(_ context.Context, key string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	err := os.Remove(l.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
