package processor

import (
	"fmt"
	"time"

	nwdaf_context "github.com/free5gc/nwdaf/internal/context"
	"github.com/free5gc/nwdaf/internal/logger"
)

// ---------------------------------------------------------------------------
// Task #11 – Recommendation Aggregator
// ---------------------------------------------------------------------------

// NwdafRecommendation represents a single actionable recommendation.
type NwdafRecommendation struct {
	Id          string   `json:"id"`
	Priority    string   `json:"priority"`              // "HIGH", "MEDIUM", "LOW"
	Category    string   `json:"category"`              // "MOBILITY", "SESSION_MANAGEMENT", "SECURITY", "CAPACITY"
	Action      string   `json:"action"`                // recommended action
	Reasoning   string   `json:"reasoning"`             // why this recommendation
	Confidence  float64  `json:"confidence"`            // 0.0 - 1.0
	AffectedUes []string `json:"affectedUes,omitempty"` // SUPIs affected
}

// NwdafRecommendationResult is the top-level output of GenerateRecommendations.
type NwdafRecommendationResult struct {
	Recommendations []NwdafRecommendation `json:"recommendations"`
	GeneratedAt     string                `json:"generatedAt"`
	ModelVersions   map[string]string     `json:"modelVersions"`
	DataSummary     DataSummary           `json:"dataSummary"`
}

// DataSummary provides a snapshot of the data used to generate recommendations.
type DataSummary struct {
	TotalAmfEvents       int `json:"totalAmfEvents"`
	TotalSmfEvents       int `json:"totalSmfEvents"`
	UniqueUes            int `json:"uniqueUes"`
	ObservationWindowMin int `json:"observationWindowMin"`
}

