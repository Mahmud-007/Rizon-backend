package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Feedback struct {
	ID             bson.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID         bson.ObjectID `bson:"user_id" json:"user_id"`
	Text           string        `bson:"text" json:"text"`
	Rating         int           `bson:"rating" json:"rating"`
	IdempotencyKey string        `bson:"idempotency_key" json:"idempotency_key"`
	CreatedAt      time.Time     `bson:"created_at" json:"created_at"`
}
