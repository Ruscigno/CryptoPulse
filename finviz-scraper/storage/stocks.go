package storage

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (s *MongoStorage) SaveStocks(ctx context.Context, stocks []map[string]string) error {
	coll := s.db.Collection("scraped_data")
	now := time.Now().UTC().Format("2006-01-02") // Daily granularity; adjust if needed

	for _, stock := range stocks {
		stock["timestamp"] = now

		// Define the filter for upsert (ticker + timestamp as composite key)
		filter := bson.M{
			"ticker":    stock["ticker"],
			"timestamp": stock["timestamp"],
		}

		// Upsert: replace the document if it exists, insert if not
		opts := options.Replace().SetUpsert(true)
		_, err := coll.ReplaceOne(ctx, filter, stock, opts)
		if err != nil {
			return fmt.Errorf("failed to upsert stock data for ticker %s: %v", stock["ticker"], err)
		}
	}
	return nil
}
