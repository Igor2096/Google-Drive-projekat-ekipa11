package main

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrQuotaRequestNotFound = errors.New("zahtev za kvotu nije pronađen")

type QuotaRequestRepository struct {
	collection *mongo.Collection
}

func NewQuotaRequestRepository(collection *mongo.Collection) *QuotaRequestRepository {
	return &QuotaRequestRepository{collection: collection}
}

func (r *QuotaRequestRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "status", Value: 1},
		},
		Options: options.Index().SetName("user_status"),
	})
	return err
}

func (r *QuotaRequestRepository) Create(ctx context.Context, request *QuotaRequest) error {
	result, err := r.collection.InsertOne(ctx, request)
	if err != nil {
		return err
	}

	id, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return errors.New("MongoDB je vratio neočekivani ID")
	}

	request.ID = id
	return nil
}

func (r *QuotaRequestRepository) FindAll(ctx context.Context) ([]QuotaRequest, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var requests []QuotaRequest
	if err := cursor.All(ctx, &requests); err != nil {
		return nil, err
	}

	return requests, nil
}

func (r *QuotaRequestRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*QuotaRequest, error) {
	var request QuotaRequest

	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&request)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrQuotaRequestNotFound
	}
	if err != nil {
		return nil, err
	}

	return &request, nil
}

func (r *QuotaRequestRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error {
	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id, "status": QuotaRequestPending},
		bson.M{"$set": bson.M{
			"status":     status,
			"updated_at": time.Now().UTC(),
		}},
	)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return ErrQuotaRequestNotFound
	}

	return nil
}
