package processor

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	nwdaf_context "github.com/free5gc/nwdaf/internal/context"
	"github.com/free5gc/nwdaf/internal/logger"
	"github.com/free5gc/openapi/models"
)


// NwdafAnalyticsData is the top-level response body for analytics queries.
type NwdafAnalyticsData struct {
	Start                      string                      `json:"start,omitempty"`
	Expiry                     string                      `json:"expiry,omitempty"`
	NfLoadLevelInfo            []NfLoadLevelInfo           `json:"nfLoadLevelInfo,omitempty"`
	NetworkPerfInfo            []NetworkPerfInfo           `json:"networkPerfInfo,omitempty"`
	SliceLoadLevel             int32                       `json:"sliceLoadLevel,omitempty"`
	UeMobilityAnalytics        *UeMobilityAnalytics        `json:"ueMobilityAnalytics,omitempty"`
	UeCommunicationAnalytics   *UeCommunicationAnalytics   `json:"ueCommunicationAnalytics,omitempty"`
	AbnormalBehaviourAnalytics *AbnormalBehaviourAnalytics `json:"abnormalBehaviourAnalytics,omitempty"`
	RecommendationResult       *NwdafRecommendationResult  `json:"recommendationResult,omitempty"`
}

// NfLoadLevelInfo carries per-NF load analytics.
type NfLoadLevelInfo struct {
	NfType          string  `json:"nfType"`
	NfLoad          int32   `json:"nfLoad"`               // 0-100 percentage
	NfCpuLoad       int32   `json:"nfCpuLoad,omitempty"`
	NfMemLoad       int32   `json:"nfMemLoad,omitempty"`
	EventRate       float64 `json:"eventRate,omitempty"`       // events per minute from collected data
	ActiveUeCount   int     `json:"activeUeCount,omitempty"`   // unique UEs seen in events
	CollectedEvents int     `json:"collectedEvents,omitempty"` // total events collected
}

// NetworkPerfInfo carries network performance analytics.
type NetworkPerfInfo struct {
	NetworkPerfType string `json:"networkPerfType"`
	RelativeRatio   int32  `json:"relativeRatio,omitempty"`
	AbsoluteNum     int32  `json:"absoluteNum,omitempty"`
}

// UeMobilityAnalytics holds UE mobility analytics derived from AMF location events.
type UeMobilityAnalytics struct {
	TotalLocationReports int                `json:"totalLocationReports"`
	UniqueUes            int                `json:"uniqueUes"`
	UeTrajectories       []UeTrajectoryInfo `json:"ueTrajectories,omitempty"`
	MobilityStats        MobilityStats      `json:"mobilityStats"`
}

// UeTrajectoryInfo holds per-UE mobility information.
type UeTrajectoryInfo struct {
	Supi            string `json:"supi"`
	LocationChanges int    `json:"locationChanges"`
	LastReportTime  string `json:"lastReportTime"`
	IsRegistered    bool   `json:"isRegistered"`
	IsConnected     bool   `json:"isConnected"`
}

// MobilityStats holds aggregate mobility statistics.
type MobilityStats struct {
	AvgLocationChangesPerUe float64 `json:"avgLocationChangesPerUe"`
	EventRatePerMinute      float64 `json:"eventRatePerMinute"`
	ActiveUeCount           int     `json:"activeUeCount"`
}

// UeCommunicationAnalytics holds PDU session analytics derived from SMF events.
type UeCommunicationAnalytics struct {
	TotalSessionEstablishments int                `json:"totalSessionEstablishments"`
	TotalSessionReleases       int                `json:"totalSessionReleases"`
	ActiveSessions             int                `json:"activeSessions"`
	UniqueUes                  int                `json:"uniqueUes"`
	SessionStats               []UeSessionInfo    `json:"sessionStats,omitempty"`
	CommunicationStats         CommunicationStats `json:"communicationStats"`
}

