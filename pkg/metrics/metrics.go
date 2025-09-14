package metrics

import (
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"
)

// MetricsCollector interface for collecting metrics
type MetricsCollector interface {
	IncrementCounter(name string, labels map[string]string)
	RecordHistogram(name string, value float64, labels map[string]string)
	SetGauge(name string, value float64, labels map[string]string)
	RecordDuration(name string, duration time.Duration, labels map[string]string)
}

// SimpleMetricsCollector is a basic in-memory metrics collector
type SimpleMetricsCollector struct {
	counters   map[string]float64
	histograms map[string][]float64
	gauges     map[string]float64
	logger     *zap.Logger
}

// NewSimpleMetricsCollector creates a new simple metrics collector
func NewSimpleMetricsCollector(logger *zap.Logger) *SimpleMetricsCollector {
	return &SimpleMetricsCollector{
		counters:   make(map[string]float64),
		histograms: make(map[string][]float64),
		gauges:     make(map[string]float64),
		logger:     logger,
	}
}

// IncrementCounter increments a counter metric
func (smc *SimpleMetricsCollector) IncrementCounter(name string, labels map[string]string) {
	key := buildMetricKey(name, labels)
	smc.counters[key]++
	
	smc.logger.Debug("Counter incremented",
		zap.String("metric", name),
		zap.Any("labels", labels),
		zap.Float64("value", smc.counters[key]))
}

// RecordHistogram records a histogram value
func (smc *SimpleMetricsCollector) RecordHistogram(name string, value float64, labels map[string]string) {
	key := buildMetricKey(name, labels)
	if smc.histograms[key] == nil {
		smc.histograms[key] = make([]float64, 0)
	}
	smc.histograms[key] = append(smc.histograms[key], value)
	
	smc.logger.Debug("Histogram recorded",
		zap.String("metric", name),
		zap.Any("labels", labels),
		zap.Float64("value", value))
}

// SetGauge sets a gauge metric value
func (smc *SimpleMetricsCollector) SetGauge(name string, value float64, labels map[string]string) {
	key := buildMetricKey(name, labels)
	smc.gauges[key] = value
	
	smc.logger.Debug("Gauge set",
		zap.String("metric", name),
		zap.Any("labels", labels),
		zap.Float64("value", value))
}

// RecordDuration records a duration metric
func (smc *SimpleMetricsCollector) RecordDuration(name string, duration time.Duration, labels map[string]string) {
	smc.RecordHistogram(name+"_duration_seconds", duration.Seconds(), labels)
}

// buildMetricKey builds a unique key for a metric with labels
func buildMetricKey(name string, labels map[string]string) string {
	key := name
	for k, v := range labels {
		key += "_" + k + "_" + v
	}
	return key
}

// ApplicationMetrics holds all application-specific metrics
type ApplicationMetrics struct {
	collector MetricsCollector
	logger    *zap.Logger
}

// NewApplicationMetrics creates a new application metrics instance
func NewApplicationMetrics(collector MetricsCollector, logger *zap.Logger) *ApplicationMetrics {
	return &ApplicationMetrics{
		collector: collector,
		logger:    logger,
	}
}

// HTTP Metrics
func (am *ApplicationMetrics) RecordHTTPRequest(method, path string, statusCode int, duration time.Duration) {
	labels := map[string]string{
		"method": method,
		"path":   path,
		"status": strconv.Itoa(statusCode),
	}
	
	am.collector.IncrementCounter("http_requests_total", labels)
	am.collector.RecordDuration("http_request_duration", duration, labels)
}

// Order Metrics
func (am *ApplicationMetrics) RecordOrderPlaced(market, side string, success bool) {
	labels := map[string]string{
		"market":  market,
		"side":    side,
		"success": strconv.FormatBool(success),
	}
	
	am.collector.IncrementCounter("orders_placed_total", labels)
}

func (am *ApplicationMetrics) RecordOrderCancelled(market string, success bool) {
	labels := map[string]string{
		"market":  market,
		"success": strconv.FormatBool(success),
	}
	
	am.collector.IncrementCounter("orders_cancelled_total", labels)
}

func (am *ApplicationMetrics) RecordOrderProcessingTime(market string, duration time.Duration) {
	labels := map[string]string{
		"market": market,
	}
	
	am.collector.RecordDuration("order_processing_time", duration, labels)
}

