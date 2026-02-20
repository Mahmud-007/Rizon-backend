package repository

import (
	"context"
	"time"

	"rizon-backend/internal/database"
	"rizon-backend/internal/models"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type AuthTokenRepo struct {
	collection *mongo.Collection
}

func NewAuthTokenRepo() *AuthTokenRepo {
	return &AuthTokenRepo{
		collection: database.GetCollection("auth_tokens"),
	}
}

func (r *AuthTokenRepo) Create(ctx context.Context, token *models.AuthToken) error {
	token.CreatedAt = time.Now()
	result, err := r.collection.InsertOne(ctx, token)
	if err != nil {
		return err
	}
	token.ID = result.InsertedID.(bson.ObjectID)
	return nil
}

func (r *AuthTokenRepo) FindByToken(ctx context.Context, token string) (*models.AuthToken, error) {
	var authToken models.AuthToken
	err := r.collection.FindOne(ctx, bson.M{"token": token}).Decode(&authToken)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &authToken, nil
}

func (r *AuthTokenRepo) MarkUsed(ctx context.Context, token string) error {
	_, err := r.collection.UpdateOne(ctx, bson.M{"token": token}, bson.M{
		"$set": bson.M{"is_used": true},
	})
	return err
}

// CountRecentByEmail counts how many tokens were created for an email in the given duration.
// Used for rate limiting.
func (r *AuthTokenRepo) CountRecentByEmail(ctx context.Context, email string, duration time.Duration) (int64, error) {
	since := time.Now().Add(-duration)
	count, err := r.collection.CountDocuments(ctx, bson.M{
		"email":      email,
		"created_at": bson.M{"$gte": since},
	})
	return count, err
}

// EnsureIndexes creates necessary indexes for the auth_tokens collection
func (r *AuthTokenRepo) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "token", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "email", Value: 1}, {Key: "created_at", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0), // TTL index — auto-delete expired tokens
		},
	}
	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}
