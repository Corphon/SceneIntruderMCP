// internal/utils/metrics.go
package utils

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector collects application metrics
type MetricsCollector struct {
	counters   map[string]*Counter
	gauges     map[string]*Gauge
	histograms map[string]*Histogram

	mu sync.RWMutex
}

// Counter metric - using atomic operations for thread-safe value updates
type Counter struct {
	name  string
	value int64 // Use atomic operations for this field
}

// Gauge metric - using atomic operations for thread-safe value updates
type Gauge struct {
	name  string
	value int64 // Use atomic operations for this field
}

// Histogram metric (simple implementation tracking count, sum, min, max)
type Histogram struct {
	name    string
	count   int64
	sum     int64
	min     int64
	max     int64
	buckets []int64 // For future expansion
	mu      sync.Mutex
}

var (
	globalMetrics *MetricsCollector
	metricsOnce   sync.Once
)

// GetMetricsCollector returns the global metrics collector
func GetMetricsCollector() *MetricsCollector {
	metricsOnce.Do(func() {
		globalMetrics = &MetricsCollector{
			counters:   make(map[string]*Counter),
			gauges:     make(map[string]*Gauge),
			histograms: make(map[string]*Histogram),
		}
	})
	return globalMetrics
}

// IncrementCounter increments a counter metric using atomic operations to reduce lock contention
func (m *MetricsCollector) IncrementCounter(name string) {
	// First try with read lock (fast path for existing counters)
	m.mu.RLock()
	counter, exists := m.counters[name]
	m.mu.RUnlock()

	if exists {
		atomic.AddInt64(&counter.value, 1)
		return
	}

	// Slow path: need to create new counter
	m.mu.Lock()
	// Double-check after acquiring write lock
	counter, exists = m.counters[name]
	if !exists {
		counter = &Counter{name: name}
		m.counters[name] = counter
	}
	m.mu.Unlock()

	atomic.AddInt64(&counter.value, 1)
}

// AddCounter adds a value to a counter metric using atomic operations
func (m *MetricsCollector) AddCounter(name string, value int64) {
	// First try with read lock (fast path for existing counters)
	m.mu.RLock()
	counter, exists := m.counters[name]
	m.mu.RUnlock()

	if exists {
		atomic.AddInt64(&counter.value, value)
		return
	}

	// Slow path: need to create new counter
	m.mu.Lock()
	// Double-check after acquiring write lock
	counter, exists = m.counters[name]
	if !exists {
		counter = &Counter{name: name}
		m.counters[name] = counter
	}
	m.mu.Unlock()

	atomic.AddInt64(&counter.value, value)
}

// SetGauge sets a gauge metric using atomic operations
func (m *MetricsCollector) SetGauge(name string, value int64) {
	// First try with read lock (fast path for existing gauges)
	m.mu.RLock()
	gauge, exists := m.gauges[name]
	m.mu.RUnlock()

	if exists {
		atomic.StoreInt64(&gauge.value, value)
		return
	}

	// Slow path: need to create new gauge
	m.mu.Lock()
	// Double-check after acquiring write lock
	gauge, exists = m.gauges[name]
	if !exists {
		gauge = &Gauge{name: name}
		m.gauges[name] = gauge
	}
	m.mu.Unlock()

	atomic.StoreInt64(&gauge.value, value)
}

// IncGauge increments a gauge metric
func (m *MetricsCollector) IncGauge(name string) {
	// First try with read lock (fast path for existing gauges)
	m.mu.RLock()
	gauge, exists := m.gauges[name]
	m.mu.RUnlock()

	if exists {
		atomic.AddInt64(&gauge.value, 1)
		return
	}

	// Slow path: gauge doesn't exist, use SetGauge to create and set
	m.SetGauge(name, 1)
}

// DecGauge decrements a gauge metric
func (m *MetricsCollector) DecGauge(name string) {
	// First try with read lock (fast path for existing gauges)
	m.mu.RLock()
	gauge, exists := m.gauges[name]
	m.mu.RUnlock()

	if exists {
		atomic.AddInt64(&gauge.value, -1)
		return
	}

	// Slow path: gauge doesn't exist, use SetGauge to create and set
	m.SetGauge(name, -1)
}

// GetGauge gets the current value of a gauge using atomic load
func (m *MetricsCollector) GetGauge(name string) int64 {
	m.mu.RLock()
	gauge, exists := m.gauges[name]
	m.mu.RUnlock()

	if !exists {
		return 0
	}

	return atomic.LoadInt64(&gauge.value)
}

// RecordHistogram records a value in a histogram
func (m *MetricsCollector) RecordHistogram(name string, value int64) {
	// First try with read lock (fast path for existing histograms)
	m.mu.RLock()
	histogram, exists := m.histograms[name]
	m.mu.RUnlock()

	if !exists {
		// Slow path: need to create new histogram
		m.mu.Lock()
		// Double-check after acquiring write lock
		histogram, exists = m.histograms[name]
		if !exists {
			histogram = &Histogram{
				name: name,
				min:  value,
				max:  value,
			}
			m.histograms[name] = histogram
		}
		m.mu.Unlock()
	}

	histogram.mu.Lock()
	defer histogram.mu.Unlock()

	histogram.count++
	histogram.sum += value

	if value < histogram.min {
		histogram.min = value
	}
	if value > histogram.max {
		histogram.max = value
	}
}