// Database Metrics
func (am *ApplicationMetrics) RecordDatabaseQuery(operation string, success bool, duration time.Duration) {
	labels := map[string]string{
		"operation": operation,
		"success":   strconv.FormatBool(success),
	}
	
	am.collector.IncrementCounter("database_queries_total", labels)
	am.collector.RecordDuration("database_query_duration", duration, labels)
}

func (am *ApplicationMetrics) SetDatabaseConnections(active, idle int) {
	am.collector.SetGauge("database_connections_active", float64(active), nil)
	am.collector.SetGauge("database_connections_idle", float64(idle), nil)
}

// External API Metrics
func (am *ApplicationMetrics) RecordExternalAPICall(service, endpoint string, statusCode int, duration time.Duration) {
	labels := map[string]string{
		"service":  service,
		"endpoint": endpoint,
		"status":   strconv.Itoa(statusCode),
	}
	
	am.collector.IncrementCounter("external_api_calls_total", labels)
	am.collector.RecordDuration("external_api_call_duration", duration, labels)
}

// Circuit Breaker Metrics
func (am *ApplicationMetrics) RecordCircuitBreakerState(name, state string) {
	labels := map[string]string{
		"name":  name,
		"state": state,
	}
	
	am.collector.IncrementCounter("circuit_breaker_state_changes_total", labels)
}

func (am *ApplicationMetrics) RecordCircuitBreakerRequest(name string, success bool) {
	labels := map[string]string{
		"name":    name,
		"success": strconv.FormatBool(success),
	}
	
	am.collector.IncrementCounter("circuit_breaker_requests_total", labels)
}

// Wallet Metrics
func (am *ApplicationMetrics) RecordWalletOperation(operation string, success bool, duration time.Duration) {
	labels := map[string]string{
		"operation": operation,
		"success":   strconv.FormatBool(success),
	}
	
	am.collector.IncrementCounter("wallet_operations_total", labels)
	am.collector.RecordDuration("wallet_operation_duration", duration, labels)
}

// Transaction Metrics
func (am *ApplicationMetrics) RecordTransactionBroadcast(success bool, gasUsed int64) {
	labels := map[string]string{
		"success": strconv.FormatBool(success),
	}
	
	am.collector.IncrementCounter("transactions_broadcast_total", labels)
	if gasUsed > 0 {
		am.collector.RecordHistogram("transaction_gas_used", float64(gasUsed), labels)
	}
}

// System Metrics
func (am *ApplicationMetrics) SetSystemMetrics(cpuUsage, memoryUsage float64, goroutines int) {
	am.collector.SetGauge("system_cpu_usage_percent", cpuUsage, nil)
	am.collector.SetGauge("system_memory_usage_percent", memoryUsage, nil)
	am.collector.SetGauge("system_goroutines_count", float64(goroutines), nil)
}

// Error Metrics
func (am *ApplicationMetrics) RecordError(errorType, component string) {
	labels := map[string]string{
		"type":      errorType,
		"component": component,
	}
	
	am.collector.IncrementCounter("errors_total", labels)
}

// MetricsMiddleware creates HTTP middleware for collecting metrics
func MetricsMiddleware(metrics *ApplicationMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Create a response writer wrapper to capture status code
			wrapper := &responseWriterWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}
			
			// Process request
			next.ServeHTTP(wrapper, r)
			
			// Record metrics
			duration := time.Since(start)
			metrics.RecordHTTPRequest(r.Method, r.URL.Path, wrapper.statusCode, duration)
		})
	}
}

// responseWriterWrapper wraps http.ResponseWriter to capture status code
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterWrapper) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// HealthMetrics tracks health check metrics
type HealthMetrics struct {
	metrics *ApplicationMetrics
}

// NewHealthMetrics creates a new health metrics instance
func NewHealthMetrics(metrics *ApplicationMetrics) *HealthMetrics {
	return &HealthMetrics{
		metrics: metrics,
	}
}

// RecordHealthCheck records a health check result
func (hm *HealthMetrics) RecordHealthCheck(component string, healthy bool, duration time.Duration) {
	labels := map[string]string{
		"component": component,
		"healthy":   strconv.FormatBool(healthy),
	}
	
	hm.metrics.collector.IncrementCounter("health_checks_total", labels)
	hm.metrics.collector.RecordDuration("health_check_duration", duration, labels)
}

// SetComponentHealth sets the current health status of a component
func (hm *HealthMetrics) SetComponentHealth(component string, healthy bool) {
	labels := map[string]string{
		"component": component,
	}
	
	var value float64
	if healthy {
		value = 1
	}
	
	hm.metrics.collector.SetGauge("component_health", value, labels)
}
