package main

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	QuotaRequestPending  = "pending"
	QuotaRequestApproved = "approved"
	QuotaRequestRejected = "rejected"
)

type QuotaRequest struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID         primitive.ObjectID `bson:"user_id" json:"user_id"`
	RequestedQuota int64              `bson:"requested_quota" json:"requested_quota"`
	Status         string             `bson:"status" json:"status"`
	CreatedAt      time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time          `bson:"updated_at" json:"updated_at"`
}
