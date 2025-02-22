package storage

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (s *MongoStorage) SaveStocks(ctx context.Context, stocks []map[string]string) error {
	coll := s.db.Collection("scraped_data")
	now := time.Now().UTC().Format("2006-01-02") // Daily granularity; adjust if needed

	for _, stock := range stocks {
		stock["timestamp"] = now

		// Define the filter for matching (ticker + timestamp as composite key)
		filter := bson.M{
			"ticker":    stock["ticker"],
			"timestamp": stock["timestamp"],
		}

		// Fetch existing document, if it exists
		var existing map[string]interface{}
		err := coll.FindOne(ctx, filter).Decode(&existing)
		if err != nil && err != mongo.ErrNoDocuments {
			return fmt.Errorf("failed to fetch existing stock data for ticker %s: %v", stock["ticker"], err)
		}

		// Merge existing data with new stock data
		merged := make(map[string]interface{})
		if err == nil { // If document exists, copy existing fields
			for k, v := range existing {
				if k != "_id" { // Skip MongoDB's internal ID
					merged[k] = v
				}
			}
		}
		// Add or update with new fields
		for k, v := range stock {
			merged[k] = v
		}

		// Upsert the merged document
		opts := options.Replace().SetUpsert(true)
		_, err = coll.ReplaceOne(ctx, filter, merged, opts)
		if err != nil {
			return fmt.Errorf("failed to upsert merged stock data for ticker %s: %v", stock["ticker"], err)
		}
	}
	return nil
}