// UeSessionInfo holds per-UE PDU session information.
type UeSessionInfo struct {
	Supi             string `json:"supi"`
	SessionsCreated  int    `json:"sessionsCreated"`
	SessionsReleased int    `json:"sessionsReleased"`
	ActiveSessions   int    `json:"activeSessions"`
	LastActivityTime string `json:"lastActivityTime"`
}

// CommunicationStats holds aggregate PDU session statistics.
type CommunicationStats struct {
	SessionEstRate   float64 `json:"sessionEstRatePerMinute"`
	SessionRelRate   float64 `json:"sessionRelRatePerMinute"`
	AvgSessionsPerUe float64 `json:"avgSessionsPerUe"`
}

// AbnormalBehaviourAnalytics holds anomaly detection results.
type AbnormalBehaviourAnalytics struct {
	TotalAnomalies     int             `json:"totalAnomalies"`
	AnomalyReports     []AnomalyReport `json:"anomalyReports,omitempty"`
	AnalysisWindow     string          `json:"analysisWindow"`
	BaselineEpochs     int             `json:"baselineEpochs"`               // number of epochs baseline is derived from
	BaselineConfidence string          `json:"baselineConfidence"`            // NONE, LOW, MEDIUM, HIGH
	BaselineAgeSec     int             `json:"baselineAgeSec,omitempty"`      // age of oldest epoch in seconds
	BaselineMean       float64         `json:"baselineMean,omitempty"`        // mean unique SUPIs per epoch
	BaselineStdDev     float64         `json:"baselineStdDev,omitempty"`      // stddev unique SUPIs per epoch
}

// AnomalyReport describes a single detected anomaly.
type AnomalyReport struct {
	Supi          string  `json:"supi,omitempty"`
	AnomalyType   string  `json:"anomalyType"`   // "RAPID_REREGISTRATION", "SESSION_FLAPPING", "CONNECTIVITY_FLAPPING"
	Severity      string  `json:"severity"`      // "LOW", "MEDIUM", "HIGH"
	Score         float64 `json:"score"`         // 0.0 - 1.0
	Description   string  `json:"description"`
	EventCount    int     `json:"eventCount"`
	TimeWindowSec int     `json:"timeWindowSec"`
}

