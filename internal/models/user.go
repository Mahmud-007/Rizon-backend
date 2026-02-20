package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type User struct {
	ID                  bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Email               string        `bson:"email" json:"email"`
	OnboardingCompleted bool          `bson:"onboarding_completed" json:"onboarding_completed"`
	CreatedAt           time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt           time.Time     `bson:"updated_at" json:"updated_at"`
}
