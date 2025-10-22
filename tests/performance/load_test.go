package performance

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ruscigno/CryptoPulse/pkg/service"
	"go.uber.org/zap"
)

// LoadTestConfig holds configuration for load testing
type LoadTestConfig struct {
	OrdersPerSecond int
	DurationSeconds int
	ConcurrentUsers int
	Logger          *zap.Logger
}

// LoadTestResult holds the results of a load test
type LoadTestResult struct {
	TotalOrders      int64
	SuccessfulOrders int64
	FailedOrders     int64
	TotalDuration    time.Duration
	MinLatency       time.Duration
	MaxLatency       time.Duration
	AvgLatency       time.Duration
	P95Latency       time.Duration
	P99Latency       time.Duration
	OrdersPerSecond  float64
	Errors           []string
}

// LoadTester performs load testing on the service
type LoadTester struct {
	svc    service.Service
	config LoadTestConfig
	logger *zap.Logger
}

// NewLoadTester creates a new load tester
func NewLoadTester(svc service.Service, config LoadTestConfig) *LoadTester {
	return &LoadTester{
		svc:    svc,
		config: config,
		logger: config.Logger,
	}
}

// RunLoadTest runs a load test on the service
func (lt *LoadTester) RunLoadTest(ctx context.Context, t *testing.T) *LoadTestResult {
	lt.logger.Info("Starting load test",
		zap.Int("orders_per_second", lt.config.OrdersPerSecond),
		zap.Int("duration_seconds", lt.config.DurationSeconds),
		zap.Int("concurrent_users", lt.config.ConcurrentUsers))

	result := &LoadTestResult{
		Errors: make([]string, 0),
	}

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(ctx, time.Duration(lt.config.DurationSeconds)*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var totalOrders int64
	var successfulOrders int64
	var failedOrders int64
	var minLatency int64 = 1<<63 - 1
	var maxLatency int64
	latencies := make([]time.Duration, 0)
	var latenciesMu sync.Mutex

	// Start concurrent users
	for i := 0; i < lt.config.ConcurrentUsers; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			lt.runUserLoadTest(ctx, userID, &totalOrders, &successfulOrders, &failedOrders,
				&minLatency, &maxLatency, &latencies, &latenciesMu, result)
		}(i)
	}

	wg.Wait()
	result.TotalDuration = time.Since(startTime)

	// Calculate statistics
	result.TotalOrders = atomic.LoadInt64(&totalOrders)
	result.SuccessfulOrders = atomic.LoadInt64(&successfulOrders)
	result.FailedOrders = atomic.LoadInt64(&failedOrders)
	result.OrdersPerSecond = float64(result.SuccessfulOrders) / result.TotalDuration.Seconds()

	if minLatency < 1<<63-1 {
		result.MinLatency = time.Duration(atomic.LoadInt64(&minLatency))
	}
	result.MaxLatency = time.Duration(atomic.LoadInt64(&maxLatency))

	// Calculate average latency
	if len(latencies) > 0 {
		var totalLatency time.Duration
		for _, lat := range latencies {
			totalLatency += lat
		}
		result.AvgLatency = totalLatency / time.Duration(len(latencies))

		// Calculate percentiles
		result.P95Latency = calculatePercentile(latencies, 95)
		result.P99Latency = calculatePercentile(latencies, 99)
	}

	lt.logger.Info("Load test completed",
		zap.Int64("total_orders", result.TotalOrders),
		zap.Int64("successful_orders", result.SuccessfulOrders),
		zap.Int64("failed_orders", result.FailedOrders),
		zap.Duration("total_duration", result.TotalDuration),
		zap.Float64("orders_per_second", result.OrdersPerSecond),
		zap.Duration("avg_latency", result.AvgLatency),
		zap.Duration("p95_latency", result.P95Latency),
		zap.Duration("p99_latency", result.P99Latency))

	return result
}

// runUserLoadTest runs load test for a single user
func (lt *LoadTester) runUserLoadTest(
	ctx context.Context,
	userID int,
	totalOrders, successfulOrders, failedOrders *int64,
	minLatency, maxLatency *int64,
	latencies *[]time.Duration,
	latenciesMu *sync.Mutex,
	result *LoadTestResult,
) {
	ticker := time.NewTicker(time.Second / time.Duration(lt.config.OrdersPerSecond/lt.config.ConcurrentUsers))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			atomic.AddInt64(totalOrders, 1)

			// Create order request
			req := service.OrderRequest{
				Market:      "BTC-USD",
				Side:        "BUY",
				Type:        "LIMIT",
				Size:        0.001,
				Price:       func() *float64 { p := 50000.0; return &p }(),
				TimeInForce: "GTT",
				GoodTilBlock: func() *uint32 { b := uint32(1000); return &b }(),
			}

			// Measure latency
			startTime := time.Now()
			_, err := lt.svc.PlaceOrder(ctx, req)
			latency := time.Since(startTime)

			latenciesMu.Lock()
			*latencies = append(*latencies, latency)
			latenciesMu.Unlock()

			// Update min/max latency
			latencyNs := latency.Nanoseconds()
			for {
				currentMin := atomic.LoadInt64(minLatency)
				if latencyNs >= currentMin || atomic.CompareAndSwapInt64(minLatency, currentMin, latencyNs) {
					break
				}
			}

			for {
				currentMax := atomic.LoadInt64(maxLatency)
				if latencyNs <= currentMax || atomic.CompareAndSwapInt64(maxLatency, currentMax, latencyNs) {
					break
				}
			}

			if err != nil {
				atomic.AddInt64(failedOrders, 1)
				result.Errors = append(result.Errors, fmt.Sprintf("User %d: %v", userID, err))
			} else {
				atomic.AddInt64(successfulOrders, 1)
			}

		case <-ctx.Done():
			return
		}
	}
}

// calculatePercentile calculates the percentile of latencies
func calculatePercentile(latencies []time.Duration, percentile int) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	// Simple percentile calculation (not exact, but good enough for testing)
	index := (len(latencies) * percentile) / 100
	if index >= len(latencies) {
		index = len(latencies) - 1
	}

	return latencies[index]
}

// PrintResults prints the load test results
func (result *LoadTestResult) PrintResults() {
	fmt.Printf("\n=== Load Test Results ===\n")
	fmt.Printf("Total Orders: %d\n", result.TotalOrders)
	fmt.Printf("Successful Orders: %d\n", result.SuccessfulOrders)
	fmt.Printf("Failed Orders: %d\n", result.FailedOrders)
	fmt.Printf("Success Rate: %.2f%%\n", float64(result.SuccessfulOrders)/float64(result.TotalOrders)*100)
	fmt.Printf("Total Duration: %v\n", result.TotalDuration)
	fmt.Printf("Orders Per Second: %.2f\n", result.OrdersPerSecond)
	fmt.Printf("Min Latency: %v\n", result.MinLatency)
	fmt.Printf("Max Latency: %v\n", result.MaxLatency)
	fmt.Printf("Avg Latency: %v\n", result.AvgLatency)
	fmt.Printf("P95 Latency: %v\n", result.P95Latency)
	fmt.Printf("P99 Latency: %v\n", result.P99Latency)

	if len(result.Errors) > 0 {
		fmt.Printf("\nErrors (first 10):\n")
		for i, err := range result.Errors {
			if i >= 10 {
				break
			}
			fmt.Printf("  - %s\n", err)
		}
	}
}