// HandleGetAnalyticsInfo processes GET /nnwdaf-analyticsinfo/v1/analytics.
// analyticsId identifies the event type (e.g. "NF_LOAD", "NETWORK_PERFORMANCE").
// queryParams carries the raw query-string parameters forwarded from the SBI layer.
func (p *Processor) HandleGetAnalyticsInfo(
	analyticsId string,
	queryParams map[string][]string,
) (*NwdafAnalyticsData, *models.ProblemDetails) {
	logger.ProducerLog.Infof("[Analytics] HandleGetAnalyticsInfo: analyticsId=%s", analyticsId)

	now := time.Now().UTC()
	start := now.Format(time.RFC3339)
	expiry := now.Add(time.Hour).Format(time.RFC3339)

	switch analyticsId {
	case nwdaf_context.NwdafEvent_NF_LOAD:
		logger.ProducerLog.Infof("[Analytics] Returning NF load level info")
		return p.buildNfLoadAnalytics(start, expiry), nil

	case nwdaf_context.NwdafEvent_NETWORK_PERFORMANCE:
		logger.ProducerLog.Infof("[Analytics] Returning network performance info")
		data := &NwdafAnalyticsData{
			Start:  start,
			Expiry: expiry,
			NetworkPerfInfo: []NetworkPerfInfo{
				{
					NetworkPerfType: "GNB_ACTIVE_RATIO",
					RelativeRatio:   randomLoad(),
					AbsoluteNum:     int32(rand.Intn(500) + 50),
				},
				{
					NetworkPerfType: "GNB_COMPUTING_USAGE",
					RelativeRatio:   randomLoad(),
					AbsoluteNum:     int32(rand.Intn(200) + 10),
				},
			},
		}
		return data, nil

	case nwdaf_context.NwdafEvent_SLICE_LOAD_LEVEL:
		logger.ProducerLog.Infof("[Analytics] Returning slice load level info")
		data := &NwdafAnalyticsData{
			Start:          start,
			Expiry:         expiry,
			SliceLoadLevel: randomLoad(),
		}
		return data, nil

	case nwdaf_context.NwdafEvent_UE_MOBILITY:
		logger.ProducerLog.Infof("[Analytics] Returning UE mobility analytics")
		return p.buildUeMobilityAnalytics(start, expiry), nil

	case nwdaf_context.NwdafEvent_UE_COMMUNICATION:
		logger.ProducerLog.Infof("[Analytics] Returning UE communication analytics")
		return p.buildUeCommunicationAnalytics(start, expiry), nil

	case nwdaf_context.NwdafEvent_ABNORMAL_BEHAVIOUR:
		logger.ProducerLog.Infof("[Analytics] Returning abnormal behaviour analytics")
		return p.buildAbnormalBehaviourAnalytics(start, expiry), nil

	case nwdaf_context.NwdafEvent_RECOMMENDATION:
		logger.ProducerLog.Infof("[Analytics] Returning NWDAF recommendations")
		store := p.Context().EventStore
		if store == nil {
			logger.ProducerLog.Warnf("[Analytics] EventStore not available; returning empty recommendation data")
			return &NwdafAnalyticsData{Start: start, Expiry: expiry}, nil
		}
		recResult := p.GenerateRecommendations(store)
		return &NwdafAnalyticsData{
			Start:                start,
			Expiry:               expiry,
			RecommendationResult: recResult,
		}, nil

	case nwdaf_context.NwdafEvent_QOS_SUSTAINABILITY:
		// Recognised event not yet implemented; return empty data set.
		logger.ProducerLog.Infof("[Analytics] Event %s recognised but not yet implemented; returning empty data", analyticsId)
		data := &NwdafAnalyticsData{
			Start:  start,
			Expiry: expiry,
		}
		return data, nil

	default:
		logger.ProducerLog.Warnf("[Analytics] Unknown analyticsId: %s", analyticsId)
		return nil, &models.ProblemDetails{
			Status: 400,
			Title:  "Bad Request",
			Detail: "Unknown analytics event: " + analyticsId,
		}
	}
}

// memRefMB is the reference RSS used to normalise memory load to a 0–100 percentage.
// A Go 5G NF at rest uses roughly 50–150 MB; 512 MB gives comfortable headroom.
const memRefMB = 512

// buildNfLoadAnalytics builds NF_LOAD analytics from real /proc metrics (CPU, memory)
// combined with EventStore-derived signalling metrics (event rate, active UE count).
func (p *Processor) buildNfLoadAnalytics(start, expiry string) *NwdafAnalyticsData {
	proc := p.Context().ProcMetrics

	amfInfo := nfLoadFromProc(proc, "amf", "AMF")
	smfInfo := nfLoadFromProc(proc, "smf", "SMF")
	upfInfo := nfLoadFromProc(proc, "upf", "UPF")

	store := p.Context().EventStore
	if store != nil {
		// AMF: event rate and active UE count from location reports.
		amfEvents := store.GetAmfEventsByType("LOCATION_REPORT")
		amfInfo.CollectedEvents = len(amfEvents)
		amfInfo.EventRate = store.GetEventRate("LOCATION_REPORT", time.Minute) * 60
		amfInfo.ActiveUeCount = len(uniqueSupisFromAmfEvents(amfEvents))

		// SMF: event rate and active UE count from PDU session events.
		pduEstEvents := store.GetSmfEventsByType("PDU_SES_EST")
		pduRelEvents := store.GetSmfEventsByType("PDU_SES_REL")
		smfInfo.CollectedEvents = len(pduEstEvents) + len(pduRelEvents)
		smfInfo.EventRate = store.GetEventRate("PDU_SES_EST", time.Minute)*60 +
			store.GetEventRate("PDU_SES_REL", time.Minute)*60
		smfInfo.ActiveUeCount = len(uniqueSupisFromSmfEvents(append(pduEstEvents, pduRelEvents...)))
	}

	return &NwdafAnalyticsData{
		Start:  start,
		Expiry: expiry,
		NfLoadLevelInfo: []NfLoadLevelInfo{
			amfInfo,
			smfInfo,
			upfInfo,
		},
	}
}

