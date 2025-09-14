package service

import (
	"context"
	"time"

	"github.com/Ruscigno/CryptoPulse/pkg/database"
	"go.uber.org/zap"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusDegraded  HealthStatus = "degraded"
)

// ComponentHealth represents the health of a single component
type ComponentHealth struct {
	Name      string       `json:"name"`
	Status    HealthStatus `json:"status"`
	Message   string       `json:"message,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
	Duration  string       `json:"duration,omitempty"`
}

// HealthResponse represents the overall health response
type HealthResponse struct {
	Status     HealthStatus      `json:"status"`
	Timestamp  time.Time         `json:"timestamp"`
	Version    string            `json:"version"`
	Components []ComponentHealth `json:"components"`
	Uptime     string            `json:"uptime"`
}

// HealthService defines the health check service interface
type HealthService interface {
	CheckHealth(ctx context.Context) HealthResponse
	CheckDatabase(ctx context.Context) ComponentHealth
	CheckExternalServices(ctx context.Context) []ComponentHealth
}

// healthService implements the HealthService interface
type healthService struct {
	db        *database.DB
	logger    *zap.Logger
	startTime time.Time
	version   string
}

// NewHealthService creates a new health service
func NewHealthService(db *database.DB, logger *zap.Logger, version string) HealthService {
	return &healthService{
		db:        db,
		logger:    logger,
		startTime: time.Now(),
		version:   version,
	}
}

// CheckHealth performs a comprehensive health check
func (h *healthService) CheckHealth(ctx context.Context) HealthResponse {
	start := time.Now()
	
	// Check all components
	components := []ComponentHealth{}
	
	// Check database
	dbHealth := h.CheckDatabase(ctx)
	components = append(components, dbHealth)
	
	// Check external services
	externalServices := h.CheckExternalServices(ctx)
	components = append(components, externalServices...)
	
	// Determine overall status
	overallStatus := h.determineOverallStatus(components)
	
	response := HealthResponse{
		Status:     overallStatus,
		Timestamp:  time.Now(),
		Version:    h.version,
		Components: components,
		Uptime:     time.Since(h.startTime).String(),
	}
	
	duration := time.Since(start)
	h.logger.Info("Health check completed",
		zap.String("status", string(overallStatus)),
		zap.Duration("duration", duration),
		zap.Int("components", len(components)))
	
	return response
}

// CheckDatabase checks the database health
func (h *healthService) CheckDatabase(ctx context.Context) ComponentHealth {
	start := time.Now()
	
	component := ComponentHealth{
		Name:      "database",
		Timestamp: time.Now(),
	}
	
	if h.db == nil {
		component.Status = HealthStatusUnhealthy
		component.Message = "Database connection not initialized"
		return component
	}
	
	// Check database connectivity
	if err := h.db.Health(ctx); err != nil {
		component.Status = HealthStatusUnhealthy
		component.Message = err.Error()
		h.logger.Error("Database health check failed", zap.Error(err))
		return component
	}
	
	// Check database performance (optional)
	stats := h.db.GetStats()
	if stats.OpenConnections > stats.MaxOpenConnections*8/10 { // 80% threshold
		component.Status = HealthStatusDegraded
		component.Message = "High connection usage"
	} else {
		component.Status = HealthStatusHealthy
		component.Message = "Database is healthy"
	}
	
	component.Duration = time.Since(start).String()
	return component
}

// CheckExternalServices checks the health of external services
func (h *healthService) CheckExternalServices(ctx context.Context) []ComponentHealth {
	components := []ComponentHealth{}
	
	// Check dYdX RPC (placeholder - would implement actual check)
	rpcHealth := ComponentHealth{
		Name:      "dydx_rpc",
		Status:    HealthStatusHealthy, // Placeholder
		Message:   "RPC endpoint reachable",
		Timestamp: time.Now(),
	}
	components = append(components, rpcHealth)
	
	// Check dYdX Indexer (placeholder - would implement actual check)
	indexerHealth := ComponentHealth{
		Name:      "dydx_indexer",
		Status:    HealthStatusHealthy, // Placeholder
		Message:   "Indexer API reachable",
		Timestamp: time.Now(),
	}
	components = append(components, indexerHealth)
	
	return components
}

// determineOverallStatus determines the overall health status based on component statuses
func (h *healthService) determineOverallStatus(components []ComponentHealth) HealthStatus {
	hasUnhealthy := false
	hasDegraded := false
	
	for _, component := range components {
		switch component.Status {
		case HealthStatusUnhealthy:
			hasUnhealthy = true
		case HealthStatusDegraded:
			hasDegraded = true
		}
	}
	
	if hasUnhealthy {
		return HealthStatusUnhealthy
	}
	if hasDegraded {
		return HealthStatusDegraded
	}
	return HealthStatusHealthy
}
