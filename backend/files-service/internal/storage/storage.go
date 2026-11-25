// Package storage - mesto gde se cuva SADRZAJ datoteka.
//
// Metapodaci (naziv, velicina, vlasnik...) idu u MongoDB, a sami bajtovi ovde.
// Interfejs postoji da bi se implementacija mogla zameniti bez diranja ostatka
// servisa: sada je to lokalni disk, a za zahtev 2.7 dolazi MinIO (task T-22) -
// tada se pise nova implementacija ovog istog interfejsa i menja samo cmd/main.go.
package storage

import (
	"context"
	"errors"
	"io"
	"strings"
)

var (
	ErrNotFound   = errors.New("sadrzaj datoteke ne postoji")
	ErrInvalidKey = errors.New("kljuc nije validan")
)

// Storage je apstrakcija nad object store-om.
// Kljuc (key) je identifikator sadrzaja; cuva se u polju File.StorageKey.
type Storage interface {
	// Put upisuje sadrzaj pod datim kljucem. Ako kljuc vec postoji, prepisuje ga
	// (to koristi otpremanje nove verzije - task T-25).
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	// Get vraca sadrzaj; pozivalac mora da zatvori ReadCloser.
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete brise sadrzaj. Brisanje nepostojeceg kljuca nije greska.
	Delete(ctx context.Context, key string) error
}

// ValidateKey stiti od putanja tipa "../../etc/passwd".
// Kljuceve generise servis (hex ObjectID), ali provera je ovde da bi vazila
// za svaku implementaciju.
func ValidateKey(key string) error {
	if key == "" || len(key) > 128 {
		return ErrInvalidKey
	}
	if strings.ContainsAny(key, `/\`) || strings.Contains(key, "..") {
		return ErrInvalidKey
	}
	for _, r := range key {
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
		if !isHex && r != '-' && r != '_' {
			return ErrInvalidKey
		}
	}
	return nil
}