// nfLoadFromProc builds an NfLoadLevelInfo from live /proc data.
// If the NF process is not found (not running), all load fields are zero.
func nfLoadFromProc(proc *nwdaf_context.ProcMetricsCollector, procName, nfType string) NfLoadLevelInfo {
	info := NfLoadLevelInfo{NfType: nfType}
	if proc == nil {
		return info
	}
	m := proc.Get(procName)
	if m == nil {
		return info // NF not running
	}
	cpuPct := int32(math.Round(m.CPUPercent))
	memPct := m.MemLoadPercent(memRefMB)
	// Composite load: average of CPU and memory utilisation.
	info.NfLoad = (cpuPct + memPct) / 2
	info.NfCpuLoad = cpuPct
	info.NfMemLoad = memPct
	return info
}

// buildUeMobilityAnalytics builds UE_MOBILITY analytics from real EventStore data (Task #4).
func (p *Processor) buildUeMobilityAnalytics(start, expiry string) *NwdafAnalyticsData {
	store := p.Context().EventStore
	if store == nil {
		logger.ProducerLog.Warnf("[Analytics] EventStore not available; returning empty UE mobility data")
		return &NwdafAnalyticsData{Start: start, Expiry: expiry}
	}

	locationEvents := store.GetAmfEventsByType("LOCATION_REPORT")
	regEvents := store.GetAmfEventsByType("REGISTRATION_STATE_REPORT")
	connEvents := store.GetAmfEventsByType("CONNECTIVITY_STATE_REPORT")

	// Build per-UE maps.
	locationCountBySupi := make(map[string]int)
	lastReportBySupi := make(map[string]time.Time)
	registeredSupis := make(map[string]bool)
	connectedSupis := make(map[string]bool)

	for _, ev := range locationEvents {
		if ev.Supi == "" {
			continue
		}
		locationCountBySupi[ev.Supi]++
		if ev.Timestamp.After(lastReportBySupi[ev.Supi]) {
			lastReportBySupi[ev.Supi] = ev.Timestamp
		}
	}

	for _, ev := range regEvents {
		if ev.Supi == "" {
			continue
		}
		// Check StateActive first (AMF sends State.Active for REGISTRATION_STATE_REPORT)
		if ev.StateActive != nil {
			registeredSupis[ev.Supi] = *ev.StateActive
		}
		// Fall back to RmInfoList if populated
		for _, rmInfo := range ev.RmInfoList {
			if rmInfo.RmState == models.RmState_REGISTERED {
				registeredSupis[ev.Supi] = true
			} else if rmInfo.RmState == models.RmState_DEREGISTERED {
				registeredSupis[ev.Supi] = false
			}
		}
	}

	for _, ev := range connEvents {
		if ev.Supi == "" {
			continue
		}
		for _, cmInfo := range ev.CmInfoList {
			if cmInfo.CmState == models.CmState_CONNECTED {
				connectedSupis[ev.Supi] = true
			}
		}
	}

	// Build trajectory list.
	trajectories := make([]UeTrajectoryInfo, 0, len(locationCountBySupi))
	totalChanges := 0
	for supi, count := range locationCountBySupi {
		lastTime := ""
		if !lastReportBySupi[supi].IsZero() {
			lastTime = lastReportBySupi[supi].Format(time.RFC3339)
		}
		trajectories = append(trajectories, UeTrajectoryInfo{
			Supi:            supi,
			LocationChanges: count,
			LastReportTime:  lastTime,
			IsRegistered:    registeredSupis[supi],
			IsConnected:     connectedSupis[supi],
		})
		totalChanges += count
	}

	uniqueUes := len(locationCountBySupi)
	var avgChanges float64
	if uniqueUes > 0 {
		avgChanges = float64(totalChanges) / float64(uniqueUes)
	}

	eventRatePerMin := store.GetEventRate("LOCATION_REPORT", time.Minute) * 60

	// Count active UEs (connected).
	activeCount := 0
	for _, traj := range trajectories {
		if traj.IsConnected {
			activeCount++
		}
	}

	analytics := &UeMobilityAnalytics{
		TotalLocationReports: len(locationEvents),
		UniqueUes:            uniqueUes,
		UeTrajectories:       trajectories,
		MobilityStats: MobilityStats{
			AvgLocationChangesPerUe: avgChanges,
			EventRatePerMinute:      eventRatePerMin,
			ActiveUeCount:           activeCount,
		},
	}

	return &NwdafAnalyticsData{
		Start:               start,
		Expiry:              expiry,
		UeMobilityAnalytics: analytics,
	}
}

