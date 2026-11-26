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

type FileRepository struct {
	coll    *mongo.Collection
	timeout time.Duration
}

func NewFileRepository(database *mongo.Database, timeout time.Duration) *FileRepository {
	return &FileRepository{coll: database.Collection(db.FilesCollection), timeout: timeout}
}

func (r *FileRepository) FindInFolder(ctx context.Context, ownerID string, folderID *bson.ObjectID) ([]model.File, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	filter := bson.M{"ownerId": ownerID, "folderId": folderID}
	cur, err := r.coll.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	files := []model.File{}
	if err := cur.All(ctx, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// Create upisuje metapodatke o datoteci (T-23).
// ID se postavlja unapred u servisu, jer od njega nastaje StorageKey.
func (r *FileRepository) Create(ctx context.Context, f *model.File) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err := r.coll.InsertOne(ctx, f)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

// FindByName trazi datoteku po nazivu unutar jednog foldera.
// Koristi se da se duplikat otkrije pre nego sto se sadrzaj upise u storage,
// a kasnije i za otpremanje nove verzije (T-25).
func (r *FileRepository) FindByName(ctx context.Context, ownerID string, folderID *bson.ObjectID, name string) (*model.File, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var f model.File
	err := r.coll.FindOne(ctx, bson.M{"ownerId": ownerID, "folderId": folderID, "name": name}).Decode(&f)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// FindByID vraca metapodatke jedne datoteke (T-24).
func (r *FileRepository) FindByID(ctx context.Context, id bson.ObjectID) (*model.File, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var f model.File
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&f)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// UpdateVersion upisuje novu verziju datoteke: novi sadrzaj (storageKey),
// nova velicina i tip, uvecan broj verzije i vreme izmene (T-24).
// Naziv, vlasnik i folder se ne menjaju - to je i dalje ista datoteka.
func (r *FileRepository) UpdateVersion(ctx context.Context, f *model.File) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": f.ID},
		bson.M{"$set": bson.M{
			"storageKey": f.StorageKey,
			"size":       f.Size,
			"mimeType":   f.MimeType,
			"version":    f.Version,
			"updatedAt":  f.UpdatedAt,
		}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}