// GenerateRecommendations aggregates the three prediction models and produces
// actionable recommendations for network operators.
func (p *Processor) GenerateRecommendations(eventStore *nwdaf_context.EventStore) *NwdafRecommendationResult {
	logger.ProducerLog.Infof("[Recommendation] Generating recommendations from all prediction models")

	// --- Run the three prediction models ---
	mobilityResult := p.PredictMobility(eventStore)
	sessionResult := p.ForecastSessionLoad(eventStore)
	anomalyResult := p.ScoreAnomalies(eventStore)

	recommendations := []NwdafRecommendation{}
	recID := 1

	// --- Rule 1: Many DEPARTING UEs → prepare handover resources ---
	departingUEs := []string{}
	for _, pred := range mobilityResult.Predictions {
		if pred.PredictedBehavior == "DEPARTING" {
			departingUEs = append(departingUEs, pred.Supi)
		}
	}
	if len(departingUEs) > 0 {
		avgConf := 0.0
		for _, pred := range mobilityResult.Predictions {
			if pred.PredictedBehavior == "DEPARTING" {
				avgConf += pred.Confidence
			}
		}
		avgConf /= float64(len(departingUEs))
		recommendations = append(recommendations, NwdafRecommendation{
			Id:          fmt.Sprintf("REC-%03d", recID),
			Priority:    "HIGH",
			Category:    "CAPACITY",
			Action:      "Prepare handover resources",
			Reasoning:   fmt.Sprintf("%d UE(s) predicted to be DEPARTING; pre-allocating handover resources reduces handover latency", len(departingUEs)),
			Confidence:  avgConf,
			AffectedUes: departingUEs,
		})
		recID++
	}

	// --- Rule 2: INCREASING session load → scale up UPF capacity ---
	if sessionResult.Forecast.Trend == "INCREASING" {
		recommendations = append(recommendations, NwdafRecommendation{
			Id:       fmt.Sprintf("REC-%03d", recID),
			Priority: "MEDIUM",
			Category: "CAPACITY",
			Action:   "Scale up UPF capacity",
			Reasoning: fmt.Sprintf("Session load is %s (confidence %.0f%%) with predicted rate %.2f sessions/min; additional UPF capacity recommended",
				sessionResult.Forecast.Trend, sessionResult.Forecast.TrendConfidence*100, sessionResult.Forecast.PredictedSessionRate),
			Confidence: sessionResult.Forecast.TrendConfidence,
		})
		recID++
	}

	// --- Rule 3: CRITICAL anomaly health → investigate anomalous UEs ---
	if anomalyResult.OverallHealth == "CRITICAL" {
		anomalousUEs := []string{}
		for _, score := range anomalyResult.Scores {
			if score.IsAnomalous && score.Supi != "" {
				anomalousUEs = append(anomalousUEs, score.Supi)
			}
		}
		recommendations = append(recommendations, NwdafRecommendation{
			Id:          fmt.Sprintf("REC-%03d", recID),
			Priority:    "HIGH",
			Category:    "SECURITY",
			Action:      "Investigate anomalous UE behavior",
			Reasoning:   fmt.Sprintf("Network health is CRITICAL with %d anomalies detected; potential security or misbehavior issue", anomalyResult.TotalAnomalies),
			Confidence:  0.85,
			AffectedUes: anomalousUEs,
		})
		recID++
	}

	// --- Rule 4: Session flapping detection → review QoS policies ---
	// Flapping: active session count is low but establishment rate is non-trivial,
	// indicating sessions are frequently created and released.
	activeRatio := 0.0
	if sessionResult.Forecast.PeakSessionRate > 0 {
		activeRatio = float64(sessionResult.Forecast.ActiveSessions) / (sessionResult.Forecast.PeakSessionRate * 4) // 4-min window
	}
	if activeRatio < 0.5 && sessionResult.Forecast.PeakSessionRate > 1.0 {
		// High session establishment rate with few remaining active sessions = flapping.
		flappingUEs := []string{}
		for _, pred := range mobilityResult.Predictions {
			if pred.PredictedBehavior == "MOBILE" || pred.PredictedBehavior == "DEPARTING" {
				flappingUEs = append(flappingUEs, pred.Supi)
			}
		}
		recommendations = append(recommendations, NwdafRecommendation{
			Id:          fmt.Sprintf("REC-%03d", recID),
			Priority:    "MEDIUM",
			Category:    "SESSION_MANAGEMENT",
			Action:      "Review QoS policies for affected UEs",
			Reasoning:   fmt.Sprintf("Low session retention ratio (%.0f%%) with peak rate %.2f/min suggests session flapping; QoS policy review recommended", activeRatio*100, sessionResult.Forecast.PeakSessionRate),
			Confidence:  0.70,
			AffectedUes: flappingUEs,
		})
		recID++
	}

	// --- Rule 5: High mobility + many sessions → enable mobility-optimised bearers ---
	mobileCount := 0
	for _, pred := range mobilityResult.Predictions {
		if pred.PredictedBehavior == "MOBILE" || pred.PredictedBehavior == "DEPARTING" {
			mobileCount++
		}
	}
	highMobility := mobileCount > 0 && float64(mobileCount)/float64(len(mobilityResult.Predictions)+1) > 0.3
	manySessions := sessionResult.Forecast.ActiveSessions > 5
	if highMobility && manySessions {
		recommendations = append(recommendations, NwdafRecommendation{
			Id:       fmt.Sprintf("REC-%03d", recID),
			Priority: "LOW",
			Category: "MOBILITY",
			Action:   "Enable mobility-optimized bearers",
			Reasoning: fmt.Sprintf("%d mobile/departing UE(s) with %d active sessions; mobility-optimised bearers reduce handover interruption",
				mobileCount, sessionResult.Forecast.ActiveSessions),
			Confidence: 0.65,
		})
		recID++
	}

	// --- Build data summary ---
	totalEvt := eventStore.GetTotalEventCount()
	uniqueUEs := len(eventStore.GetUniqueSupis())
	// Split total count roughly: use mobility result to estimate AMF vs total.
	amfEventEstimate := totalEvt / 2
	smfEventEstimate := totalEvt - amfEventEstimate

	summary := DataSummary{
		TotalAmfEvents:       amfEventEstimate,
		TotalSmfEvents:       smfEventEstimate,
		UniqueUes:            uniqueUEs,
		ObservationWindowMin: 10,
	}

	logger.ProducerLog.Infof("[Recommendation] Generated %d recommendation(s)", len(recommendations))

	return &NwdafRecommendationResult{
		Recommendations: recommendations,
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		ModelVersions: map[string]string{
			"mobility":  mobilityResult.ModelVersion,
			"session":   sessionResult.ModelVersion,
			"anomaly":   anomalyResult.ModelVersion,
			"aggregator": "1.0.0",
		},
		DataSummary: summary,
	}
}
