package processor

import (
	"fmt"
	"time"

	nwdaf_context "github.com/free5gc/nwdaf/internal/context"
	"github.com/free5gc/nwdaf/internal/logger"
)

// ---------------------------------------------------------------------------
// Task #8 – UE Mobility Prediction
// ---------------------------------------------------------------------------

// MobilityPrediction holds the predicted mobility state for a single UE.
type MobilityPrediction struct {
	Supi              string  `json:"supi"`
	CurrentLocation   string  `json:"currentLocation,omitempty"`
	PredictedBehavior string  `json:"predictedBehavior"` // "STATIONARY", "MOBILE", "DEPARTING"
	Confidence        float64 `json:"confidence"`         // 0.0 - 1.0
	LocationChanges   int     `json:"recentLocationChanges"`
	Reasoning         string  `json:"reasoning"`
}

// MobilityPredictionResult is the top-level output of PredictMobility.
type MobilityPredictionResult struct {
	Predictions      []MobilityPrediction `json:"predictions"`
	ModelVersion     string               `json:"modelVersion"`
	AnalysisWindow   string               `json:"analysisWindow"`
	TotalUesAnalyzed int                  `json:"totalUesAnalyzed"`
}

// PredictMobility analyses location events from the EventStore and classifies
// each tracked UE as STATIONARY, MOBILE, or DEPARTING.
func (p *Processor) PredictMobility(eventStore *nwdaf_context.EventStore) *MobilityPredictionResult {
	logger.ProducerLog.Infof("[Prediction] Running UE mobility prediction model")

	const analysisWindow = 10 * time.Minute
	windowStr := "10m"

	// Collect location-change events from AMF (UE_MOBILITY events carry location).
	locationEvents := eventStore.GetAmfEventsByType("UE_MOBILITY")

	// Group events by SUPI so we can count per-UE location changes.
	locationChangesPerUE := make(map[string]int)
	lastLocationPerUE := make(map[string]string)
	cutoff := time.Now().Add(-analysisWindow)

	for _, ev := range locationEvents {
		if ev.Timestamp.Before(cutoff) {
			continue
		}
		if ev.Supi == "" {
			continue
		}
		loc := ""
		if ev.Location != nil {
			loc = fmt.Sprintf("%v", ev.Location)
		}
		prev, seen := lastLocationPerUE[ev.Supi]
		if seen && prev != loc {
			locationChangesPerUE[ev.Supi]++
		}
		lastLocationPerUE[ev.Supi] = loc
	}

	// Also capture any SUPIs from the event store that had no location change but had events.
	for _, supi := range eventStore.GetUniqueSupis() {
		if _, exists := lastLocationPerUE[supi]; !exists {
			// Check if this SUPI had any AMF events in the window.
			for _, ev := range eventStore.GetAmfEventsForSupi(supi) {
				if !ev.Timestamp.Before(cutoff) {
					lastLocationPerUE[supi] = ""
					locationChangesPerUE[supi] = 0
					break
				}
			}
		}
	}

	predictions := make([]MobilityPrediction, 0, len(locationChangesPerUE))

	for supi, changes := range locationChangesPerUE {
		pred := MobilityPrediction{
			Supi:            supi,
			CurrentLocation: lastLocationPerUE[supi],
			LocationChanges: changes,
		}

		switch {
		case changes <= 1:
			pred.PredictedBehavior = "STATIONARY"
			pred.Confidence = 0.85
			pred.Reasoning = fmt.Sprintf("Only %d location change(s) observed in last %s; UE appears stationary", changes, windowStr)

		case changes <= 4:
			pred.PredictedBehavior = "MOBILE"
			pred.Confidence = 0.70 + float64(changes-2)*0.05
			if pred.Confidence > 0.90 {
				pred.Confidence = 0.90
			}
			pred.Reasoning = fmt.Sprintf("%d location changes in last %s indicate normal mobility", changes, windowStr)

		default:
			pred.PredictedBehavior = "DEPARTING"
			extra := changes - 4
			pred.Confidence = 0.75 + float64(extra)*0.03
			if pred.Confidence > 0.98 {
				pred.Confidence = 0.98
			}
			pred.Reasoning = fmt.Sprintf("%d location changes in last %s; high mobility suggests UE is departing area", changes, windowStr)
		}

		predictions = append(predictions, pred)
	}

	logger.ProducerLog.Infof("[Prediction] Mobility model analysed %d UEs", len(predictions))

	return &MobilityPredictionResult{
		Predictions:      predictions,
		ModelVersion:     "1.0.0",
		AnalysisWindow:   windowStr,
		TotalUesAnalyzed: len(predictions),
	}
}