// buildUeCommunicationAnalytics builds UE_COMMUNICATION analytics from SMF PDU session events (Task #5).
func (p *Processor) buildUeCommunicationAnalytics(start, expiry string) *NwdafAnalyticsData {
	store := p.Context().EventStore
	if store == nil {
		logger.ProducerLog.Warnf("[Analytics] EventStore not available; returning empty UE communication data")
		return &NwdafAnalyticsData{Start: start, Expiry: expiry}
	}

	estEvents := store.GetSmfEventsByType("PDU_SES_EST")
	relEvents := store.GetSmfEventsByType("PDU_SES_REL")

	// Build per-UE maps.
	estBySupi := make(map[string]int)
	relBySupi := make(map[string]int)
	lastActivityBySupi := make(map[string]time.Time)

	for _, ev := range estEvents {
		if ev.Supi == "" {
			continue
		}
		estBySupi[ev.Supi]++
		if ev.Timestamp.After(lastActivityBySupi[ev.Supi]) {
			lastActivityBySupi[ev.Supi] = ev.Timestamp
		}
	}
	for _, ev := range relEvents {
		if ev.Supi == "" {
			continue
		}
		relBySupi[ev.Supi]++
		if ev.Timestamp.After(lastActivityBySupi[ev.Supi]) {
			lastActivityBySupi[ev.Supi] = ev.Timestamp
		}
	}

	// Collect all unique SUPIs.
	supiSet := make(map[string]struct{})
	for supi := range estBySupi {
		supiSet[supi] = struct{}{}
	}
	for supi := range relBySupi {
		supiSet[supi] = struct{}{}
	}

	sessionStats := make([]UeSessionInfo, 0, len(supiSet))
	totalActiveSessions := 0
	for supi := range supiSet {
		est := estBySupi[supi]
		rel := relBySupi[supi]
		active := est - rel
		if active < 0 {
			active = 0
		}
		totalActiveSessions += active

		lastActivity := ""
		if !lastActivityBySupi[supi].IsZero() {
			lastActivity = lastActivityBySupi[supi].Format(time.RFC3339)
		}
		sessionStats = append(sessionStats, UeSessionInfo{
			Supi:             supi,
			SessionsCreated:  est,
			SessionsReleased: rel,
			ActiveSessions:   active,
			LastActivityTime: lastActivity,
		})
	}

	uniqueUes := len(supiSet)
	estRatePerMin := store.GetEventRate("PDU_SES_EST", time.Minute) * 60
	relRatePerMin := store.GetEventRate("PDU_SES_REL", time.Minute) * 60

	var avgSessionsPerUe float64
	if uniqueUes > 0 {
		avgSessionsPerUe = float64(len(estEvents)) / float64(uniqueUes)
	}

	analytics := &UeCommunicationAnalytics{
		TotalSessionEstablishments: len(estEvents),
		TotalSessionReleases:       len(relEvents),
		ActiveSessions:             totalActiveSessions,
		UniqueUes:                  uniqueUes,
		SessionStats:               sessionStats,
		CommunicationStats: CommunicationStats{
			SessionEstRate:   estRatePerMin,
			SessionRelRate:   relRatePerMin,
			AvgSessionsPerUe: avgSessionsPerUe,
		},
	}

	return &NwdafAnalyticsData{
		Start:                    start,
		Expiry:                   expiry,
		UeCommunicationAnalytics: analytics,
	}
}

