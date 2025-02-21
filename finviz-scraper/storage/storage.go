package storage

import (
	"context"
	"strings"
	"time"

	"github.com/Ruscigno/stockscreener/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoStorage struct {
	client *mongo.Client
	db     *mongo.Database
}

func NewMongoStorage(uri, dbName string) (*MongoStorage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	db := client.Database(dbName)
	return &MongoStorage{client: client, db: db}, nil
}

func (s *MongoStorage) Close() {
	s.client.Disconnect(context.Background())
}

func (s *MongoStorage) SaveData(ctx context.Context, data interface{}) error {
	coll := s.db.Collection("scraped_data")
	_, err := coll.InsertOne(ctx, data)
	return err
}

func (s *MongoStorage) GetRule(ctx context.Context, id string) (*models.Rule, error) {
	coll := s.db.Collection("rules")
	var rule models.Rule
	err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(&rule)
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (s *MongoStorage) SaveRule(ctx context.Context, rule *models.Rule) error {
	coll := s.db.Collection("rules")
	_, err := coll.InsertOne(ctx, rule)
	return err
}

func (s *MongoStorage) UpdateRule(ctx context.Context, rule *models.Rule) error {
	coll := s.db.Collection("rules")
	_, err := coll.ReplaceOne(ctx, bson.M{"_id": rule.ID}, rule)
	return err
}

func (s *MongoStorage) DeleteRule(ctx context.Context, id string) error {
	coll := s.db.Collection("rules")
	_, err := coll.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (s *MongoStorage) SaveJob(ctx context.Context, job *models.ScrapeJob) error {
	coll := s.db.Collection("jobs")
	_, err := coll.InsertOne(ctx, job)
	// if it exists, it will be replaced
	if strings.Contains(err.Error(), "duplicate key error collection") {
		_, err = coll.ReplaceOne(ctx, bson.M{"_id": job.ID}, job)
	}
	return err
}
