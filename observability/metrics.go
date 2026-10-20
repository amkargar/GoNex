package observability

import (
	"context"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// MaxOperations bounds total operation labels, including one shared overflow series.
const MaxOperations = 128
const MaxOperationLength = 160

var latencyBounds = [...]time.Duration{5 * time.Millisecond, 10 * time.Millisecond, 25 * time.Millisecond, 50 * time.Millisecond, 100 * time.Millisecond, 250 * time.Millisecond, 500 * time.Millisecond, time.Second, 2500 * time.Millisecond, 5 * time.Second}

func LatencyBounds() []time.Duration { return append([]time.Duration(nil), latencyBounds[:]...) }

type Measurement struct {
	DurationBuckets  [11]int64 `json:"duration_buckets"`
	Requests         int64     `json:"requests"`
	Errors           int64     `json:"errors"`
	InFlight         int64     `json:"in_flight"`
	TotalNanoseconds int64     `json:"total_nanoseconds"`
}
type Snapshot struct {
	Ready      bool                   `json:"ready"`
	Operations map[string]Measurement `json:"operations"`
}
type Metrics struct {
	mu         sync.Mutex
	ready      bool
	check      func(context.Context) error
	operations map[string]Measurement
}

func NewMetrics() *Metrics { return &Metrics{operations: map[string]Measurement{}} }

// SetReadiness connects snapshots to the application's dependency checks.
func (m *Metrics) SetReadiness(check func(context.Context) error) {
	m.mu.Lock()
	m.check = check
	m.mu.Unlock()
}
func (m *Metrics) Start(context.Context) error {
	m.mu.Lock()
	m.ready = true
	m.mu.Unlock()
	return nil
}
func (m *Metrics) Close(context.Context) error {
	m.mu.Lock()
	m.ready = false
	m.mu.Unlock()
	return nil
}

// Begin accepts a fixed route/RPC label, never a URL, user ID or secret.
func (m *Metrics) Begin(operation string) func(bool) {
	start := time.Now()
	m.mu.Lock()
	if m.operations == nil {
		m.operations = map[string]Measurement{}
	}
	if len(operation) > MaxOperationLength || !utf8.ValidString(operation) {
		operation = "__overflow__"
	}
	for _, r := range operation {
		if unicode.IsControl(r) {
			operation = "__overflow__"
			break
		}
	}
	if _, known := m.operations[operation]; !known && len(m.operations) >= MaxOperations-1 {
		operation = "__overflow__"
	}
	value := m.operations[operation]
	value.InFlight++
	m.operations[operation] = value
	m.mu.Unlock()
	var once sync.Once
	return func(failed bool) {
		once.Do(func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			value := m.operations[operation]
			value.InFlight--
			value.Requests++
			if failed {
				value.Errors++
			}
			elapsed := time.Since(start)
			value.TotalNanoseconds += elapsed.Nanoseconds()
			for i, bound := range latencyBounds {
				if elapsed <= bound {
					value.DurationBuckets[i]++
				}
			}
			value.DurationBuckets[len(latencyBounds)]++
			m.operations[operation] = value
		})
	}
}
func (m *Metrics) Snapshot() Snapshot {
	return m.SnapshotContext(context.Background())
}

func (m *Metrics) SnapshotContext(ctx context.Context) Snapshot {
	m.mu.Lock()
	result := Snapshot{Ready: m.ready, Operations: map[string]Measurement{}}
	check := m.check
	for key, value := range m.operations {
		result.Operations[key] = value
	}
	m.mu.Unlock()
	if result.Ready && check != nil {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		result.Ready = check(ctx) == nil && ctx.Err() == nil
	}
	return result
}
