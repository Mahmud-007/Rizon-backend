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

type FeedbackRepo struct {
	collection *mongo.Collection
}

func NewFeedbackRepo() *FeedbackRepo {
	return &FeedbackRepo{
		collection: database.GetCollection("feedbacks"),
	}
}

func (r *FeedbackRepo) Create(ctx context.Context, feedback *models.Feedback) error {
	feedback.CreatedAt = time.Now()
	result, err := r.collection.InsertOne(ctx, feedback)
	if err != nil {
		return err
	}
	feedback.ID = result.InsertedID.(bson.ObjectID)
	return nil
}

// FindByIdempotencyKey checks if feedback with this key already exists (duplicate prevention)
func (r *FeedbackRepo) FindByIdempotencyKey(ctx context.Context, key string) (*models.Feedback, error) {
	var feedback models.Feedback
	err := r.collection.FindOne(ctx, bson.M{"idempotency_key": key}).Decode(&feedback)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &feedback, nil
}

// EnsureIndexes creates necessary indexes for the feedbacks collection
func (r *FeedbackRepo) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "idempotency_key", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
		},
	}
	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}
