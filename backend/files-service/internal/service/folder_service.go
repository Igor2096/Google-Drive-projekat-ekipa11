// Package service - poslovna logika: validacija, provera vlasnistva, sklapanje odgovora.
package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/v2/bson"

	"files-service/internal/model"
	"files-service/internal/repository"
)

var (
	ErrInvalidName = errors.New("naziv nije validan")
	ErrInvalidID   = errors.New("ID nije validan")
	ErrNotFound    = errors.New("folder ne postoji")
	ErrForbidden   = errors.New("nemate pravo pristupa ovom folderu")
	ErrDuplicate   = errors.New("folder sa tim nazivom vec postoji na ovoj lokaciji")
)

const maxNameLength = 255

// Karakteri koji nisu dozvoljeni u nazivu (isti skup kao na Windows-u).
const forbiddenChars = `/\:*?"<>|`

type folderStore interface {
	Create(ctx context.Context, f *model.Folder) error
	FindByID(ctx context.Context, id bson.ObjectID) (*model.Folder, error)
	FindChildren(ctx context.Context, ownerID string, parentID *bson.ObjectID) ([]model.Folder, error)
	FindByIDs(ctx context.Context, ids []bson.ObjectID) ([]model.Folder, error)
	Rename(ctx context.Context, id bson.ObjectID, name string) error
}

type fileStore interface {
	FindInFolder(ctx context.Context, ownerID string, folderID *bson.ObjectID) ([]model.File, error)
}

type FolderService struct {
	folders folderStore
	files   fileStore
}

func NewFolderService(folders folderStore, files fileStore) *FolderService {
	return &FolderService{folders: folders, files: files}
}

// CreateFolder - T-12: kreiranje foldera u korenu (parentID == "") ili u podfolderu.
func (s *FolderService) CreateFolder(ctx context.Context, ownerID, name, parentID string) (*model.Folder, error) {
	name, err := ValidateName(name)
	if err != nil {
		return nil, err
	}

	folder := &model.Folder{
		Name:      name,
		OwnerID:   ownerID,
		Ancestors: []bson.ObjectID{},
		CreatedAt: time.Now().UTC(),
	}
	folder.UpdatedAt = folder.CreatedAt

	if parentID != "" {
		// T-13: roditelj mora da postoji i da pripada korisniku.
		parent, err := s.getOwnedFolder(ctx, ownerID, parentID)
		if err != nil {
			return nil, err
		}
		folder.ParentID = &parent.ID
		folder.Ancestors = append(append([]bson.ObjectID{}, parent.Ancestors...), parent.ID)
	}

	if err := s.folders.Create(ctx, folder); err != nil {
		return nil, mapRepoErr(err)
	}
	return folder, nil
}

// GetContents - T-20: podfolderi + datoteke izabranog foldera (folderID == "" => koren).
func (s *FolderService) GetContents(ctx context.Context, ownerID, folderID string) (*model.FolderContents, error) {
	result := &model.FolderContents{Breadcrumbs: []model.Breadcrumb{}}
	var parent *bson.ObjectID

	if folderID != "" {
		folder, err := s.getOwnedFolder(ctx, ownerID, folderID)
		if err != nil {
			return nil, err
		}
		result.Folder = folder
		parent = &folder.ID

		crumbs, err := s.breadcrumbs(ctx, folder)
		if err != nil {
			return nil, err
		}
		result.Breadcrumbs = crumbs
	}

	folders, err := s.folders.FindChildren(ctx, ownerID, parent)
	if err != nil {
		return nil, err
	}
	files, err := s.files.FindInFolder(ctx, ownerID, parent)
	if err != nil {
		return nil, err
	}
	result.Folders = folders
	result.Files = files
	return result, nil
}

// RenameFolder - deo zahteva 1.6.
func (s *FolderService) RenameFolder(ctx context.Context, ownerID, folderID, newName string) (*model.Folder, error) {
	newName, err := ValidateName(newName)
	if err != nil {
		return nil, err
	}
	folder, err := s.getOwnedFolder(ctx, ownerID, folderID)
	if err != nil {
		return nil, err
	}
	if err := s.folders.Rename(ctx, folder.ID, newName); err != nil {
		return nil, mapRepoErr(err)
	}
	folder.Name = newName
	folder.UpdatedAt = time.Now().UTC()
	return folder, nil
}

// getOwnedFolder - T-13: centralno mesto za proveru vlasnistva nad folderom.
// Kada se uvede deljenje (1.10), ovde ce se dodati i provera kod Sharing servisa.
func (s *FolderService) getOwnedFolder(ctx context.Context, ownerID, folderID string) (*model.Folder, error) {
	id, err := bson.ObjectIDFromHex(folderID)
	if err != nil {
		return nil, ErrInvalidID
	}
	folder, err := s.folders.FindByID(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	if folder.OwnerID != ownerID {
		return nil, ErrForbidden
	}
	return folder, nil
}

func (s *FolderService) breadcrumbs(ctx context.Context, folder *model.Folder) ([]model.Breadcrumb, error) {
	ancestors, err := s.folders.FindByIDs(ctx, folder.Ancestors)
	if err != nil {
		return nil, err
	}
	byID := make(map[bson.ObjectID]string, len(ancestors))
	for _, a := range ancestors {
		byID[a.ID] = a.Name
	}
	crumbs := make([]model.Breadcrumb, 0, len(folder.Ancestors)+1)
	for _, id := range folder.Ancestors { // redosled iz Ancestors = od korena nanize
		crumbs = append(crumbs, model.Breadcrumb{ID: id.Hex(), Name: byID[id]})
	}
	crumbs = append(crumbs, model.Breadcrumb{ID: folder.ID.Hex(), Name: folder.Name})
	return crumbs, nil
}

// ValidateName - serverska validacija naziva foldera/datoteke.
func ValidateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "", ErrInvalidName
	}
	if utf8.RuneCountInString(name) > maxNameLength {
		return "", ErrInvalidName
	}
	if strings.ContainsAny(name, forbiddenChars) {
		return "", ErrInvalidName
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f { // kontrolni karakteri
			return "", ErrInvalidName
		}
	}
	return name, nil
}

func mapRepoErr(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return ErrNotFound
	case errors.Is(err, repository.ErrDuplicate):
		return ErrDuplicate
	default:
		return err
	}
}
