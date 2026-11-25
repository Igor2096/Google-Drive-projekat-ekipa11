package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"files-service/internal/model"
	"files-service/internal/repository"
)

// --- fake repozitorijumi u memoriji, da testovi ne zahtevaju Mongo ---

type fakeFolders struct {
	items map[bson.ObjectID]model.Folder
}

func (f *fakeFolders) Create(_ context.Context, folder *model.Folder) error {
	for _, ex := range f.items {
		if ex.OwnerID == folder.OwnerID && ex.Name == folder.Name && sameParent(ex.ParentID, folder.ParentID) {
			return repository.ErrDuplicate
		}
	}
	folder.ID = bson.NewObjectID()
	f.items[folder.ID] = *folder
	return nil
}

func (f *fakeFolders) FindByID(_ context.Context, id bson.ObjectID) (*model.Folder, error) {
	folder, ok := f.items[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &folder, nil
}

func (f *fakeFolders) FindChildren(_ context.Context, owner string, parent *bson.ObjectID) ([]model.Folder, error) {
	out := []model.Folder{}
	for _, ex := range f.items {
		if ex.OwnerID == owner && sameParent(ex.ParentID, parent) {
			out = append(out, ex)
		}
	}
	return out, nil
}

func (f *fakeFolders) FindByIDs(_ context.Context, ownerID string, ids []bson.ObjectID) ([]model.Folder, error) {
	out := []model.Folder{}
	for _, id := range ids {
		if ex, ok := f.items[id]; ok && ex.OwnerID == ownerID {
			out = append(out, ex)
		}
	}
	return out, nil
}

func (f *fakeFolders) Rename(_ context.Context, id bson.ObjectID, ownerID string, name string) error {
	ex, ok := f.items[id]
	if !ok || ex.OwnerID != ownerID {
		return repository.ErrNotFound
	}
	ex.Name = name
	f.items[id] = ex
	return nil
}

type fakeFiles struct{}

func (fakeFiles) FindInFolder(context.Context, string, *bson.ObjectID) ([]model.File, error) {
	return []model.File{}, nil
}

func sameParent(a, b *bson.ObjectID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func newSvc() *FolderService {
	return NewFolderService(&fakeFolders{items: map[bson.ObjectID]model.Folder{}}, fakeFiles{})
}

// --- testovi ---

func TestCreateInRootAndSubfolder(t *testing.T) {
	svc, ctx := newSvc(), context.Background()

	a, err := svc.CreateFolder(ctx, "u1", "  Skola  ", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "Skola" || a.ParentID != nil || len(a.Ancestors) != 0 {
		t.Fatalf("los folder u korenu: %+v", a)
	}

	b, err := svc.CreateFolder(ctx, "u1", "SOA", a.ID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	c, err := svc.CreateFolder(ctx, "u1", "Projekat", b.ID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Ancestors) != 2 || c.Ancestors[0] != a.ID || c.Ancestors[1] != b.ID {
		t.Fatalf("losi ancestors: %v", c.Ancestors)
	}

	contents, err := svc.GetContents(ctx, "u1", c.ID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Skola", "SOA", "Projekat"}
	if len(contents.Breadcrumbs) != len(want) {
		t.Fatalf("breadcrumbs: %+v", contents.Breadcrumbs)
	}
	for i, w := range want {
		if contents.Breadcrumbs[i].Name != w {
			t.Fatalf("breadcrumb %d = %q, ocekivano %q", i, contents.Breadcrumbs[i].Name, w)
		}
	}
}

func TestParentOwnership(t *testing.T) {
	svc, ctx := newSvc(), context.Background()
	a, _ := svc.CreateFolder(ctx, "u1", "Privatno", "")

	if _, err := svc.CreateFolder(ctx, "u2", "Upad", a.ID.Hex()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ocekivano ErrForbidden, dobijeno %v", err)
	}
	if _, err := svc.GetContents(ctx, "u2", a.ID.Hex()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ocekivano ErrForbidden, dobijeno %v", err)
	}
	if _, err := svc.CreateFolder(ctx, "u1", "X", bson.NewObjectID().Hex()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ocekivano ErrNotFound, dobijeno %v", err)
	}
	if _, err := svc.CreateFolder(ctx, "u1", "X", "nije-id"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("ocekivano ErrInvalidID, dobijeno %v", err)
	}
}

func TestDuplicateName(t *testing.T) {
	svc, ctx := newSvc(), context.Background()
	_, _ = svc.CreateFolder(ctx, "u1", "Isto", "")
	if _, err := svc.CreateFolder(ctx, "u1", "Isto", ""); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("ocekivano ErrDuplicate, dobijeno %v", err)
	}
	// drugi korisnik sme da ima isti naziv
	if _, err := svc.CreateFolder(ctx, "u2", "Isto", ""); err != nil {
		t.Fatal(err)
	}
}

func TestValidateName(t *testing.T) {
	bad := []string{"", "   ", ".", "..", "a/b", `a\b`, "a:b", "a*b", "a?b", `a"b`, "a<b", "a>b", "a|b", "a\nb", strings.Repeat("a", 256)}
	for _, n := range bad {
		if _, err := ValidateName(n); err == nil {
			t.Errorf("naziv %q je trebalo da bude odbijen", n)
		}
	}
	good := []string{"Skola", "Projekat 2026", "Ćirilica i šđžčć", "fajl.v2"}
	for _, n := range good {
		if _, err := ValidateName(n); err != nil {
			t.Errorf("naziv %q je trebalo da prodje: %v", n, err)
		}
	}
}
