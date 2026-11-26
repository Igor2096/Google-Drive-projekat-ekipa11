package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"files-service/internal/model"
	"files-service/internal/repository"
	"files-service/internal/storage"
	"files-service/internal/users"
)

// --- fake Users servis ---

type fakeQuota struct {
	available int64
	err       error
	calls     int
	// zabelezene promene zauzeca (T-25)
	deltas   []int64
	addErr   error
	addCalls int
}

func (f *fakeQuota) Quota(context.Context, string) (users.Quota, error) {
	f.calls++
	if f.err != nil {
		return users.Quota{}, f.err
	}
	return users.Quota{StorageQuota: f.available, UsedStorage: 0, AvailableStorage: f.available}, nil
}

func (f *fakeQuota) AddUsedStorage(_ context.Context, _ string, delta int64) error {
	f.addCalls++
	if f.addErr != nil {
		return f.addErr
	}
	f.deltas = append(f.deltas, delta)
	return nil
}

// --- fake storage u memoriji ---

type fakeStorage struct {
	objects map[string]string
	failPut bool
}

func newFakeStorage() *fakeStorage { return &fakeStorage{objects: map[string]string{}} }

func (f *fakeStorage) Put(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	if f.failPut {
		return errors.New("storage nije dostupan")
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	f.objects[key] = string(b)
	return nil
}
func (f *fakeStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	content, ok := f.objects[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return io.NopCloser(strings.NewReader(content)), nil
}
func (f *fakeStorage) Delete(_ context.Context, key string) error {
	delete(f.objects, key)
	return nil
}

// --- fake repozitorijum datoteka ---

type fakeFileStore struct {
	items      []model.File
	failCreate error
	failUpdate error
}

func (f *fakeFileStore) FindByName(_ context.Context, owner string, folder *bson.ObjectID, name string) (*model.File, error) {
	for i := range f.items {
		if f.items[i].OwnerID == owner && f.items[i].Name == name && sameParent(f.items[i].FolderID, folder) {
			return &f.items[i], nil
		}
	}
	return nil, repository.ErrNotFound
}
func (f *fakeFileStore) FindByID(_ context.Context, id bson.ObjectID) (*model.File, error) {
	for i := range f.items {
		if f.items[i].ID == id {
			found := f.items[i] // kopija, kao sto i pravi repozitorijum vraca
			return &found, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (f *fakeFileStore) UpdateVersion(_ context.Context, file *model.File) error {
	if f.failUpdate != nil {
		return f.failUpdate
	}
	for i := range f.items {
		if f.items[i].ID == file.ID {
			f.items[i] = *file
			return nil
		}
	}
	return repository.ErrNotFound
}
func (f *fakeFileStore) RenameFile(_ context.Context, id bson.ObjectID, name string, updatedAt time.Time) error {
	for i := range f.items {
		if f.items[i].ID == id {
			for _, other := range f.items {
				if other.ID != id && other.OwnerID == f.items[i].OwnerID && other.Name == name && sameParent(other.FolderID, f.items[i].FolderID) {
					return repository.ErrDuplicate
				}
			}
			f.items[i].Name, f.items[i].UpdatedAt = name, updatedAt
			return nil
		}
	}
	return repository.ErrNotFound
}
func (f *fakeFileStore) Create(_ context.Context, file *model.File) error {
	if f.failCreate != nil {
		return f.failCreate
	}
	f.items = append(f.items, *file)
	return nil
}

func newFileSvc() (*FileService, *fakeFolders, *fakeFileStore, *fakeStorage) {
	svc, folders, files, store, _ := newFileSvcQuota(1 << 30) // 1 GB, dovoljno za sve testove
	return svc, folders, files, store
}

func newFileSvcQuota(available int64) (*FileService, *fakeFolders, *fakeFileStore, *fakeStorage, *fakeQuota) {
	folders := &fakeFolders{items: map[bson.ObjectID]model.Folder{}}
	files := &fakeFileStore{}
	store := newFakeStorage()
	quota := &fakeQuota{available: available}
	return NewFileService(folders, files, store, quota, nil), folders, files, store, quota
}

func upload(svc *FileService, owner, name, folderID, body string) (*model.File, error) {
	return svc.Upload(context.Background(), owner, UploadInput{
		Name:     name,
		FolderID: folderID,
		Size:     int64(len(body)),
		Content:  strings.NewReader(body),
	})
}

// --- testovi ---

func TestUploadToRoot(t *testing.T) {
	svc, _, files, store := newFileSvc()

	file, err := upload(svc, "u1", "beleske.txt", "", "zdravo")
	if err != nil {
		t.Fatal(err)
	}
	if file.FolderID != nil {
		t.Fatal("datoteka u korenu mora imati folderId = nil")
	}
	if file.Version != 1 || file.Size != 6 || file.MimeType != "text/plain" {
		t.Fatalf("losi metapodaci: %+v", file)
	}
	// Od T-24 kljuc sadrzaja vise nije vezan za ID dokumenta: svaka verzija
	// dobija svoj kljuc, da neuspeo upis u bazu ne pregazi staru verziju.
	if file.StorageKey == "" {
		t.Fatal("StorageKey nije postavljen")
	}
	if store.objects[file.StorageKey] != "zdravo" {
		t.Fatal("sadrzaj nije stigao u storage")
	}
	if len(files.items) != 1 {
		t.Fatal("metapodaci nisu upisani")
	}
}

func TestUploadIntoFolderChecksOwnership(t *testing.T) {
	svc, _, _, _ := newFileSvc()
	folderSvc := NewFolderService(svc.folders, fakeFiles{})

	folder, err := folderSvc.CreateFolder(context.Background(), "u1", "Fakultet", "")
	if err != nil {
		t.Fatal(err)
	}

	file, err := upload(svc, "u1", "rad.pdf", folder.ID.Hex(), "x")
	if err != nil {
		t.Fatal(err)
	}
	if file.FolderID == nil || *file.FolderID != folder.ID {
		t.Fatal("datoteka nije smestena u trazeni folder")
	}

	// tudji folder
	if _, err := upload(svc, "u2", "upad.pdf", folder.ID.Hex(), "x"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ocekivano ErrForbidden, dobijeno %v", err)
	}
	// nepostojeci folder
	if _, err := upload(svc, "u1", "a.pdf", bson.NewObjectID().Hex(), "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ocekivano ErrNotFound, dobijeno %v", err)
	}
	// ID koji nije ObjectID
	if _, err := upload(svc, "u1", "a.pdf", "nije-id", "x"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("ocekivano ErrInvalidID, dobijeno %v", err)
	}
}

func TestUploadDuplicateName(t *testing.T) {
	svc, _, _, store := newFileSvc()

	if _, err := upload(svc, "u1", "isto.txt", "", "prvi"); err != nil {
		t.Fatal(err)
	}
	if _, err := upload(svc, "u1", "isto.txt", "", "drugi"); !errors.Is(err, ErrFileDuplicate) {
		t.Fatalf("ocekivano ErrFileDuplicate, dobijeno %v", err)
	}
	if len(store.objects) != 1 {
		t.Fatal("sadrzaj drugog pokusaja nije smeo da zavrsi u storage-u")
	}
	// drugi korisnik sme da ima isti naziv
	if _, err := upload(svc, "u2", "isto.txt", "", "treci"); err != nil {
		t.Fatal(err)
	}
}

// Ako upis metapodataka pukne, sadrzaj mora da se obrise iz storage-a.
func TestUploadCompensatesWhenMetadataFails(t *testing.T) {
	svc, _, files, store := newFileSvc()
	files.failCreate = errors.New("baza nije dostupna")

	if _, err := upload(svc, "u1", "a.txt", "", "sadrzaj"); err == nil {
		t.Fatal("ocekivana greska")
	}
	if len(store.objects) != 0 {
		t.Fatalf("u storage-u je ostalo sirce: %v", store.objects)
	}
}

func TestUploadRejectsBadName(t *testing.T) {
	svc, _, _, store := newFileSvc()
	for _, name := range []string{"", "   ", "..", "a/b.txt"} {
		if _, err := upload(svc, "u1", name, "", "x"); !errors.Is(err, ErrInvalidName) {
			t.Errorf("naziv %q je trebalo da bude odbijen, dobijeno %v", name, err)
		}
	}
	if len(store.objects) != 0 {
		t.Fatal("nista nije smelo da se upise")
	}
}

func TestUploadStorageFailureLeavesNoMetadata(t *testing.T) {
	svc, _, files, store := newFileSvc()
	store.failPut = true

	if _, err := upload(svc, "u1", "a.txt", "", "x"); err == nil {
		t.Fatal("ocekivana greska")
	}
	if len(files.items) != 0 {
		t.Fatal("metapodaci su upisani iako sadrzaj nije")
	}
}

// --- T-23: provera kvote kod Users servisa ---

func TestUploadRejectedWhenOverQuota(t *testing.T) {
	svc, _, files, store, quota := newFileSvcQuota(5)

	_, err := upload(svc, "u1", "veliko.txt", "", "deset bajt")
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("ocekivano ErrQuotaExceeded, dobijeno %v", err)
	}
	if quota.calls != 1 {
		t.Fatalf("Users servis je pozvan %d puta", quota.calls)
	}
	if len(store.objects) != 0 || len(files.items) != 0 {
		t.Fatal("nista nije smelo da se upise kada kvota nije dovoljna")
	}
}

func TestUploadAllowedWhenExactlyFits(t *testing.T) {
	svc, _, _, _, _ := newFileSvcQuota(6)
	if _, err := upload(svc, "u1", "tacno.txt", "", "zdravo"); err != nil {
		t.Fatalf("datoteka koja tacno staje je odbijena: %v", err)
	}
}

func TestUploadRejectedWhenUsersServiceDown(t *testing.T) {
	svc, _, files, store, quota := newFileSvcQuota(1 << 30)
	quota.err = users.ErrUnavailable

	if _, err := upload(svc, "u1", "a.txt", "", "x"); !errors.Is(err, ErrUsersUnavailable) {
		t.Fatalf("ocekivano ErrUsersUnavailable, dobijeno %v", err)
	}
	if len(store.objects) != 0 || len(files.items) != 0 {
		t.Fatal("bez potvrdjene kvote nista ne sme da se upise")
	}
}

func TestUploadRejectedForUnknownUser(t *testing.T) {
	svc, _, _, _, quota := newFileSvcQuota(1 << 30)
	quota.err = users.ErrUserNotFound

	if _, err := upload(svc, "nepostojeci", "a.txt", "", "x"); !errors.Is(err, ErrUnknownUser) {
		t.Fatalf("ocekivano ErrUnknownUser, dobijeno %v", err)
	}
}

// Prazna datoteka ne zauzima prostor, pa nema razloga zvati Users servis.
func TestEmptyFileSkipsQuotaCall(t *testing.T) {
	svc, _, _, _, quota := newFileSvcQuota(0)

	if _, err := upload(svc, "u1", "prazno.txt", "", ""); err != nil {
		t.Fatal(err)
	}
	if quota.calls != 0 {
		t.Fatalf("Users servis nije trebalo zvati, pozvan %d puta", quota.calls)
	}
}

// Lokalne provere (naziv, vlasnistvo) moraju da se dese pre mreznog poziva.
func TestBadRequestDoesNotCallUsersService(t *testing.T) {
	svc, _, _, _, quota := newFileSvcQuota(1 << 30)

	_, _ = upload(svc, "u1", "a/b.txt", "", "x")                     // nevalidan naziv
	_, _ = upload(svc, "u1", "a.txt", bson.NewObjectID().Hex(), "x") // nepostojeci folder
	if quota.calls != 0 {
		t.Fatalf("Users servis je pozvan %d puta, a nije trebalo nijednom", quota.calls)
	}
}

// --- T-24: otpremanje nove verzije ---

func newVersion(svc *FileService, owner, fileID, body string) (*model.File, error) {
	return svc.UploadNewVersion(context.Background(), owner, fileID, NewVersionInput{
		Size:    int64(len(body)),
		Content: strings.NewReader(body),
	})
}

func TestNewVersionReplacesContent(t *testing.T) {
	svc, _, files, store, _ := newFileSvcQuota(1 << 30)

	first, err := upload(svc, "u1", "rad.txt", "", "prva verzija")
	if err != nil {
		t.Fatal(err)
	}
	oldKey := first.StorageKey

	second, err := newVersion(svc, "u1", first.ID.Hex(), "druga")
	if err != nil {
		t.Fatal(err)
	}

	if second.Version != 2 {
		t.Fatalf("verzija je %d, ocekivano 2", second.Version)
	}
	if second.Name != "rad.txt" || second.ID != first.ID {
		t.Fatal("nova verzija mora da zadrzi naziv i ID iste datoteke")
	}
	if second.Size != 5 {
		t.Fatalf("velicina je %d, ocekivano 5", second.Size)
	}
	if !second.UpdatedAt.After(first.UpdatedAt) && !second.UpdatedAt.Equal(first.UpdatedAt) {
		t.Fatal("vreme poslednje izmene nije zabelezeno")
	}
	if store.objects[second.StorageKey] != "druga" {
		t.Fatal("novi sadrzaj nije upisan")
	}
	if _, ostao := store.objects[oldKey]; ostao {
		t.Fatal("stari sadrzaj je trebalo obrisati")
	}
	if len(store.objects) != 1 || len(files.items) != 1 {
		t.Fatalf("ocekivana jedna datoteka, imamo %d objekata i %d zapisa", len(store.objects), len(files.items))
	}
}

func TestNewVersionChecksOwnership(t *testing.T) {
	svc, _, _, _, _ := newFileSvcQuota(1 << 30)
	file, _ := upload(svc, "u1", "moje.txt", "", "sadrzaj")

	if _, err := newVersion(svc, "u2", file.ID.Hex(), "upad"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ocekivano ErrForbidden, dobijeno %v", err)
	}
	if _, err := newVersion(svc, "u1", bson.NewObjectID().Hex(), "x"); !errors.Is(err, ErrFileNotFound) {
		t.Fatalf("ocekivano ErrFileNotFound, dobijeno %v", err)
	}
	if _, err := newVersion(svc, "u1", "nije-id", "x"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("ocekivano ErrInvalidID, dobijeno %v", err)
	}
}

// Kvota se gleda samo na prirast: manja verzija ne trosi nista novo.
func TestNewVersionQuotaCountsOnlyGrowth(t *testing.T) {
	svc, _, _, _, quota := newFileSvcQuota(10)

	file, err := upload(svc, "u1", "a.txt", "", "0123456789") // potroseno svih 10
	if err != nil {
		t.Fatal(err)
	}
	quota.available = 0 // nema vise slobodnog prostora
	callsBefore := quota.calls

	if _, err := newVersion(svc, "u1", file.ID.Hex(), "kratko"); err != nil {
		t.Fatalf("manja verzija je odbijena: %v", err)
	}
	if quota.calls != callsBefore {
		t.Fatal("Users servis nije trebalo zvati kada nova verzija ne zauzima vise")
	}
}

func TestNewVersionRejectedWhenGrowthExceedsQuota(t *testing.T) {
	svc, _, _, store, quota := newFileSvcQuota(1 << 30)

	file, _ := upload(svc, "u1", "a.txt", "", "malo")
	oldKey := file.StorageKey
	quota.available = 2 // staje jos samo 2 bajta

	if _, err := newVersion(svc, "u1", file.ID.Hex(), "mnogo duzi sadrzaj"); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("ocekivano ErrQuotaExceeded, dobijeno %v", err)
	}
	if store.objects[oldKey] != "malo" {
		t.Fatal("stara verzija mora da ostane netaknuta")
	}
	if len(store.objects) != 1 {
		t.Fatal("novi sadrzaj nije smeo da ostane u storage-u")
	}
}

// Ako upis metapodataka pukne, stara verzija mora da ostane ispravna.
func TestNewVersionCompensatesWhenMetadataFails(t *testing.T) {
	svc, _, files, store, _ := newFileSvcQuota(1 << 30)

	file, _ := upload(svc, "u1", "a.txt", "", "stara")
	oldKey := file.StorageKey
	files.failUpdate = errors.New("baza nije dostupna")

	if _, err := newVersion(svc, "u1", file.ID.Hex(), "nova"); err == nil {
		t.Fatal("ocekivana greska")
	}
	if store.objects[oldKey] != "stara" {
		t.Fatal("stari sadrzaj je izgubljen")
	}
	if len(store.objects) != 1 {
		t.Fatalf("u storage-u je ostalo sirce: %v", store.objects)
	}
	if files.items[0].Version != 1 {
		t.Fatal("broj verzije se ne sme uvecati ako upis nije uspeo")
	}
}

// --- T-25: azuriranje zauzeca skladista ---

func TestUploadReportsStorageUsed(t *testing.T) {
	svc, _, _, _, quota := newFileSvcQuota(1 << 30)

	if _, err := upload(svc, "u1", "a.txt", "", "sedam!!"); err != nil {
		t.Fatal(err)
	}
	if len(quota.deltas) != 1 || quota.deltas[0] != 7 {
		t.Fatalf("prijavljene promene zauzeca: %v, ocekivano [7]", quota.deltas)
	}
}

func TestNewVersionReportsOnlyDifference(t *testing.T) {
	svc, _, _, _, quota := newFileSvcQuota(1 << 30)

	file, _ := upload(svc, "u1", "a.txt", "", "12345") // +5
	if _, err := newVersion(svc, "u1", file.ID.Hex(), "1234567890"); err != nil {
		t.Fatal(err)
	}
	if len(quota.deltas) != 2 || quota.deltas[1] != 5 {
		t.Fatalf("prijavljene promene: %v, ocekivano [5 5]", quota.deltas)
	}
}

func TestSmallerNewVersionReportsNegativeDelta(t *testing.T) {
	svc, _, _, _, quota := newFileSvcQuota(1 << 30)

	file, _ := upload(svc, "u1", "a.txt", "", "1234567890") // +10
	if _, err := newVersion(svc, "u1", file.ID.Hex(), "123"); err != nil {
		t.Fatal(err)
	}
	if len(quota.deltas) != 2 || quota.deltas[1] != -7 {
		t.Fatalf("prijavljene promene: %v, ocekivano [10 -7]", quota.deltas)
	}
}

// Neuspesno azuriranje zauzeca ne sme da obori otpremanje:
// datoteka je u tom trenutku vec sacuvana.
func TestUploadSucceedsEvenIfReportingFails(t *testing.T) {
	svc, _, files, store, quota := newFileSvcQuota(1 << 30)
	quota.addErr = errors.New("users servis pao")

	file, err := upload(svc, "u1", "a.txt", "", "sadrzaj")
	if err != nil {
		t.Fatalf("otpremanje je palo zbog prijave zauzeca: %v", err)
	}
	if file == nil || len(files.items) != 1 || len(store.objects) != 1 {
		t.Fatal("datoteka mora da ostane sacuvana")
	}
}

// Nema promene velicine - nema ni poziva ka Users servisu.
func TestNoReportWhenSizeUnchanged(t *testing.T) {
	svc, _, _, _, quota := newFileSvcQuota(1 << 30)

	file, _ := upload(svc, "u1", "a.txt", "", "12345")
	callsBefore := quota.addCalls

	if _, err := newVersion(svc, "u1", file.ID.Hex(), "54321"); err != nil {
		t.Fatal(err)
	}
	if quota.addCalls != callsBefore {
		t.Fatal("prijava zauzeca nije trebalo da se desi kada se velicina nije promenila")
	}
}

func TestEmptyFileReportsNothing(t *testing.T) {
	svc, _, _, _, quota := newFileSvcQuota(1 << 30)

	if _, err := upload(svc, "u1", "prazno.txt", "", ""); err != nil {
		t.Fatal(err)
	}
	if quota.addCalls != 0 {
		t.Fatal("prazna datoteka ne menja zauzece")
	}
}

// --- bela lista tipova i otisak sadrzaja (2.18) ---

func TestUploadRejectsDisallowedContentRegardlessOfName(t *testing.T) {
	svc, _, files, store, quota := newFileSvcQuota(1 << 30)

	cases := map[string]string{
		"stranica.html": "<!DOCTYPE html><html><script>alert(1)</script></html>",
		"slika.svg":     `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"izvestaj.pdf":  "MZ\x90\x00 ovo je zapravo izvrsni fajl", // laziran naziv, sadrzaj je .exe
		"program.exe":   "\x7fELF\x02\x01\x01 binarni fajl",
	}
	for name, body := range cases {
		if _, err := upload(svc, "u1", name, "", body); !errors.Is(err, ErrFileTypeNotAllowed) {
			t.Errorf("%s: ocekivano ErrFileTypeNotAllowed, dobijeno %v", name, err)
		}
	}
	if len(files.items) != 0 || len(store.objects) != 0 {
		t.Fatal("nijedan odbijen fajl ne sme da stigne do skladista ni baze")
	}
	if quota.calls != 0 {
		t.Fatal("tip se proverava lokalno, pre poziva ka Users servisu")
	}
}

func TestUploadDetectsTypeFromContent(t *testing.T) {
	svc, _, _, _, _ := newFileSvcQuota(1 << 30)

	png := "\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 20)
	file, err := upload(svc, "u1", "fotka.txt", "", png) // pogresna ekstenzija
	if err != nil {
		t.Fatal(err)
	}
	if file.MimeType != "image/png" {
		t.Fatalf("tip se odredjuje po sadrzaju, dobijeno %q", file.MimeType)
	}
}

func TestUploadStoresSHA256Checksum(t *testing.T) {
	svc, _, _, _, _ := newFileSvcQuota(1 << 30)

	file, err := upload(svc, "u1", "b.txt", "", "abc")
	if err != nil {
		t.Fatal(err)
	}
	const sha256OfABC = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if file.Checksum != sha256OfABC {
		t.Fatalf("checksum %q", file.Checksum)
	}
}

func TestNewVersionUpdatesChecksum(t *testing.T) {
	svc, _, _, _, _ := newFileSvcQuota(1 << 30)
	first, _ := upload(svc, "u1", "a.txt", "", "abc")
	second, err := newVersion(svc, "u1", first.ID.Hex(), "abcd")
	if err != nil {
		t.Fatal(err)
	}
	if second.Checksum == first.Checksum || second.Checksum == "" {
		t.Fatal("nova verzija mora imati nov otisak")
	}
}

func TestNewVersionRejectsDisallowedType(t *testing.T) {
	svc, _, _, store, _ := newFileSvcQuota(1 << 30)
	first, _ := upload(svc, "u1", "a.txt", "", "abc")

	if _, err := newVersion(svc, "u1", first.ID.Hex(), "<html><script>x</script></html>"); !errors.Is(err, ErrFileTypeNotAllowed) {
		t.Fatalf("ocekivano ErrFileTypeNotAllowed, dobijeno %v", err)
	}
	if store.objects[first.StorageKey] != "abc" || len(store.objects) != 1 {
		t.Fatal("stara verzija mora ostati netaknuta")
	}
}

func TestCustomAllowList(t *testing.T) {
	folders := &fakeFolders{items: map[bson.ObjectID]model.Folder{}}
	svc := NewFileService(folders, &fakeFileStore{}, newFakeStorage(), &fakeQuota{available: 1 << 30}, []string{"application/pdf"})

	if _, err := upload(svc, "u1", "a.txt", "", "obican tekst"); !errors.Is(err, ErrFileTypeNotAllowed) {
		t.Fatalf("tekst nije na listi, dobijeno %v", err)
	}
	if _, err := upload(svc, "u1", "a.pdf", "", "%PDF-1.7 sadrzaj"); err != nil {
		t.Fatalf("PDF je na listi: %v", err)
	}
}

// --- provera integriteta ---

func TestVerifyIntegrity(t *testing.T) {
	svc, _, _, store, _ := newFileSvcQuota(1 << 30)
	file, _ := upload(svc, "u1", "a.txt", "", "originalni sadrzaj")

	res, err := svc.VerifyIntegrity(context.Background(), "u1", file.ID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid || res.Actual != file.Checksum {
		t.Fatalf("netaknuta datoteka mora proci proveru: %+v", res)
	}

	store.objects[file.StorageKey] = "neko je ovo izmenio mimo aplikacije"
	res, _ = svc.VerifyIntegrity(context.Background(), "u1", file.ID.Hex())
	if res.Valid {
		t.Fatal("izmenjen sadrzaj mora pasti na proveri integriteta")
	}

	delete(store.objects, file.StorageKey)
	res, _ = svc.VerifyIntegrity(context.Background(), "u1", file.ID.Hex())
	if res.Valid {
		t.Fatal("nestao sadrzaj mora pasti na proveri integriteta")
	}

	if _, err := svc.VerifyIntegrity(context.Background(), "u2", file.ID.Hex()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("tudja datoteka: ocekivano ErrForbidden, dobijeno %v", err)
	}
}

// --- preimenovanje datoteke (1.6) ---

func TestRenameFile(t *testing.T) {
	svc, _, files, _, _ := newFileSvcQuota(1 << 30)
	a, _ := upload(svc, "u1", "a.txt", "", "x")
	_, _ = upload(svc, "u1", "b.txt", "", "y")

	renamed, err := svc.RenameFile(context.Background(), "u1", a.ID.Hex(), "  novi naziv.txt ")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Name != "novi naziv.txt" || files.items[0].Name != "novi naziv.txt" {
		t.Fatalf("naziv nije promenjen: %+v", renamed)
	}
	if renamed.Version != 1 {
		t.Fatal("preimenovanje ne sme da menja verziju")
	}

	if _, err := svc.RenameFile(context.Background(), "u1", a.ID.Hex(), "b.txt"); !errors.Is(err, ErrFileDuplicate) {
		t.Fatalf("ocekivano ErrFileDuplicate, dobijeno %v", err)
	}
	if _, err := svc.RenameFile(context.Background(), "u2", a.ID.Hex(), "upad.txt"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ocekivano ErrForbidden, dobijeno %v", err)
	}
	if _, err := svc.RenameFile(context.Background(), "u1", a.ID.Hex(), "a/b.txt"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("ocekivano ErrInvalidName, dobijeno %v", err)
	}
	if _, err := svc.RenameFile(context.Background(), "u1", bson.NewObjectID().Hex(), "x.txt"); !errors.Is(err, ErrFileNotFound) {
		t.Fatalf("ocekivano ErrFileNotFound, dobijeno %v", err)
	}
}
