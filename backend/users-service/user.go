package main

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const regularUserRole = "user"

type User struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	FirstName    string             `bson:"first_name" json:"first_name"`
	LastName     string             `bson:"last_name" json:"last_name"`
	Email        string             `bson:"email" json:"email"`
	PasswordHash string             `bson:"password_hash" json:"-"`
	Role         string             `bson:"role" json:"role"`
	StorageQuota int64              `bson:"storage_quota" json:"storage_quota"`
	UsedStorage  int64              `bson:"used_storage" json:"used_storage"`
	Verified     bool               `bson:"verified" json:"verified"`
	OTPHash      string             `bson:"otp_hash,omitempty" json:"-"`
	OTPExpiresAt *time.Time         `bson:"otp_expires_at,omitempty" json:"-"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at" json:"updated_at"`
}
