package service

import (
	"context"
	"errors"
	"io"
	"log"
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

// usersClient je ono sto Files servisu treba od Users servisa.
// Interfejs stoji ovde (kod potrosaca), pa se u testovima lako zameni.
type usersClient interface {
	Quota(ctx context.Context, userID string) (users.Quota, error)
	AddUsedStorage(ctx context.Context, userID string, delta int64) error
}

type fileWriteStore interface {
	FindByName(ctx context.Context, ownerID string, folderID *bson.ObjectID, name string) (*model.File, error)
	FindByID(ctx context.Context, id bson.ObjectID) (*model.File, error)
	Create(ctx context.Context, f *model.File) error
	UpdateVersion(ctx context.Context, f *model.File) error
}

// FileService - otpremanje datoteka (zahtev 1.5, task T-23).
type FileService struct {
	folders folderStore
	files   fileWriteStore
	store   storage.Storage
	quotas  usersClient
}

func NewFileService(folders folderStore, files fileWriteStore, store storage.Storage, quotas usersClient) *FileService {
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
	key := newStorageKey()

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

	// T-25: zauzece se uvecava tek kada je datoteka stvarno sacuvana.
	s.reportStorageChange(ctx, ownerID, file.Size)

	return file, nil
}

// NewVersionInput - telo zahteva za novu verziju. Naziv i folder se ne salju:
// to je i dalje ista datoteka, menja se samo njen sadrzaj (zahtev 1.5).
type NewVersionInput struct {
	MimeType string
	Size     int64
	Content  io.Reader
}

// UploadNewVersion zamenjuje sadrzaj postojece datoteke (zahtev 1.5, task T-24).
//
// Novi sadrzaj ide pod NOV kljuc, a stari se brise tek kada metapodaci budu
// uspesno upisani. Da se pise preko starog kljuca, neuspeo upis u bazu bi
// ostavio datoteku sa starim metapodacima i novim sadrzajem.
func (s *FileService) UploadNewVersion(ctx context.Context, ownerID, fileID string, in NewVersionInput) (*model.File, error) {
	if in.Size < 0 {
		return nil, ErrInvalidName
	}

	id, err := bson.ObjectIDFromHex(fileID)
	if err != nil {
		return nil, ErrInvalidID
	}
	file, err := s.files.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}
	if file.OwnerID != ownerID {
		return nil, ErrForbidden
	}

	// Kvota se proverava samo na PRIRAST. Ako je nova verzija manja ili iste
	// velicine, korisnik ne zauzima nista novo i Users servis se ne zove.
	if err := s.checkQuota(ctx, ownerID, in.Size-file.Size); err != nil {
		return nil, err
	}

	mimeType := in.MimeType
	if mimeType == "" {
		mimeType = file.MimeType
	}

	// Stara velicina se pamti pre izmene: od nje zavisi prijava zauzeca,
	// a repozitorijum ne garantuje da vraceni zapis ostaje nepromenjen.
	oldKey, oldSize := file.StorageKey, file.Size
	newKey := newStorageKey()
	if err := s.store.Put(ctx, newKey, in.Content, in.Size, mimeType); err != nil {
		return nil, err
	}

	updated := *file
	updated.StorageKey = newKey
	updated.Size = in.Size
	updated.MimeType = mimeType
	updated.Version = file.Version + 1
	updated.UpdatedAt = time.Now().UTC()

	if err := s.files.UpdateVersion(ctx, &updated); err != nil {
		// Metapodaci nisu promenjeni, pa stara verzija i dalje vazi:
		// brise se novi sadrzaj, stari ostaje netaknut.
		_ = s.store.Delete(context.WithoutCancel(ctx), newKey)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}

	// Stari sadrzaj vise nikome ne treba. Ako brisanje ne uspe, ostaje
	// bezopasno smece na disku - datoteka je vec ispravno azurirana.
	_ = s.store.Delete(context.WithoutCancel(ctx), oldKey)

	// T-25: zauzece se menja za razliku velicina; negativno ako je nova
	// verzija manja od stare.
	s.reportStorageChange(ctx, ownerID, updated.Size-oldSize)

	return &updated, nil
}

// reportStorageChange javlja Users servisu koliko se promenilo zauzece
// korisnika (zahtev 1.8, task T-25).
//
// Namerno NE obara zahtev ako poziv ne uspe: datoteka je u tom trenutku vec
// sacuvana i korisniku bi bilo vraceno "greska" za nesto sto je proslo.
// Posledica je da zauzece moze da zaostane za stvarnim stanjem; ispravno
// resenje je ponavljanje poziva preko reda poruka (outbox), sto izlazi iz
// obima ovog sprinta. Zato se neuspeh bar glasno loguje.
func (s *FileService) reportStorageChange(ctx context.Context, ownerID string, delta int64) {
	if delta == 0 {
		return
	}
	if err := s.quotas.AddUsedStorage(context.WithoutCancel(ctx), ownerID, delta); err != nil {
		log.Printf("UPOZORENJE: zauzece korisnika %s nije azurirano za %d bajtova: %v", ownerID, delta, err)
	}
}

// newStorageKey pravi nov, jedinstven kljuc za sadrzaj u object store-u.
func newStorageKey() string {
	return bson.NewObjectID().Hex()
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
