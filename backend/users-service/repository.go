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

var ErrEmailTaken = errors.New("email je već registrovan")

type UserRepository struct{ collection *mongo.Collection }

func NewUserRepository(collection *mongo.Collection) *UserRepository {
	return &UserRepository{collection: collection}
}

func (r *UserRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true).SetName("unique_email")})
	return err
}

func (r *UserRepository) Create(ctx context.Context, user *User) error {
	result, err := r.collection.InsertOne(ctx, user)
	if mongo.IsDuplicateKeyError(err) {
		return ErrEmailTaken
	}
	if err != nil {
		return err
	}
	id, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return errors.New("MongoDB je vratio neočekivani ID")
	}
	user.ID = id
	return nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	if err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*User, error) {
	var user User
	if err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) SetEmailVerificationToken(ctx context.Context, userID primitive.ObjectID, tokenHash string, expiresAt time.Time) error {
	_, err := r.collection.UpdateByID(ctx, userID, bson.M{"$set": bson.M{
		"email_verification_token_hash": tokenHash,
		"email_verification_expires_at": expiresAt,
		"updated_at":                    time.Now().UTC(),
	}})
	return err
}

func (r *UserRepository) ConfirmEmail(ctx context.Context, userID primitive.ObjectID, tokenHash string) (bool, error) {
	result, err := r.collection.UpdateOne(ctx, bson.M{
		"_id":                           userID,
		"email_verification_token_hash": tokenHash,
		"email_verification_expires_at": bson.M{"$gt": time.Now().UTC()},
	}, bson.M{
		"$set":   bson.M{"verified": true, "updated_at": time.Now().UTC()},
		"$unset": bson.M{"email_verification_token_hash": "", "email_verification_expires_at": ""},
	})
	if err != nil {
		return false, err
	}
	return result.MatchedCount == 1, nil
}

func (r *UserRepository) DeleteByID(ctx context.Context, userID primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": userID})
	return err
}

func (r *UserRepository) SetLoginOTP(ctx context.Context, userID primitive.ObjectID, otpHash string, expiresAt time.Time) error {
	_, err := r.collection.UpdateByID(ctx, userID, bson.M{"$set": bson.M{
		"otp_hash":       otpHash,
		"otp_expires_at": expiresAt,
		"updated_at":     time.Now().UTC(),
	}})
	return err
}

func (r *UserRepository) ConsumeLoginOTP(ctx context.Context, userID primitive.ObjectID, otpHash string) (bool, error) {
	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": userID, "otp_hash": otpHash}, bson.M{
		"$unset": bson.M{"otp_hash": "", "otp_expires_at": ""},
		"$set":   bson.M{"updated_at": time.Now().UTC()},
	})
	if err != nil {
		return false, err
	}
	return result.MatchedCount == 1, nil
}
