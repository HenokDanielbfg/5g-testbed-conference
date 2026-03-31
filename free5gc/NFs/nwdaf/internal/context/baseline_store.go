package context

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/free5gc/nwdaf/internal/logger"
)

// epochSample holds the registration activity observed during one epoch window.
type epochSample struct {
	Timestamp   time.Time
	UniqueSupis int     // unique SUPIs seen in REGISTRATION_STATE_REPORT this epoch
	RegRate     float64 // registration events/sec this epoch
}

// DetectedAnomaly records a flood anomaly detected during a background epoch sample.
// Storing detections here means the signal persists across query windows — an agent
// querying 45 seconds after the flood will still see it.
type DetectedAnomaly struct {
	Timestamp   time.Time
	AnomalyType string  // currently "REGISTRATION_FLOOD"
	Severity    string  // LOW / MEDIUM / HIGH
	Score       float64 // 0.0–1.0
	Current     float64 // unique SUPIs observed this epoch
	Mean        float64 // baseline mean at detection time
	StdDev      float64 // baseline stddev at detection time
	Confidence  string  // baseline confidence at detection time
	ZScore      float64 // z-score (or ratio when stddev=0)
	Description string
}

// BaselineStats is the summary of the baseline derived from accumulated epochs.
type BaselineStats struct {
	Mean           float64
	StdDev         float64
	EpochCount     int
	OldestEpochAge time.Duration
	Confidence     string // "NONE", "LOW", "MEDIUM", "HIGH"
}

// BaselineStore accumulates epoch-level registration statistics to establish
// a dynamic baseline for anomaly detection. A background goroutine samples
// the EventStore every epochDur, appends to a rolling window of maxEpochs,
// and stores any detected anomalies for later retrieval.
type BaselineStore struct {
	mu                sync.RWMutex
	epochs            []epochSample
	maxEpochs         int
	epochDur          time.Duration
	eventStore        *EventStore
	stopCh            chan struct{}
	detectedAnomalies []DetectedAnomaly // ring buffer of recent anomaly detections
	maxAnomalies      int               // max anomalies retained
}

// NewBaselineStore creates a BaselineStore and starts its background sampler.
// epochDur controls how often a sample is taken; maxEpochs controls how many
// samples are retained (oldest are dropped when the cap is reached).
func NewBaselineStore(es *EventStore, epochDur time.Duration, maxEpochs int) *BaselineStore {
	b := &BaselineStore{
		maxEpochs:    maxEpochs,
		epochDur:     epochDur,
		eventStore:   es,
		stopCh:       make(chan struct{}),
		maxAnomalies: 100,
	}
	go b.run()
	return b
}

// Stop shuts down the background sampler.
func (b *BaselineStore) Stop() {
	close(b.stopCh)
}

// EpochDuration returns the configured epoch interval.
func (b *BaselineStore) EpochDuration() time.Duration {
	return b.epochDur
}

// GetRegistrationBaseline returns baseline statistics derived from accumulated
// epoch samples. Confidence reflects how many epochs have been collected:
//
//	NONE   < 1 epoch
//	LOW    1–2 epochs   (~30–60 s of history)
//	MEDIUM 3–9 epochs   (~90 s – 4.5 min)
//	HIGH   ≥ 10 epochs  (~5+ min)
func (b *BaselineStore) GetRegistrationBaseline() BaselineStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	n := len(b.epochs)
	if n == 0 {
		return BaselineStats{Confidence: "NONE"}
	}

	// Mean unique-SUPI count per epoch.
	sum := 0.0
	for _, e := range b.epochs {
		sum += float64(e.UniqueSupis)
	}
	mean := sum / float64(n)

	// Population standard deviation.
	variance := 0.0
	for _, e := range b.epochs {
		d := float64(e.UniqueSupis) - mean
		variance += d * d
	}
	stddev := math.Sqrt(variance / float64(n))

	confidence := "LOW"
	switch {
	case n >= 10:
		confidence = "HIGH"
	case n >= 3:
		confidence = "MEDIUM"
	}

	return BaselineStats{
		Mean:           mean,
		StdDev:         stddev,
		EpochCount:     n,
		OldestEpochAge: time.Since(b.epochs[0].Timestamp),
		Confidence:     confidence,
	}
}

