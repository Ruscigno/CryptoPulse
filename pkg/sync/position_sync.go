package sync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Ruscigno/CryptoPulse/pkg/query"
	"go.uber.org/zap"
)

// PositionCache represents cached position data
type PositionCache struct {
	Positions map[string]*CachedPosition
	mu        sync.RWMutex
	ttl       time.Duration
	lastSync  time.Time
}

// CachedPosition represents a cached position with metadata
type CachedPosition struct {
	Market        string
	Side          string
	Size          float64
	EntryPrice    float64
	UnrealizedPnl float64
	RealizedPnl   float64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// PositionSyncService handles synchronization of positions from dYdX Indexer API
type PositionSyncService struct {
	queryClient  *query.QueryClient
	cache        *PositionCache
	logger       *zap.Logger
	pollInterval time.Duration
	stopChan     chan struct{}
	ticker       *time.Ticker
}

// NewPositionSyncService creates a new position sync service
func NewPositionSyncService(
	queryClient *query.QueryClient,
	logger *zap.Logger,
	pollInterval time.Duration,
	cacheTTL time.Duration,
) *PositionSyncService {
	return &PositionSyncService{
		queryClient:  queryClient,
		logger:       logger,
		pollInterval: pollInterval,
		stopChan:     make(chan struct{}),
		cache: &PositionCache{
			Positions: make(map[string]*CachedPosition),
			ttl:       cacheTTL,
		},
	}
}

// Start begins the background position synchronization service
func (pss *PositionSyncService) Start(ctx context.Context, address string) error {
	pss.logger.Info("Starting position sync service",
		zap.String("address", address),
		zap.Duration("poll_interval", pss.pollInterval))

	pss.ticker = time.NewTicker(pss.pollInterval)

	go func() {
		// Sync immediately on start
		if err := pss.syncPositions(ctx, address); err != nil {
			pss.logger.Error("Initial position sync failed", zap.Error(err))
		}

		// Then sync on interval
		for {
			select {
			case <-pss.ticker.C:
				if err := pss.syncPositions(ctx, address); err != nil {
					pss.logger.Error("Position sync failed", zap.Error(err))
				}
			case <-pss.stopChan:
				pss.logger.Info("Position sync service stopped")
				return
			case <-ctx.Done():
				pss.logger.Info("Position sync service context cancelled")
				return
			}
		}
	}()

	return nil
}

// Stop stops the background position synchronization service
func (pss *PositionSyncService) Stop() {
	pss.logger.Info("Stopping position sync service")
	if pss.ticker != nil {
		pss.ticker.Stop()
	}
	close(pss.stopChan)
}

// syncPositions synchronizes positions from Indexer API to cache
func (pss *PositionSyncService) syncPositions(ctx context.Context, address string) error {
	pss.logger.Debug("Syncing positions from Indexer API", zap.String("address", address))

	// Query positions from Indexer API
	positionsResp, err := pss.queryClient.GetPositions(ctx, address)
	if err != nil {
		return fmt.Errorf("failed to query positions from Indexer: %w", err)
	}

	if positionsResp == nil || len(positionsResp.Positions) == 0 {
		pss.logger.Debug("No positions found in Indexer API")
		pss.cache.mu.Lock()
		pss.cache.Positions = make(map[string]*CachedPosition)
		pss.cache.lastSync = time.Now()
		pss.cache.mu.Unlock()
		return nil
	}

	// Update cache with new positions
	pss.cache.mu.Lock()
	defer pss.cache.mu.Unlock()

	newPositions := make(map[string]*CachedPosition)
	for _, pos := range positionsResp.Positions {
		cachedPos := &CachedPosition{
			Market:        pos.Market,
			Side:          pos.Side,
			Size:          parseFloat(pos.Size),
			EntryPrice:    parseFloat(pos.EntryPrice),
			UnrealizedPnl: parseFloat(pos.UnrealizedPnl),
			RealizedPnl:   parseFloat(pos.RealizedPnl),
			UpdatedAt:     time.Now(),
		}

		// Parse created at time
		if createdTime, err := time.Parse(time.RFC3339, pos.CreatedAt); err == nil {
			cachedPos.CreatedAt = createdTime
		}

		newPositions[pos.Market] = cachedPos
	}

	pss.cache.Positions = newPositions
	pss.cache.lastSync = time.Now()

	pss.logger.Debug("Position sync completed",
		zap.Int("positions_synced", len(newPositions)))

	return nil
}

// GetPositions returns cached positions
func (pss *PositionSyncService) GetPositions() []*CachedPosition {
	pss.cache.mu.RLock()
	defer pss.cache.mu.RUnlock()

	positions := make([]*CachedPosition, 0, len(pss.cache.Positions))
	for _, pos := range pss.cache.Positions {
		positions = append(positions, pos)
	}

	return positions
}

// GetPosition returns a specific cached position
func (pss *PositionSyncService) GetPosition(market string) *CachedPosition {
	pss.cache.mu.RLock()
	defer pss.cache.mu.RUnlock()

	return pss.cache.Positions[market]
}

// GetTotalUnrealizedPnl returns total unrealized P&L across all positions
func (pss *PositionSyncService) GetTotalUnrealizedPnl() float64 {
	pss.cache.mu.RLock()
	defer pss.cache.mu.RUnlock()

	total := 0.0
	for _, pos := range pss.cache.Positions {
		total += pos.UnrealizedPnl
	}

	return total
}

// GetTotalRealizedPnl returns total realized P&L across all positions
func (pss *PositionSyncService) GetTotalRealizedPnl() float64 {
	pss.cache.mu.RLock()
	defer pss.cache.mu.RUnlock()

	total := 0.0
	for _, pos := range pss.cache.Positions {
		total += pos.RealizedPnl
	}

	return total
}

// IsCacheValid checks if cache is still valid based on TTL
func (pss *PositionSyncService) IsCacheValid() bool {
	pss.cache.mu.RLock()
	defer pss.cache.mu.RUnlock()

	return time.Since(pss.cache.lastSync) < pss.cache.ttl
}

// GetLastSyncTime returns the last sync time
func (pss *PositionSyncService) GetLastSyncTime() time.Time {
	pss.cache.mu.RLock()
	defer pss.cache.mu.RUnlock()

	return pss.cache.lastSync
}

// SyncPositionsOnce performs a single synchronization of positions
func (pss *PositionSyncService) SyncPositionsOnce(ctx context.Context, address string) error {
	return pss.syncPositions(ctx, address)
}

