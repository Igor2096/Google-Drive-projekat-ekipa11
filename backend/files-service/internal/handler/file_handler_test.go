package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"files-service/internal/handler"
	"files-service/internal/middleware"
	"files-service/internal/model"
	"files-service/internal/repository"
	"files-service/internal/service"
	"files-service/internal/users"
)

// --- minimalni fake-ovi, da test ne zahteva Mongo ni disk ---

type folders struct {
	items map[bson.ObjectID]model.Folder
}

func (f *folders) Create(_ context.Context, folder *model.Folder) error {
	folder.ID = bson.NewObjectID()
	f.items[folder.ID] = *folder
	return nil
}

func (f *folders) FindByID(_ context.Context, id bson.ObjectID) (*model.Folder, error) {
	if folder, ok := f.items[id]; ok {
		return &folder, nil
	}
	return nil, repository.ErrNotFound
}

func (f *folders) FindChildren(context.Context, string, *bson.ObjectID) ([]model.Folder, error) {
	return nil, nil
}

func (f *folders) FindByIDs(_ context.Context, _ string, _ []bson.ObjectID) ([]model.Folder, error) {
	return nil, nil
}

func (f *folders) Rename(_ context.Context, _ bson.ObjectID, _ string, _ string) error {
	return nil
}

type files struct {
	items []model.File
}

func (f *files) FindByName(_ context.Context, owner string, folder *bson.ObjectID, name string) (*model.File, error) {
	for i := range f.items {
		if f.items[i].OwnerID == owner && f.items[i].Name == name && f.items[i].FolderID == folder {
			return &f.items[i], nil
		}
	}
	return nil, repository.ErrNotFound
}

func (f *files) Create(_ context.Context, file *model.File) error {
	f.items = append(f.items, *file)
	return nil
}

type memStore struct {
	objects map[string][]byte
}

func (m *memStore) Put(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	b, err := io.ReadAll(r)
	m.objects[key] = b
	return err
}

func (m *memStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, nil
}

func (m *memStore) Delete(_ context.Context, key string) error {
	delete(m.objects, key)
	return nil
}

// quota je fake Users servis: uvek kaze koliko je prostora ostalo.
type quota struct{ available int64 }

func (q quota) Quota(context.Context, string) (users.Quota, error) {
	return users.Quota{StorageQuota: q.available, AvailableStorage: q.available}, nil
}

// newServer sklapa isti lanac kao cmd/main.go: CORS se preskace, ali Auth ostaje.
func newServer(maxUpload int64) (http.Handler, *memStore) {
	return newServerQuota(maxUpload, 1<<30)
}

func newServerQuota(maxUpload, available int64) (http.Handler, *memStore) {
	store := &memStore{objects: map[string][]byte{}}
	svc := service.NewFileService(
		&folders{items: map[bson.ObjectID]model.Folder{}},
		&files{},
		store,
		quota{available: available},
	)

	api := http.NewServeMux()
	handler.NewFileHandler(svc, maxUpload).Register(api)
	return middleware.Auth(api), store
}

// multipartBody pravi telo zahteva kakvo salje browser.
func multipartBody(t *testing.T, fieldName, filename, content, folderID string) (string, io.Reader) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if folderID != "" {
		_ = w.WriteField("folderId", folderID)
	}

	part, err := w.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := io.Copy(part, strings.NewReader(content)); err != nil {
		t.Fatal(err)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	return w.FormDataContentType(), &buf
}

func post(t *testing.T, h http.Handler, userID, fieldName, filename, content string) *httptest.ResponseRecorder {
	t.Helper()

	contentType, body := multipartBody(t, fieldName, filename, content, "")
	req := httptest.NewRequest(http.MethodPost, "/files", body)
	req.Header.Set("Content-Type", contentType)

	if userID != "" {
		req.Header.Set(middleware.UserIDHeader, userID)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// --- testovi ---

func TestUploadSuccess(t *testing.T) {
	h, store := newServer(1 << 20)

	rec := post(t, h, "u1", "file", "beleske.txt", "zdravo svete")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, telo: %s", rec.Code, rec.Body.String())
	}

	var got model.File
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}

	if got.Name != "beleske.txt" || got.Size != 12 || got.Version != 1 {
		t.Fatalf("los odgovor: %+v", got)
	}

	if !strings.HasPrefix(got.MimeType, "text/plain") {
		t.Fatalf("MIME tip: %q", got.MimeType)
	}

	if len(store.objects) != 1 {
		t.Fatal("sadrzaj nije zavrsio u storage-u")
	}

	// StorageKey se ne sme pojaviti u JSON odgovoru
	if strings.Contains(rec.Body.String(), "storageKey") {
		t.Fatal("StorageKey ne sme da curi ka klijentu")
	}
}

func TestUploadWithoutUserIsRejected(t *testing.T) {
	h, _ := newServer(1 << 20)

	if rec := post(t, h, "", "file", "a.txt", "x"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestUploadMissingFileField(t *testing.T) {
	h, _ := newServer(1 << 20)

	if rec := post(t, h, "u1", "dokument", "a.txt", "x"); rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestUploadTooLarge(t *testing.T) {
	h, store := newServer(100) // 100 bajtova

	rec := post(t, h, "u1", "file", "veliko.txt", strings.Repeat("a", 5000))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d, telo: %s", rec.Code, rec.Body.String())
	}

	if len(store.objects) != 0 {
		t.Fatal("prevelik fajl nije smeo da dodje do storage-a")
	}
}

func TestUploadDuplicateReturnsConflict(t *testing.T) {
	h, _ := newServer(1 << 20)

	if rec := post(t, h, "u1", "file", "isto.txt", "prvi"); rec.Code != http.StatusCreated {
		t.Fatalf("status %d", rec.Code)
	}

	if rec := post(t, h, "u1", "file", "isto.txt", "drugi"); rec.Code != http.StatusConflict {
		t.Fatalf("status %d, telo: %s", rec.Code, rec.Body.String())
	}
}

// Browser ponekad salje celu putanju kao naziv; servis mora da je obradi.
func TestUploadStripsPathFromFilename(t *testing.T) {
	h, _ := newServer(1 << 20)

	rec := post(t, h, "u1", "file", `C:\Users\mm\Desktop\slika.png`, "x")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, telo: %s", rec.Code, rec.Body.String())
	}

	var got model.File
	_ = json.Unmarshal(rec.Body.Bytes(), &got)

	if got.Name != "slika.png" {
		t.Fatalf("naziv %q", got.Name)
	}
}

// Kada korisnik nema dovoljno prostora, servis vraca 507 Insufficient Storage.
func TestUploadOverQuotaReturns507(t *testing.T) {
	h, store := newServerQuota(1<<20, 3)

	rec := post(t, h, "u1", "file", "veliko.txt", "mnogo vise od tri bajta")
	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("status %d, telo: %s", rec.Code, rec.Body.String())
	}
	if len(store.objects) != 0 {
		t.Fatal("sadrzaj nije smeo da dodje do storage-a")
	}
}