// GetMetrics returns a snapshot of all metrics
func (m *MetricsCollector) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make(map[string]interface{})

	// Collect counters using atomic load
	counters := make(map[string]int64)
	for name, counter := range m.counters {
		counters[name] = atomic.LoadInt64(&counter.value)
	}
	metrics["counters"] = counters

	// Collect gauges using atomic load
	gauges := make(map[string]int64)
	for name, gauge := range m.gauges {
		gauges[name] = atomic.LoadInt64(&gauge.value)
	}
	metrics["gauges"] = gauges

	// Collect histograms (still needs mutex for min/max consistency)
	histograms := make(map[string]map[string]int64)
	for name, histogram := range m.histograms {
		histogram.mu.Lock()
		histograms[name] = map[string]int64{
			"count": histogram.count,
			"sum":   histogram.sum,
			"min":   histogram.min,
			"max":   histogram.max,
		}
		histogram.mu.Unlock()
	}
	metrics["histograms"] = histograms

	return metrics
}

// GetCounterValue gets the current value of a counter using atomic load
func (m *MetricsCollector) GetCounterValue(name string) int64 {
	m.mu.RLock()
	counter, exists := m.counters[name]
	m.mu.RUnlock()

	if !exists {
		return 0
	}

	return atomic.LoadInt64(&counter.value)
}

// APIMetrics represents API-specific metrics
type APIMetrics struct {
	metrics *MetricsCollector
	logger  *Logger
}

// NewAPIMetrics creates a new API metrics instance
func NewAPIMetrics() *APIMetrics {
	return &APIMetrics{
		metrics: GetMetricsCollector(),
		logger:  GetLogger(),
	}
}

// RecordAPIRequest records metrics for an API request
func (am *APIMetrics) RecordAPIRequest(endpoint, method string, statusCode int, duration time.Duration) {
	// Increment total requests
	am.metrics.IncrementCounter("api_requests_total")
	
	// Increment requests by endpoint and method
	am.metrics.IncrementCounter("api_requests_" + method + "_" + endpoint)
	
	// Record response time histogram
	am.metrics.RecordHistogram("api_response_time_ms", duration.Milliseconds())
	
	// Increment status code counter
	am.metrics.IncrementCounter("api_responses_" + string(rune(statusCode/100)) + "xx")
	
	// Log the request (at debug level)
	am.logger.Debug("API request completed", map[string]interface{}{
		"endpoint":   endpoint,
		"method":     method,
		"status":     statusCode,
		"duration":   duration.Milliseconds(),
		"timestamp":  time.Now().Unix(),
	})
}

// RecordLLMRequest records metrics for an LLM request
func (am *APIMetrics) RecordLLMRequest(provider, model string, tokensUsed int, duration time.Duration) {
	// Increment total LLM requests
	am.metrics.IncrementCounter("llm_requests_total")
	
	// Increment by provider
	am.metrics.IncrementCounter("llm_requests_" + provider)
	
	// Record tokens used
	am.metrics.AddCounter("llm_tokens_total", int64(tokensUsed))
	
	// Record response time
	am.metrics.RecordHistogram("llm_response_time_ms", duration.Milliseconds())
	
	// Log the LLM request
	am.logger.Info("LLM request completed", map[string]interface{}{
		"provider":   provider,
		"model":      model,
		"tokens":     tokensUsed,
		"duration":   duration.Milliseconds(),
		"timestamp":  time.Now().Unix(),
	})
}

// RecordSceneInteraction records metrics for a scene interaction
func (am *APIMetrics) RecordSceneInteraction(sceneID, interactionType string) {
	// Increment total interactions
	am.metrics.IncrementCounter("scene_interactions_total")
	
	// Increment by interaction type
	am.metrics.IncrementCounter("scene_interactions_" + interactionType)
	
	// Increment by scene
	am.metrics.IncrementCounter("scene_" + sceneID + "_interactions")
	
	// Log the interaction
	am.logger.Debug("Scene interaction recorded", map[string]interface{}{
		"scene_id": sceneID,
		"type":     interactionType,
		"timestamp": time.Now().Unix(),
	})
}

// RecordUserAction records metrics for a user action
func (am *APIMetrics) RecordUserAction(userID, action string) {
	// Increment total user actions
	am.metrics.IncrementCounter("user_actions_total")
	
	// Increment by action type
	am.metrics.IncrementCounter("user_actions_" + action)
	
	// Increment by user
	am.metrics.IncrementCounter("user_" + userID + "_actions")
	
	// Log the user action
	am.logger.Debug("User action recorded", map[string]interface{}{
		"user_id": userID,
		"action":  action,
		"timestamp": time.Now().Unix(),
	})
}

// RecordError records an error metric
func (am *APIMetrics) RecordError(errorType, component string) {
	// Increment total errors
	am.metrics.IncrementCounter("errors_total")
	
	// Increment by error type
	am.metrics.IncrementCounter("errors_" + errorType)
	
	// Increment by component
	am.metrics.IncrementCounter("errors_" + component)
	
	// Log the error
	am.logger.Error("Error recorded", map[string]interface{}{
		"type":      errorType,
		"component": component,
		"timestamp": time.Now().Unix(),
	})
}

// StartMetricsCollection starts background metrics collection
func (am *APIMetrics) StartMetricsCollection(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Periodically log metrics summary
				metrics := am.metrics.GetMetrics()
				am.logger.Info("Periodic metrics report", map[string]interface{}{
					"timestamp": time.Now().Unix(),
					"metrics":   metrics,
				})
			}
		}
	}()
}
