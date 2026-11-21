package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"files-service/internal/db"
	"files-service/internal/model"
)

// FileRepository - za sada samo citanje (prikaz sadrzaja foldera).
// Otpremanje (1.5) se dodaje ovde: Create, UpdateVersion...
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
