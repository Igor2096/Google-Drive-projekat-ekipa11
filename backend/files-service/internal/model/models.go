// Package model - entiteti Files servisa (zahtev 2.1 / T-11).
package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Folder - korenski folder korisnika NIJE poseban dokument:
// folder ciji je ParentID == nil nalazi se direktno u korenu ("Moj disk").
type Folder struct {
	ID       bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	Name     string         `bson:"name"          json:"name"`
	OwnerID  string         `bson:"ownerId"       json:"ownerId"`
	ParentID *bson.ObjectID `bson:"parentId"      json:"parentId"` // nil = koren
	// Ancestors - ID-jevi svih nadredjenih foldera, od korena ka roditelju.
	// Olaksava breadcrumbs, a kasnije i brisanje celog podstabla (1.13)
	// i deljenje foldera "sa svim sadrzajem" (1.10).
	Ancestors []bson.ObjectID `bson:"ancestors" json:"ancestors"`
	CreatedAt time.Time       `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time       `bson:"updatedAt" json:"updatedAt"`
}

// File - metapodaci o datoteci. Sam sadrzaj ce se cuvati u object store-u (2.7),
// ovde se cuva samo referenca na njega (StorageKey).
type File struct {
	ID         bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	Name       string         `bson:"name"          json:"name"`
	OwnerID    string         `bson:"ownerId"       json:"ownerId"`
	FolderID   *bson.ObjectID `bson:"folderId"      json:"folderId"` // nil = koren
	MimeType   string         `bson:"mimeType"      json:"mimeType"`
	Size       int64          `bson:"size"          json:"size"`     // u bajtovima
	Checksum   string         `bson:"checksum"      json:"checksum"` // SHA-256 sadrzaja (2.18)
	StorageKey string         `bson:"storageKey"    json:"-"`
	Version    int            `bson:"version"       json:"version"`
	CreatedAt  time.Time      `bson:"createdAt"     json:"createdAt"`
	UpdatedAt  time.Time      `bson:"updatedAt"     json:"updatedAt"` // vreme poslednje izmene (nova verzija)
}

// Breadcrumb - jedan korak u putanji "Moj disk > A > B".
type Breadcrumb struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// FolderContents - odgovor za pregled sadrzaja foldera (T-20 / 1.6).
type FolderContents struct {
	Folder      *Folder      `json:"folder"` // nil kada je u pitanju koren
	Breadcrumbs []Breadcrumb `json:"breadcrumbs"`
	Folders     []Folder     `json:"folders"`
	Files       []File       `json:"files"`
}