// ---------------------------------------------------------------------------
// Task #9 – Session Load Forecasting
// ---------------------------------------------------------------------------

// SessionLoadForecast holds the session-load forecast for the network.
type SessionLoadForecast struct {
	CurrentSessionRate   float64 `json:"currentSessionRatePerMin"`
	PredictedSessionRate float64 `json:"predictedSessionRatePerMin"`
	Trend                string  `json:"trend"`           // "INCREASING", "STABLE", "DECREASING"
	TrendConfidence      float64 `json:"trendConfidence"` // 0.0 - 1.0
	ActiveSessions       int     `json:"activeSessions"`
	PeakSessionRate      float64 `json:"peakSessionRatePerMin"`
	Reasoning            string  `json:"reasoning"`
}

// SessionLoadForecastResult wraps the forecast with model metadata.
type SessionLoadForecastResult struct {
	Forecast       SessionLoadForecast `json:"forecast"`
	ModelVersion   string              `json:"modelVersion"`
	AnalysisWindow string              `json:"analysisWindow"`
}

// ForecastSessionLoad analyses PDU session establishment and release events
// and forecasts near-term session load.
func (p *Processor) ForecastSessionLoad(eventStore *nwdaf_context.EventStore) *SessionLoadForecastResult {
	logger.ProducerLog.Infof("[Prediction] Running session load forecasting model")

	const totalWindow = 4 * time.Minute
	windowStr := "4m"

	estEvents := eventStore.GetSmfEventsByType("PDU_SES_EST")
	relEvents := eventStore.GetSmfEventsByType("PDU_SES_REL")

	now := time.Now()
	cutoff := now.Add(-totalWindow)

	// Build per-minute buckets (4 buckets of 1 minute each, newest first).
	// bucket[0] = most recent minute, bucket[3] = oldest minute.
	buckets := make([]int, 4)
	for _, ev := range estEvents {
		if ev.Timestamp.Before(cutoff) {
			continue
		}
		age := now.Sub(ev.Timestamp)
		bucket := int(age.Minutes()) // 0 = most recent minute
		if bucket < 4 {
			buckets[bucket]++
		}
	}

	// Determine active sessions: establishments minus releases.
	activeSessions := len(estEvents) - len(relEvents)
	if activeSessions < 0 {
		activeSessions = 0
	}

	// Find peak rate across all buckets (events per minute).
	peakRate := 0.0
	for _, b := range buckets {
		rate := float64(b)
		if rate > peakRate {
			peakRate = rate
		}
	}

	// Compare average of newest 2 minutes vs previous 2 minutes.
	recent := (float64(buckets[0]) + float64(buckets[1])) / 2.0
	previous := (float64(buckets[2]) + float64(buckets[3])) / 2.0

	var trend string
	var trendConfidence float64
	var reasoning string
	var predictedRate float64

	if previous == 0 {
		trend = "STABLE"
		trendConfidence = 0.50
		predictedRate = recent
		reasoning = "Insufficient historical data; defaulting to STABLE forecast"
	} else {
		changePct := (recent - previous) / previous
		switch {
		case changePct > 0.20:
			trend = "INCREASING"
			trendConfidence = 0.60 + changePct*0.30
			if trendConfidence > 0.95 {
				trendConfidence = 0.95
			}
			predictedRate = recent * (1 + changePct)
			reasoning = fmt.Sprintf("Session rate increased %.0f%% in recent 2 min vs prior 2 min", changePct*100)

		case changePct < -0.20:
			trend = "DECREASING"
			trendConfidence = 0.60 + (-changePct)*0.30
			if trendConfidence > 0.95 {
				trendConfidence = 0.95
			}
			predictedRate = recent * (1 + changePct)
			if predictedRate < 0 {
				predictedRate = 0
			}
			reasoning = fmt.Sprintf("Session rate decreased %.0f%% in recent 2 min vs prior 2 min", -changePct*100)

		default:
			trend = "STABLE"
			trendConfidence = 0.80 - (changePct*changePct)*2
			if trendConfidence < 0.50 {
				trendConfidence = 0.50
			}
			predictedRate = recent
			reasoning = fmt.Sprintf("Session rate change (%.0f%%) within ±20%% threshold; trend is STABLE", changePct*100)
		}
	}

	logger.ProducerLog.Infof("[Prediction] Session forecast trend=%s confidence=%.2f", trend, trendConfidence)

	return &SessionLoadForecastResult{
		Forecast: SessionLoadForecast{
			CurrentSessionRate:   recent,
			PredictedSessionRate: predictedRate,
			Trend:                trend,
			TrendConfidence:      trendConfidence,
			ActiveSessions:       activeSessions,
			PeakSessionRate:      peakRate,
			Reasoning:            reasoning,
		},
		ModelVersion:   "1.0.0",
		AnalysisWindow: windowStr,
	}
}

