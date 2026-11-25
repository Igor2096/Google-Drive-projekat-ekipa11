package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"files-service/internal/model"
	"files-service/internal/repository"
)

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
func (f *fakeStorage) Get(context.Context, string) (io.ReadCloser, error) { return nil, nil }
func (f *fakeStorage) Delete(_ context.Context, key string) error {
	delete(f.objects, key)
	return nil
}

// --- fake repozitorijum datoteka ---

type fakeFileStore struct {
	items      []model.File
	failCreate error
}

func (f *fakeFileStore) FindByName(_ context.Context, owner string, folder *bson.ObjectID, name string) (*model.File, error) {
	for i := range f.items {
		if f.items[i].OwnerID == owner && f.items[i].Name == name && sameParent(f.items[i].FolderID, folder) {
			return &f.items[i], nil
		}
	}
	return nil, repository.ErrNotFound
}
func (f *fakeFileStore) Create(_ context.Context, file *model.File) error {
	if f.failCreate != nil {
		return f.failCreate
	}
	f.items = append(f.items, *file)
	return nil
}

func newFileSvc() (*FileService, *fakeFolders, *fakeFileStore, *fakeStorage) {
	folders := &fakeFolders{items: map[bson.ObjectID]model.Folder{}}
	files := &fakeFileStore{}
	store := newFakeStorage()
	return NewFileService(folders, files, store), folders, files, store
}

func upload(svc *FileService, owner, name, folderID, body string) (*model.File, error) {
	return svc.Upload(context.Background(), owner, UploadInput{
		Name:     name,
		FolderID: folderID,
		MimeType: "text/plain",
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
	if file.StorageKey != file.ID.Hex() {
		t.Fatalf("StorageKey %q ne odgovara ID-u %q", file.StorageKey, file.ID.Hex())
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
