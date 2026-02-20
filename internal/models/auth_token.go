package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type AuthToken struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Email     string        `bson:"email" json:"email"`
	Token     string        `bson:"token" json:"token"`
	ExpiresAt time.Time     `bson:"expires_at" json:"expires_at"`
	IsUsed    bool          `bson:"is_used" json:"is_used"`
	CreatedAt time.Time     `bson:"created_at" json:"created_at"`
}

func (t *AuthToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}