// ---------------------------------------------------------------------------
// Task #10 – Anomaly Scoring
// ---------------------------------------------------------------------------

// AnomalyScore holds the anomaly score for a single UE / event category.
type AnomalyScore struct {
	Supi         string  `json:"supi,omitempty"`
	Category     string  `json:"category"`     // "REGISTRATION", "SESSION", "CONNECTIVITY"
	Score        float64 `json:"score"`         // 0.0 (normal) – 1.0 (highly anomalous)
	EventCount   int     `json:"eventCount"`
	BaselineRate float64 `json:"baselineRate"` // expected events per minute
	ObservedRate float64 `json:"observedRate"` // actual events per minute
	IsAnomalous  bool    `json:"isAnomalous"`
	Reasoning    string  `json:"reasoning"`
}

// AnomalyScoreResult is the top-level output of ScoreAnomalies.
type AnomalyScoreResult struct {
	Scores         []AnomalyScore `json:"scores"`
	OverallHealth  string         `json:"overallHealth"` // "HEALTHY", "WARNING", "CRITICAL"
	TotalAnomalies int            `json:"totalAnomalies"`
	ModelVersion   string         `json:"modelVersion"`
	AnalysisWindow string         `json:"analysisWindow"`
}

// anomalyBaseline defines the expected event rate (events/min) for each tracked category.
type anomalyBaseline struct {
	eventType      string
	category       string
	baselinePerMin float64 // task spec: 1 event/5min → 0.2/min; 2 events/5min → 0.4/min
}

var anomalyBaselines = []anomalyBaseline{
	// Registration: 1 event / 5 min = 0.2 events/min
	{eventType: "AM_POLICY_ASSOC", category: "REGISTRATION", baselinePerMin: 0.20},
	// Session: 2 events / 5 min = 0.4 events/min
	{eventType: "PDU_SES_EST", category: "SESSION", baselinePerMin: 0.40},
	// Connectivity: 2 events / 5 min = 0.4 events/min
	{eventType: "UE_REACHABILITY", category: "CONNECTIVITY", baselinePerMin: 0.40},
}

