package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"files-service/internal/model"
	"files-service/internal/repository"
	"files-service/internal/storage"
	"files-service/internal/users"
)

var (
	ErrFileNotFound       = errors.New("datoteka ne postoji")
	ErrFileDuplicate      = errors.New("datoteka sa tim nazivom vec postoji u ovom folderu")
	ErrQuotaExceeded      = errors.New("nemate dovoljno slobodnog prostora")
	ErrUnknownUser        = errors.New("korisnik ne postoji")
	ErrUsersUnavailable   = errors.New("provera kvote trenutno nije moguca")
	ErrFileTypeNotAllowed = errors.New("ovaj tip datoteke nije dozvoljen")
)

// DefaultAllowedTypes - bela lista tipova datoteka (zahtev 2.18).
//
// Tip se odredjuje po SADRZAJU (prvih 512 bajtova, "magicni bajtovi"), a ne po
// ekstenziji ili zaglavlju koje salje klijent - oba moze lako da se podmetne.
// Namerno NISU dozvoljeni text/html i text/xml (ukljucujuci SVG): mogu da nose
// JavaScript i da posluze za XSS napad ako se ikad prikazu u browseru.
// Izvrsni fajlovi se prepoznaju kao application/octet-stream i takodje padaju.
var DefaultAllowedTypes = []string{
	"text/plain", // .txt, .csv, .json, .md, izvorni kod...
	"application/pdf",
	"image/png", "image/jpeg", "image/gif", "image/webp", "image/bmp",
	"application/zip", // .zip, a i .docx, .xlsx, .pptx (to su ZIP arhive)
	"audio/mpeg", "audio/wave",
	"video/mp4", "video/webm",
}

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
	RenameFile(ctx context.Context, id bson.ObjectID, name string, updatedAt time.Time) error
}

// FileService - otpremanje, nove verzije, preimenovanje i provera integriteta datoteka.
type FileService struct {
	folders folderStore
	files   fileWriteStore
	store   storage.Storage
	quotas  usersClient
	allowed map[string]bool
}

// NewFileService - allowedTypes prazno znaci DefaultAllowedTypes.
func NewFileService(folders folderStore, files fileWriteStore, store storage.Storage, quotas usersClient, allowedTypes []string) *FileService {
	if len(allowedTypes) == 0 {
		allowedTypes = DefaultAllowedTypes
	}
	allowed := make(map[string]bool, len(allowedTypes))
	for _, t := range allowedTypes {
		allowed[strings.ToLower(strings.TrimSpace(t))] = true
	}
	return &FileService{folders: folders, files: files, store: store, quotas: quotas, allowed: allowed}
}

// UploadInput - ono sto handler izvuce iz multipart zahteva.
// MIME tip se NE prima od klijenta: servis ga sam odredjuje iz sadrzaja.
type UploadInput struct {
	Name     string    // naziv datoteke
	FolderID string    // prazno => koren ("Moj disk")
	Size     int64     // velicina u bajtovima
	Content  io.Reader // sam sadrzaj
}

// Upload upisuje sadrzaj u object store, pa metapodatke u Mongo.
//
// Redosled je bitan: prvo sve besplatne lokalne provere (naziv, folder,
// duplikat, tip sadrzaja), pa tek onda mrezni poziv ka Users servisu za kvotu,
// pa upis. Ako upis metapodataka pukne, sadrzaj se brise iz store-a da ne
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
	// Prava garancija je jedinstven indeks u bazi.
	switch _, err := s.files.FindByName(ctx, ownerID, folderID, name); {
	case err == nil:
		return nil, ErrFileDuplicate
	case errors.Is(err, repository.ErrNotFound):
		// slobodno ime - nastavlja se dalje
	default:
		return nil, err
	}

	content, err := s.inspect(in.Content)
	if err != nil {
		return nil, err
	}

	// Kvota se proverava sinhrono kod Users servisa (2.5), pre upisa.
	if err := s.checkQuota(ctx, ownerID, in.Size); err != nil {
		return nil, err
	}

	id := bson.NewObjectID()
	key := newStorageKey()
	if err := s.store.Put(ctx, key, content.body, in.Size, content.mimeType); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	file := &model.File{
		ID:         id,
		Name:       name,
		OwnerID:    ownerID,
		FolderID:   folderID,
		MimeType:   content.mimeType,
		Size:       in.Size,
		Checksum:   content.checksum(),
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

	// Zauzece se uvecava tek kada je datoteka stvarno sacuvana.
	s.reportStorageChange(ctx, ownerID, file.Size)

	return file, nil
}

// NewVersionInput - telo zahteva za novu verziju. Naziv i folder se ne salju:
// to je i dalje ista datoteka, menja se samo njen sadrzaj (zahtev 1.5).
type NewVersionInput struct {
	Size    int64
	Content io.Reader
}

