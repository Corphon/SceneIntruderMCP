// internal/utils/metrics.go
package utils

import (
	"context"
	"sync"
	"time"
)

// MetricsCollector collects application metrics
type MetricsCollector struct {
	counters   map[string]*Counter
	gauges     map[string]*Gauge
	histograms map[string]*Histogram
	
	mu sync.RWMutex
}

// Counter metric
type Counter struct {
	name  string
	value int64
	mu    sync.Mutex
}

// Gauge metric
type Gauge struct {
	name  string
	value int64
	mu    sync.Mutex
}

// Histogram metric (simple implementation tracking count, sum, min, max)
type Histogram struct {
	name        string
	count       int64
	sum         int64
	min         int64
	max         int64
	buckets     []int64 // For future expansion
	mu          sync.Mutex
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

// IncrementCounter increments a counter metric
func (m *MetricsCollector) IncrementCounter(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	counter, exists := m.counters[name]
	if !exists {
		counter = &Counter{name: name}
		m.counters[name] = counter
	}
	
	counter.mu.Lock()
	defer counter.mu.Unlock()
	counter.value++
}

// AddCounter adds a value to a counter metric
func (m *MetricsCollector) AddCounter(name string, value int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	counter, exists := m.counters[name]
	if !exists {
		counter = &Counter{name: name}
		m.counters[name] = counter
	}
	
	counter.mu.Lock()
	defer counter.mu.Unlock()
	counter.value += value
}

// SetGauge sets a gauge metric
func (m *MetricsCollector) SetGauge(name string, value int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	gauge, exists := m.gauges[name]
	if !exists {
		gauge = &Gauge{name: name}
		m.gauges[name] = gauge
	}
	
	gauge.mu.Lock()
	defer gauge.mu.Unlock()
	gauge.value = value
}

// IncGauge increments a gauge metric
func (m *MetricsCollector) IncGauge(name string) {
	m.SetGauge(name, m.GetGauge(name)+1)
}

// DecGauge decrements a gauge metric
func (m *MetricsCollector) DecGauge(name string) {
	m.SetGauge(name, m.GetGauge(name)-1)
}

// GetGauge gets the current value of a gauge
func (m *MetricsCollector) GetGauge(name string) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	gauge, exists := m.gauges[name]
	if !exists {
		return 0
	}
	
	gauge.mu.Lock()
	defer gauge.mu.Unlock()
	return gauge.value
}

// RecordHistogram records a value in a histogram
func (m *MetricsCollector) RecordHistogram(name string, value int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	histogram, exists := m.histograms[name]
	if !exists {
		histogram = &Histogram{
			name:  name,
			min:   value,
			max:   value,
		}
		m.histograms[name] = histogram
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
	
	// Collect counters
	counters := make(map[string]int64)
	for name, counter := range m.counters {
		counter.mu.Lock()
		counters[name] = counter.value
		counter.mu.Unlock()
	}
	metrics["counters"] = counters
	
	// Collect gauges
	gauges := make(map[string]int64)
	for name, gauge := range m.gauges {
		gauge.mu.Lock()
		gauges[name] = gauge.value
		gauge.mu.Unlock()
	}
	metrics["gauges"] = gauges
	
	// Collect histograms
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

// GetCounterValue gets the current value of a counter
func (m *MetricsCollector) GetCounterValue(name string) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	counter, exists := m.counters[name]
	if !exists {
		return 0
	}
	
	counter.mu.Lock()
	defer counter.mu.Unlock()
	return counter.value
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