// buildAbnormalBehaviourAnalytics detects anomalies in collected events (Task #6).
func (p *Processor) buildAbnormalBehaviourAnalytics(start, expiry string) *NwdafAnalyticsData {
	const windowSecs = 300 // 5 minutes
	windowDur := time.Duration(windowSecs) * time.Second
	windowStart := time.Now().Add(-windowDur)

	store := p.Context().EventStore
	if store == nil {
		logger.ProducerLog.Warnf("[Analytics] EventStore not available; returning empty abnormal behaviour data")
		return &NwdafAnalyticsData{Start: start, Expiry: expiry}
	}

	regEvents := store.GetAmfEventsByType("REGISTRATION_STATE_REPORT")
	connEvents := store.GetAmfEventsByType("CONNECTIVITY_STATE_REPORT")
	estEvents := store.GetSmfEventsByType("PDU_SES_EST")
	relEvents := store.GetSmfEventsByType("PDU_SES_REL")

	// Count events per UE within the analysis window.
	regCountBySupi := countAmfEventsInWindow(regEvents, windowStart)
	connCountBySupi := countAmfEventsInWindow(connEvents, windowStart)

	// Combine PDU session events per SUPI.
	pduBySupi := make(map[string]int)
	for _, ev := range estEvents {
		if ev.Supi != "" && ev.Timestamp.After(windowStart) {
			pduBySupi[ev.Supi]++
		}
	}
	for _, ev := range relEvents {
		if ev.Supi != "" && ev.Timestamp.After(windowStart) {
			pduBySupi[ev.Supi]++
		}
	}

	var anomalyReports []AnomalyReport

	// Thresholds.
	const regThreshold = 5
	const pduThreshold = 10
	const connThreshold = 8

	for supi, count := range regCountBySupi {
		if count > regThreshold {
			score := math.Min(1.0, float64(count)/float64(2*regThreshold))
			severity := scoreSeverity(score)
			anomalyReports = append(anomalyReports, AnomalyReport{
				Supi:          supi,
				AnomalyType:   "RAPID_REREGISTRATION",
				Severity:      severity,
				Score:         score,
				Description:   "UE is registering/deregistering at an abnormally high rate",
				EventCount:    count,
				TimeWindowSec: windowSecs,
			})
		}
	}

	for supi, count := range pduBySupi {
		if count > pduThreshold {
			score := math.Min(1.0, float64(count)/float64(2*pduThreshold))
			severity := scoreSeverity(score)
			anomalyReports = append(anomalyReports, AnomalyReport{
				Supi:          supi,
				AnomalyType:   "SESSION_FLAPPING",
				Severity:      severity,
				Score:         score,
				Description:   "UE is establishing and releasing PDU sessions at an abnormally high rate",
				EventCount:    count,
				TimeWindowSec: windowSecs,
			})
		}
	}

	for supi, count := range connCountBySupi {
		if count > connThreshold {
			score := math.Min(1.0, float64(count)/float64(2*connThreshold))
			severity := scoreSeverity(score)
			anomalyReports = append(anomalyReports, AnomalyReport{
				Supi:          supi,
				AnomalyType:   "CONNECTIVITY_FLAPPING",
				Severity:      severity,
				Score:         score,
				Description:   "UE is transitioning between CONNECTED and IDLE states at an abnormally high rate",
				EventCount:    count,
				TimeWindowSec: windowSecs,
			})
		}
	}

	// Aggregate REGISTRATION_FLOOD detection from background baseline sampler.
	// Anomalies are pre-computed every epoch and stored; this query reads from
	// that store rather than computing on-demand, so the signal persists even
	// if the query arrives after the narrow current-epoch window has passed.
	baselineEpochs := 0
	baselineConfidence := "NONE"
	baselineAgeSec := 0
	baselineMean := 0.0
	baselineStdDev := 0.0

	if bl := p.Context().Baseline; bl != nil {
		stats := bl.GetRegistrationBaseline()
		baselineEpochs = stats.EpochCount
		baselineConfidence = stats.Confidence
		baselineAgeSec = int(stats.OldestEpochAge.Seconds())
		baselineMean = stats.Mean
		baselineStdDev = stats.StdDev

		// Retrieve anomalies detected within the last 5 minutes.
		recentFloods := bl.GetDetectedAnomalies(5 * time.Minute)
		for _, flood := range recentFloods {
			anomalyReports = append(anomalyReports, AnomalyReport{
				AnomalyType:   flood.AnomalyType,
				Severity:      flood.Severity,
				Score:         flood.Score,
				Description:   fmt.Sprintf("[detected at %s] %s", flood.Timestamp.Format("15:04:05"), flood.Description),
				EventCount:    int(flood.Current),
				TimeWindowSec: int(bl.EpochDuration().Seconds()),
			})
		}
	}

	analytics := &AbnormalBehaviourAnalytics{
		TotalAnomalies:     len(anomalyReports),
		AnomalyReports:     anomalyReports,
		AnalysisWindow:     windowDur.String(),
		BaselineEpochs:     baselineEpochs,
		BaselineConfidence: baselineConfidence,
		BaselineAgeSec:     baselineAgeSec,
		BaselineMean:       baselineMean,
		BaselineStdDev:     baselineStdDev,
	}

	return &NwdafAnalyticsData{
		Start:                      start,
		Expiry:                     expiry,
		AbnormalBehaviourAnalytics: analytics,
	}
}

