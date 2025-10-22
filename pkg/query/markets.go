package query

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// MarketMetadata contains market configuration from dYdX
type MarketMetadata struct {
	ID                 uint32
	Pair               string
	AtomicResolution   int32
	SubticksPerTick    int32
	StepBaseQuantums   uint64
	MinPriceChange     uint64
	SmallestTickSize   float64
	CacheTime          time.Time
}

// MarketCache manages market metadata with TTL
type MarketCache struct {
	markets    map[string]*MarketMetadata
	mu         sync.RWMutex
	ttl        time.Duration
	lastUpdate time.Time
	logger     *zap.Logger
	client     *QueryClient
}

// NewMarketCache creates a new market cache
func NewMarketCache(client *QueryClient, ttl time.Duration, logger *zap.Logger) *MarketCache {
	return &MarketCache{
		markets: make(map[string]*MarketMetadata),
		ttl:     ttl,
		logger:  logger,
		client:  client,
	}
}

// GetMarket retrieves market metadata from cache or fetches from API
func (mc *MarketCache) GetMarket(ctx context.Context, market string) (*MarketMetadata, error) {
	mc.mu.RLock()
	cached, exists := mc.markets[market]
	cacheValid := time.Since(mc.lastUpdate) < mc.ttl
	mc.mu.RUnlock()

	if exists && cacheValid {
		mc.logger.Debug("Market found in cache", zap.String("market", market))
		return cached, nil
	}

	// Fetch from API
	mc.logger.Debug("Fetching market from API", zap.String("market", market))
	metadata, err := mc.fetchMarketFromAPI(ctx, market)
	if err != nil {
		return nil, err
	}

	// Update cache
	mc.mu.Lock()
	mc.markets[market] = metadata
	mc.lastUpdate = time.Now()
	mc.mu.Unlock()

	return metadata, nil
}

// RefreshAll refreshes all markets in cache
func (mc *MarketCache) RefreshAll(ctx context.Context) error {
	mc.logger.Debug("Refreshing all markets in cache")

	// Fetch all markets from API
	markets, err := mc.fetchAllMarketsFromAPI(ctx)
	if err != nil {
		return err
	}

	// Update cache
	mc.mu.Lock()
	mc.markets = markets
	mc.lastUpdate = time.Now()
	mc.mu.Unlock()

	mc.logger.Info("Markets cache refreshed", zap.Int("market_count", len(markets)))
	return nil
}

// fetchMarketFromAPI fetches a single market from the Indexer API
func (mc *MarketCache) fetchMarketFromAPI(ctx context.Context, market string) (*MarketMetadata, error) {
	// This would call the Indexer API endpoint
	// For now, return hardcoded values for common markets
	// In production, this would make an HTTP request to the Indexer API

	mc.logger.Debug("Fetching market from Indexer API", zap.String("market", market))

	// Hardcoded market configurations for testing
	// In production, these would come from the API
	markets := map[string]*MarketMetadata{
		"BTC-USD": {
			ID:               0,
			Pair:             "BTC-USD",
			AtomicResolution: -8,
			SubticksPerTick:  1000000,
			StepBaseQuantums: 1000000,
			MinPriceChange:   1000000,
			SmallestTickSize: 0.000001,
			CacheTime:        time.Now(),
		},
		"ETH-USD": {
			ID:               1,
			Pair:             "ETH-USD",
			AtomicResolution: -18,
			SubticksPerTick:  1000000,
			StepBaseQuantums: 1000000000000000000,
			MinPriceChange:   1000000,
			SmallestTickSize: 0.000001,
			CacheTime:        time.Now(),
		},
		"SOL-USD": {
			ID:               2,
			Pair:             "SOL-USD",
			AtomicResolution: -9,
			SubticksPerTick:  1000000,
			StepBaseQuantums: 1000000000,
			MinPriceChange:   1000000,
			SmallestTickSize: 0.000001,
			CacheTime:        time.Now(),
		},
	}

	if metadata, exists := markets[market]; exists {
		mc.logger.Debug("Market found in hardcoded config",
			zap.String("market", market),
			zap.Int32("atomic_resolution", metadata.AtomicResolution))
		return metadata, nil
	}

	return nil, fmt.Errorf("market not found: %s", market)
}

// fetchAllMarketsFromAPI fetches all markets from the Indexer API
func (mc *MarketCache) fetchAllMarketsFromAPI(ctx context.Context) (map[string]*MarketMetadata, error) {
	mc.logger.Debug("Fetching all markets from Indexer API")

	// Hardcoded markets for testing
	// In production, this would make an HTTP request to the Indexer API
	markets := map[string]*MarketMetadata{
		"BTC-USD": {
			ID:               0,
			Pair:             "BTC-USD",
			AtomicResolution: -8,
			SubticksPerTick:  1000000,
			StepBaseQuantums: 1000000,
			MinPriceChange:   1000000,
			SmallestTickSize: 0.000001,
			CacheTime:        time.Now(),
		},
		"ETH-USD": {
			ID:               1,
			Pair:             "ETH-USD",
			AtomicResolution: -18,
			SubticksPerTick:  1000000,
			StepBaseQuantums: 1000000000000000000,
			MinPriceChange:   1000000,
			SmallestTickSize: 0.000001,
			CacheTime:        time.Now(),
		},
		"SOL-USD": {
			ID:               2,
			Pair:             "SOL-USD",
			AtomicResolution: -9,
			SubticksPerTick:  1000000,
			StepBaseQuantums: 1000000000,
			MinPriceChange:   1000000,
			SmallestTickSize: 0.000001,
			CacheTime:        time.Now(),
		},
	}

	mc.logger.Info("All markets fetched from Indexer API", zap.Int("market_count", len(markets)))
	return markets, nil
}

// ClearCache clears the market cache
func (mc *MarketCache) ClearCache() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.markets = make(map[string]*MarketMetadata)
	mc.logger.Debug("Market cache cleared")
}

// GetCachedMarkets returns all cached markets
func (mc *MarketCache) GetCachedMarkets() map[string]*MarketMetadata {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// Return a copy to prevent external modifications
	result := make(map[string]*MarketMetadata)
	for k, v := range mc.markets {
		result[k] = v
	}
	return result
}

// IsCacheValid checks if the cache is still valid
func (mc *MarketCache) IsCacheValid() bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return time.Since(mc.lastUpdate) < mc.ttl
}

// GetCacheAge returns the age of the cache
func (mc *MarketCache) GetCacheAge() time.Duration {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return time.Since(mc.lastUpdate)
}

// MarshalJSON implements json.Marshaler for MarketMetadata
func (m *MarketMetadata) MarshalJSON() ([]byte, error) {
	type Alias MarketMetadata
	return json.Marshal(&struct {
		*Alias
		CacheTime string `json:"cache_time"`
	}{
		Alias:     (*Alias)(m),
		CacheTime: m.CacheTime.Format(time.RFC3339),
	})
}

