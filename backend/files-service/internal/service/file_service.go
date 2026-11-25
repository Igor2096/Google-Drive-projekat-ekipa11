package service

import (
	"context"
	"errors"
	"io"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"files-service/internal/model"
	"files-service/internal/repository"
	"files-service/internal/storage"
	"files-service/internal/users"
)

var (
	ErrFileNotFound     = errors.New("datoteka ne postoji")
	ErrFileDuplicate    = errors.New("datoteka sa tim nazivom vec postoji u ovom folderu")
	ErrQuotaExceeded    = errors.New("nemate dovoljno slobodnog prostora")
	ErrUnknownUser      = errors.New("korisnik ne postoji")
	ErrUsersUnavailable = errors.New("provera kvote trenutno nije moguca")
)

const defaultMimeType = "application/octet-stream"

// quotaChecker je ono sto Files servisu treba od Users servisa.
// Interfejs stoji ovde (kod potrosaca), pa se u testovima lako zameni.
type quotaChecker interface {
	Quota(ctx context.Context, userID string) (users.Quota, error)
}

type fileWriteStore interface {
	FindByName(ctx context.Context, ownerID string, folderID *bson.ObjectID, name string) (*model.File, error)
	Create(ctx context.Context, f *model.File) error
}

// FileService - otpremanje datoteka (zahtev 1.5, task T-23).
type FileService struct {
	folders folderStore
	files   fileWriteStore
	store   storage.Storage
	quotas  quotaChecker
}

func NewFileService(folders folderStore, files fileWriteStore, store storage.Storage, quotas quotaChecker) *FileService {
	return &FileService{folders: folders, files: files, store: store, quotas: quotas}
}

// UploadInput - ono sto handler izvuce iz multipart zahteva.
type UploadInput struct {
	Name     string    // naziv datoteke
	FolderID string    // prazno => koren ("Moj disk")
	MimeType string    // iz zaglavlja dela multipart zahteva
	Size     int64     // velicina u bajtovima
	Content  io.Reader // sam sadrzaj
}

// Upload upisuje sadrzaj u object store, pa metapodatke u Mongo.
//
// Redosled je bitan: ID dokumenta se generise unapred jer od njega nastaje
// StorageKey. Ako upis metapodataka pukne, sadrzaj se brise iz store-a da ne
// ostanu "siroceta" - bajtovi na koje nijedan dokument ne pokazuje.
func (s *FileService) Upload(ctx context.Context, ownerID string, in UploadInput) (*model.File, error) {
	name, err := ValidateName(in.Name)
	if err != nil {
		return nil, err
	}
	if in.Size < 0 {
		return nil, ErrInvalidName
	}

	// Koren ili podfolder - u oba slucaja folder mora da bude korisnikov (T-13).
	var folderID *bson.ObjectID
	if in.FolderID != "" {
		folder, err := ownedFolder(ctx, s.folders, ownerID, in.FolderID)
		if err != nil {
			return nil, err
		}
		folderID = &folder.ID
	}

	// Provera duplikata unapred, da se veliki fajl ne upisuje uzalud.
	// Prava garancija je jedinstven indeks u bazi, provera ispod.
	switch _, err := s.files.FindByName(ctx, ownerID, folderID, name); {
	case err == nil:
		return nil, ErrFileDuplicate
	case errors.Is(err, repository.ErrNotFound):
		// slobodno ime - nastavlja se dalje
	default:
		return nil, err
	}

	// T-23: kvota se proverava sinhrono kod Users servisa, pre nego sto se
	// isporuci i jedan bajt. Lokalne provere idu pre ovoga jer su besplatne.
	if err := s.checkQuota(ctx, ownerID, in.Size); err != nil {
		return nil, err
	}

	id := bson.NewObjectID()
	key := id.Hex()

	mimeType := in.MimeType
	if mimeType == "" {
		mimeType = defaultMimeType
	}

	if err := s.store.Put(ctx, key, in.Content, in.Size, mimeType); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	file := &model.File{
		ID:         id,
		Name:       name,
		OwnerID:    ownerID,
		FolderID:   folderID,
		MimeType:   mimeType,
		Size:       in.Size,
		StorageKey: key,
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.files.Create(ctx, file); err != nil {
		// Kompenzacija: metapodaci nisu upisani, pa sadrzaj ne sme da ostane.
		_ = s.store.Delete(context.WithoutCancel(ctx), key)
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, ErrFileDuplicate
		}
		return nil, err
	}
	return file, nil
}

// checkQuota pita Users servis koliko je korisniku ostalo i poredi sa
// trazenim prirastom (needed). Za novu datoteku to je njena velicina.
//
// Ako Users servis ne odgovara, otpremanje se odbija umesto da se propusti:
// bolje je odbiti nego dozvoliti korisniku da probije kvotu. Blaza varijanta
// (rad i kada susedni servis padne) je zahtev 2.6 i dolazi kasnije.
func (s *FileService) checkQuota(ctx context.Context, ownerID string, needed int64) error {
	if needed <= 0 {
		return nil
	}
	quota, err := s.quotas.Quota(ctx, ownerID)
	switch {
	case errors.Is(err, users.ErrUserNotFound):
		return ErrUnknownUser
	case err != nil:
		return ErrUsersUnavailable
	}
	if needed > quota.AvailableStorage {
		return ErrQuotaExceeded
	}
	return nil
}

// ownedFolder - zajednicka provera vlasnistva nad folderom (T-13).
// Koriste je i FolderService i FileService.
func ownedFolder(ctx context.Context, folders folderStore, ownerID, folderID string) (*model.Folder, error) {
	id, err := bson.ObjectIDFromHex(folderID)
	if err != nil {
		return nil, ErrInvalidID
	}
	folder, err := folders.FindByID(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	if folder.OwnerID != ownerID {
		return nil, ErrForbidden
	}
	return folder, nil
}