// UploadNewVersion zamenjuje sadrzaj postojece datoteke (zahtev 1.5).
//
// Novi sadrzaj ide pod NOV kljuc, a stari se brise tek kada metapodaci budu
// uspesno upisani. Da se pise preko starog kljuca, neuspeo upis u bazu bi
// ostavio datoteku sa starim metapodacima i novim sadrzajem.
func (s *FileService) UploadNewVersion(ctx context.Context, ownerID, fileID string, in NewVersionInput) (*model.File, error) {
	if in.Size < 0 {
		return nil, ErrInvalidName
	}
	file, err := s.ownedFile(ctx, ownerID, fileID)
	if err != nil {
		return nil, err
	}

	content, err := s.inspect(in.Content)
	if err != nil {
		return nil, err
	}

	// Kvota se proverava samo na PRIRAST. Ako je nova verzija manja ili iste
	// velicine, korisnik ne zauzima nista novo i Users servis se ne zove.
	if err := s.checkQuota(ctx, ownerID, in.Size-file.Size); err != nil {
		return nil, err
	}

	// Stara velicina se pamti pre izmene: od nje zavisi prijava zauzeca,
	// a repozitorijum ne garantuje da vraceni zapis ostaje nepromenjen.
	oldKey, oldSize := file.StorageKey, file.Size
	newKey := newStorageKey()
	if err := s.store.Put(ctx, newKey, content.body, in.Size, content.mimeType); err != nil {
		return nil, err
	}

	updated := *file
	updated.StorageKey = newKey
	updated.Size = in.Size
	updated.MimeType = content.mimeType
	updated.Checksum = content.checksum()
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

	// Zauzece se menja za razliku velicina; negativno ako je nova verzija manja.
	s.reportStorageChange(ctx, ownerID, updated.Size-oldSize)

	return &updated, nil
}

// RenameFile menja naziv datoteke (zahtev 1.6). Sadrzaj i verzija se ne menjaju.
func (s *FileService) RenameFile(ctx context.Context, ownerID, fileID, newName string) (*model.File, error) {
	name, err := ValidateName(newName)
	if err != nil {
		return nil, err
	}
	file, err := s.ownedFile(ctx, ownerID, fileID)
	if err != nil {
		return nil, err
	}
	if file.Name == name {
		return file, nil
	}

	now := time.Now().UTC()
	if err := s.files.RenameFile(ctx, file.ID, name, now); err != nil {
		switch {
		case errors.Is(err, repository.ErrDuplicate):
			return nil, ErrFileDuplicate
		case errors.Is(err, repository.ErrNotFound):
			return nil, ErrFileNotFound
		default:
			return nil, err
		}
	}
	file.Name = name
	file.UpdatedAt = now
	return file, nil
}

// IntegrityResult - ishod provere integriteta jedne datoteke.
type IntegrityResult struct {
	FileID   string `json:"fileId"`
	Expected string `json:"expected"` // SHA-256 zabelezen pri otpremanju
	Actual   string `json:"actual"`   // SHA-256 onoga sto je sada u skladistu
	Valid    bool   `json:"valid"`
}

// VerifyIntegrity ponovo cita sacuvani sadrzaj, racuna SHA-256 i poredi ga sa
// otiskom zabelezenim pri otpremanju (zahtev 2.18). Ako je neko izmenio fajl
// direktno u skladistu, mimo aplikacije, otisci se nece poklopiti.
func (s *FileService) VerifyIntegrity(ctx context.Context, ownerID, fileID string) (*IntegrityResult, error) {
	file, err := s.ownedFile(ctx, ownerID, fileID)
	if err != nil {
		return nil, err
	}
	result := &IntegrityResult{FileID: file.ID.Hex(), Expected: file.Checksum}

	content, err := s.store.Get(ctx, file.StorageKey)
	if errors.Is(err, storage.ErrNotFound) {
		return result, nil // sadrzaja nema - integritet narusen
	}
	if err != nil {
		return nil, err
	}
	defer content.Close()

	h := sha256.New()
	if _, err := io.Copy(h, content); err != nil {
		return nil, err
	}
	result.Actual = hex.EncodeToString(h.Sum(nil))
	result.Valid = result.Expected != "" && result.Expected == result.Actual
	return result, nil
}

// --- pomocne funkcije ---

// inspectedContent - sadrzaj ciji je tip proveren, uz SHA-256 koji se racuna
// "u prolazu" dok se sadrzaj upisuje (bez drugog citanja).
type inspectedContent struct {
	mimeType string
	body     io.Reader
	hasher   hash.Hash
}

func (c *inspectedContent) checksum() string {
	return hex.EncodeToString(c.hasher.Sum(nil))
}

// inspect cita pocetak sadrzaja, odredjuje tip i proverava ga prema beloj listi.
// Vraceni body ponovo daje CEO sadrzaj, ukljucujuci vec procitan pocetak.
func (s *FileService) inspect(content io.Reader) (*inspectedContent, error) {
	head := make([]byte, 512) // http.DetectContentType gleda najvise 512 bajtova
	n, err := io.ReadFull(content, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	head = head[:n]

	mimeType := DetectType(head)
	if !s.allowed[mimeType] {
		return nil, ErrFileTypeNotAllowed
	}

	hasher := sha256.New()
	body := io.TeeReader(io.MultiReader(bytes.NewReader(head), content), hasher)
	return &inspectedContent{mimeType: mimeType, body: body, hasher: hasher}, nil
}

// DetectType vraca MIME tip bez parametara ("text/plain; charset=utf-8" -> "text/plain").
func DetectType(head []byte) string {
	t := http.DetectContentType(head)
	if i := strings.IndexByte(t, ';'); i >= 0 {
		t = t[:i]
	}
	return strings.ToLower(strings.TrimSpace(t))
}

// ownedFile pronalazi datoteku i proverava da pripada korisniku.
func (s *FileService) ownedFile(ctx context.Context, ownerID, fileID string) (*model.File, error) {
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
	return file, nil
}

// reportStorageChange javlja Users servisu koliko se promenilo zauzece
// korisnika (zahtev 1.8).
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
// (rad i kada susedni servis padne) je zahtev 2.6.
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