// GetDetectedAnomalies returns anomalies detected in background epoch sampling
// that occurred within maxAge of now. Anomalies are deduplicated by type within
// each epoch, so a single flood produces one entry per epoch it was detected in.
func (b *BaselineStore) GetDetectedAnomalies(maxAge time.Duration) []DetectedAnomaly {
	b.mu.RLock()
	defer b.mu.RUnlock()
	cutoff := time.Now().Add(-maxAge)
	var result []DetectedAnomaly
	for _, a := range b.detectedAnomalies {
		if a.Timestamp.After(cutoff) {
			result = append(result, a)
		}
	}
	return result
}

func (b *BaselineStore) run() {
	ticker := time.NewTicker(b.epochDur)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			b.sample()
		case <-b.stopCh:
			return
		}
	}
}

func (b *BaselineStore) sample() {
	now := time.Now()
	cutoff := now.Add(-b.epochDur)
	events := b.eventStore.GetAmfEventsByType("REGISTRATION_STATE_REPORT")

	supiSet := make(map[string]struct{})
	for _, ev := range events {
		if ev.Timestamp.After(cutoff) && ev.Supi != "" {
			supiSet[ev.Supi] = struct{}{}
		}
	}

	regRate := b.eventStore.GetEventRate("REGISTRATION_STATE_REPORT", b.epochDur)

	s := epochSample{
		Timestamp:   now,
		UniqueSupis: len(supiSet),
		RegRate:     regRate,
	}

	b.mu.Lock()
	b.epochs = append(b.epochs, s)
	if len(b.epochs) > b.maxEpochs {
		b.epochs = b.epochs[len(b.epochs)-b.maxEpochs:]
	}

	// Run flood detection against the current baseline (computed inline so we
	// hold the lock only once and avoid calling GetRegistrationBaseline which
	// would try to re-acquire the read lock).
	n := len(b.epochs)
	if n > 0 {
		sum := 0.0
		for _, e := range b.epochs {
			sum += float64(e.UniqueSupis)
		}
		mean := sum / float64(n)

		variance := 0.0
		for _, e := range b.epochs {
			d := float64(e.UniqueSupis) - mean
			variance += d * d
		}
		stddev := math.Sqrt(variance / float64(n))

		confidence := "LOW"
		switch {
		case n >= 10:
			confidence = "HIGH"
		case n >= 3:
			confidence = "MEDIUM"
		}

		current := float64(s.UniqueSupis)
		var zScore float64
		isFlood := false

		if stddev > 0 {
			zScore = (current - mean) / stddev
			isFlood = zScore > 2.5
		} else if mean > 0 {
			zScore = current / mean
			isFlood = zScore > 3.0
		}

		if isFlood {
			score := math.Min(1.0, zScore/5.0)
			severity := "LOW"
			switch {
			case score >= 0.75:
				severity = "HIGH"
			case score >= 0.4:
				severity = "MEDIUM"
			}
			ratio := 0.0
			if mean > 0 {
				ratio = current / mean
			}
			anomaly := DetectedAnomaly{
				Timestamp:   now,
				AnomalyType: "REGISTRATION_FLOOD",
				Severity:    severity,
				Score:       score,
				Current:     current,
				Mean:        mean,
				StdDev:      stddev,
				Confidence:  confidence,
				ZScore:      zScore,
				Description: fmt.Sprintf("%.0f unique UEs registered this epoch (%.1fx above baseline mean=%.1f, confidence=%s)",
					current, ratio, mean, confidence),
			}
			b.detectedAnomalies = append(b.detectedAnomalies, anomaly)
			if len(b.detectedAnomalies) > b.maxAnomalies {
				b.detectedAnomalies = b.detectedAnomalies[len(b.detectedAnomalies)-b.maxAnomalies:]
			}
			logger.CtxLog.Warnf("[Baseline] REGISTRATION_FLOOD detected: uniqueSupis=%.0f zScore=%.2f severity=%s",
				current, zScore, severity)
		}
	}

	b.mu.Unlock()

	logger.CtxLog.Debugf("[Baseline] epoch sampled: uniqueSupis=%d regRate=%.3f/s totalEpochs=%d",
		s.UniqueSupis, s.RegRate, len(b.epochs))
}