// scoreSeverity maps a normalized anomaly score to a severity label.
func scoreSeverity(score float64) string {
	switch {
	case score >= 0.75:
		return "HIGH"
	case score >= 0.4:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

// countAmfEventsInWindow counts AMF events per SUPI that occurred after windowStart.
func countAmfEventsInWindow(events []nwdaf_context.ReceivedAmfEvent, windowStart time.Time) map[string]int {
	counts := make(map[string]int)
	for _, ev := range events {
		if ev.Supi != "" && ev.Timestamp.After(windowStart) {
			counts[ev.Supi]++
		}
	}
	return counts
}

// uniqueSupisFromAmfEvents returns a set of unique SUPIs from a slice of AMF events.
func uniqueSupisFromAmfEvents(events []nwdaf_context.ReceivedAmfEvent) map[string]struct{} {
	supis := make(map[string]struct{})
	for _, ev := range events {
		if ev.Supi != "" {
			supis[ev.Supi] = struct{}{}
		}
	}
	return supis
}

// uniqueSupisFromSmfEvents returns a set of unique SUPIs from a slice of SMF events.
func uniqueSupisFromSmfEvents(events []nwdaf_context.ReceivedSmfEvent) map[string]struct{} {
	supis := make(map[string]struct{})
	for _, ev := range events {
		if ev.Supi != "" {
			supis[ev.Supi] = struct{}{}
		}
	}
	return supis
}

// randomLoad returns a pseudo-random integer in the range [20, 80] to simulate
// a realistic load percentage.
func randomLoad() int32 {
	return int32(rand.Intn(61) + 20) // 20 + [0,60] => [20,80]
}
