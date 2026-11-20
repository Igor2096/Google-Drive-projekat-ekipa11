// Package db - konekcija na MongoDB (dokument baza) i kreiranje indeksa (T-11).
package db

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	FoldersCollection = "folders"
	FilesCollection   = "files"
)

// Connect otvara konekciju i proverava je ping-om.
// Ponavlja pokusaj nekoliko puta jer u Docker Compose-u Mongo cesto
// nije spreman u trenutku kada se servis podigne.
func Connect(uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}

	var pingErr error
	for attempt := 1; attempt <= 10; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		pingErr = client.Ping(ctx, nil)
		cancel()
		if pingErr == nil {
			return client, nil
		}
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("mongo ping: %w", pingErr)
}

// EnsureIndexes kreira indekse koji ubrzavaju najcesce upite
// i sprecavaju dva foldera/fajla sa istim imenom u istom roditelju.
func EnsureIndexes(ctx context.Context, database *mongo.Database) error {
	folders := database.Collection(FoldersCollection)
	_, err := folders.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "ownerId", Value: 1}, {Key: "parentId", Value: 1}, {Key: "name", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_owner_parent_name"),
		},
		{Keys: bson.D{{Key: "ancestors", Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("folders indexes: %w", err)
	}

	files := database.Collection(FilesCollection)
	_, err = files.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "ownerId", Value: 1}, {Key: "folderId", Value: 1}, {Key: "name", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_owner_folder_name"),
		},
	})
	if err != nil {
		return fmt.Errorf("files indexes: %w", err)
	}
	return nil
}
