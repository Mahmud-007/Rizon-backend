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

type UserRepo struct {
	collection *mongo.Collection
}

func NewUserRepo() *UserRepo {
	return &UserRepo{
		collection: database.GetCollection("users"),
	}
}

func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) FindByID(ctx context.Context, id bson.ObjectID) (*models.User, error) {
	var user models.User
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) Create(ctx context.Context, user *models.User) error {
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	result, err := r.collection.InsertOne(ctx, user)
	if err != nil {
		return err
	}
	user.ID = result.InsertedID.(bson.ObjectID)
	return nil
}

func (r *UserRepo) FindOrCreate(ctx context.Context, email string) (*models.User, error) {
	user, err := r.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user != nil {
		return user, nil
	}

	newUser := &models.User{
		Email:               email,
		OnboardingCompleted: false,
	}
	if err := r.Create(ctx, newUser); err != nil {
		return nil, err
	}
	return newUser, nil
}

func (r *UserRepo) UpdateOnboarding(ctx context.Context, id bson.ObjectID, completed bool) error {
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{
			"onboarding_completed": completed,
			"updated_at":           time.Now(),
		},
	})
	return err
}

// EnsureIndexes creates necessary indexes for the users collection
func (r *UserRepo) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}