// ScoreAnomalies calculates anomaly scores across event categories and per UE.
//
// GetEventRate returns events-per-second; we convert to events-per-minute for
// display by multiplying by 60.
func (p *Processor) ScoreAnomalies(eventStore *nwdaf_context.EventStore) *AnomalyScoreResult {
	logger.ProducerLog.Infof("[Prediction] Running anomaly scoring model")

	const analysisWindow = 5 * time.Minute
	windowStr := "5m"
	windowMin := analysisWindow.Minutes()

	scores := []AnomalyScore{}
	anomalyCount := 0

	// --- Category-level scoring ---
	for _, cb := range anomalyBaselines {
		// GetEventRate returns events/second; convert to events/min.
		observedRatePerSec := eventStore.GetEventRate(cb.eventType, analysisWindow)
		observedRate := observedRatePerSec * 60.0

		// Approximate event count from the rate and window.
		eventCount := int(observedRatePerSec * analysisWindow.Seconds())

		var score float64
		if observedRate > cb.baselinePerMin {
			diff := observedRate - cb.baselinePerMin
			score = diff / cb.baselinePerMin
			if score > 1.0 {
				score = 1.0
			}
		}

		isAnomalous := score > 0.5

		var reasoning string
		if isAnomalous {
			reasoning = fmt.Sprintf("Observed %.2f events/min vs baseline %.2f events/min for %s events (%.0f%% above baseline)",
				observedRate, cb.baselinePerMin, cb.category, score*100)
		} else {
			reasoning = fmt.Sprintf("Event rate %.2f events/min is within normal range (baseline %.2f events/min)",
				observedRate, cb.baselinePerMin)
		}

		as := AnomalyScore{
			Category:     cb.category,
			Score:        score,
			EventCount:   eventCount,
			BaselineRate: cb.baselinePerMin,
			ObservedRate: observedRate,
			IsAnomalous:  isAnomalous,
			Reasoning:    reasoning,
		}
		scores = append(scores, as)
		if isAnomalous {
			anomalyCount++
		}
	}

	// --- Per-UE scoring based on location event frequency ---
	// Baseline: 2 location events per 5 min = 0.4 events/min
	const ueBaselinePerMin = 0.40
	cutoff := time.Now().Add(-analysisWindow)

	for _, supi := range eventStore.GetUniqueSupis() {
		amfEvents := eventStore.GetAmfEventsForSupi(supi)
		count := 0
		for _, ev := range amfEvents {
			if !ev.Timestamp.Before(cutoff) {
				count++
			}
		}
		if count == 0 {
			continue
		}

		observedRate := float64(count) / windowMin

		var score float64
		if observedRate > ueBaselinePerMin {
			diff := observedRate - ueBaselinePerMin
			score = diff / ueBaselinePerMin
			if score > 1.0 {
				score = 1.0
			}
		}

		isAnomalous := score > 0.5

		var reasoning string
		if isAnomalous {
			reasoning = fmt.Sprintf("UE %s generated %.2f events/min (baseline %.2f events/min)", supi, observedRate, ueBaselinePerMin)
		} else {
			reasoning = fmt.Sprintf("UE %s event rate %.2f events/min is normal", supi, observedRate)
		}

		as := AnomalyScore{
			Supi:         supi,
			Category:     "CONNECTIVITY",
			Score:        score,
			EventCount:   count,
			BaselineRate: ueBaselinePerMin,
			ObservedRate: observedRate,
			IsAnomalous:  isAnomalous,
			Reasoning:    reasoning,
		}
		scores = append(scores, as)
		if isAnomalous {
			anomalyCount++
		}
	}

	var overallHealth string
	switch {
	case anomalyCount == 0:
		overallHealth = "HEALTHY"
	case anomalyCount <= 2:
		overallHealth = "WARNING"
	default:
		overallHealth = "CRITICAL"
	}

	logger.ProducerLog.Infof("[Prediction] Anomaly scoring complete: health=%s anomalies=%d", overallHealth, anomalyCount)

	return &AnomalyScoreResult{
		Scores:         scores,
		OverallHealth:  overallHealth,
		TotalAnomalies: anomalyCount,
		ModelVersion:   "1.0.0",
		AnalysisWindow: windowStr,
	}
}
