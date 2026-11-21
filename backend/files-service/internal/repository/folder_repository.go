// Package repository - sav pristup MongoDB-u je ovde, ostatak servisa ne zna za bazu.
package repository

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"files-service/internal/db"
	"files-service/internal/model"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrDuplicate = errors.New("duplicate name")
)

type FolderRepository struct {
	coll    *mongo.Collection
	timeout time.Duration
}

func NewFolderRepository(database *mongo.Database, timeout time.Duration) *FolderRepository {
	return &FolderRepository{coll: database.Collection(db.FoldersCollection), timeout: timeout}
}

func (r *FolderRepository) Create(ctx context.Context, f *model.Folder) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	res, err := r.coll.InsertOne(ctx, f)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrDuplicate
		}
		return err
	}
	f.ID = res.InsertedID.(bson.ObjectID)
	return nil
}

func (r *FolderRepository) FindByID(ctx context.Context, id bson.ObjectID) (*model.Folder, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var f model.Folder
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&f)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// FindChildren vraca podfoldere datog roditelja (parentID == nil => koren).
func (r *FolderRepository) FindChildren(ctx context.Context, ownerID string, parentID *bson.ObjectID) ([]model.Folder, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// U Mongo-u {parentId: null} pogadja i dokumente gde je polje null
	// i one gde ne postoji - sto je upravo "koren".
	filter := bson.M{"ownerId": ownerID, "parentId": parentID}
	cur, err := r.coll.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	folders := []model.Folder{}
	if err := cur.All(ctx, &folders); err != nil {
		return nil, err
	}
	return folders, nil
}

// FindByIDs vraca foldere sa datim ID-jevima (koristi se za breadcrumbs).
func (r *FolderRepository) FindByIDs(ctx context.Context, ids []bson.ObjectID) ([]model.Folder, error) {
	if len(ids) == 0 {
		return []model.Folder{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	cur, err := r.coll.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	folders := []model.Folder{}
	if err := cur.All(ctx, &folders); err != nil {
		return nil, err
	}
	return folders, nil
}

func (r *FolderRepository) Rename(ctx context.Context, id bson.ObjectID, name string) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"name": name, "updatedAt": time.Now().UTC()}},
	)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrDuplicate
		}
		return err
	}
	if res.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}
