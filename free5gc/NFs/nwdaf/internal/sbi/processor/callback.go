package processor

import (
	"time"

	nwdaf_context "github.com/free5gc/nwdaf/internal/context"
	"github.com/free5gc/nwdaf/internal/logger"
	"github.com/free5gc/openapi/models"
)

func (p *Processor) HandleAmfEventNotification(notification *models.AmfEventNotification) {
	logger.CallbackLog.Infof("[AMF] Processing event notification, correlationId=%s, reports=%d",
		notification.NotifyCorrelationId, len(notification.ReportList))

	nwdafCtx := p.Context()

	for _, report := range notification.ReportList {
		eventType := string(report.Type)
		logger.CallbackLog.Infof("[AMF] Event: %s, State: %+v", eventType, report.State)

		received := nwdaf_context.ReceivedAmfEvent{
			EventType:           eventType,
			Timestamp:           time.Now(),
			NotifyCorrelationId: notification.NotifyCorrelationId,
			Supi:                report.Supi,
			Location:            report.Location,
			Timezone:            report.Timezone,
			Reachability:        string(report.Reachability),
			CmInfoList:          report.CmInfoList,
			RmInfoList:          report.RmInfoList,
		}

		// Capture State.Active for REGISTRATION_STATE_REPORT
		// (AMF uses report.State.Active instead of RmInfoList)
		if report.State != nil {
			active := report.State.Active
			received.StateActive = &active
		}

		// Log location data if present
		if report.Location != nil {
			logger.CallbackLog.Infof("[AMF] Location report for event %s: %+v", eventType, report.Location)
		}

		// Log registration state if present
		if len(report.RmInfoList) > 0 {
			logger.CallbackLog.Infof("[AMF] Registration state report: %+v", report.RmInfoList)
		}

		// Log connectivity state if present
		if len(report.CmInfoList) > 0 {
			logger.CallbackLog.Infof("[AMF] Connectivity state report: %+v", report.CmInfoList)
		}

		// Log reachability if present
		if report.Reachability != "" {
			logger.CallbackLog.Infof("[AMF] Reachability report: %s", report.Reachability)
		}

		// Handle timezone report
		if report.Type == models.AmfEventType_TIMEZONE_REPORT && report.Timezone != "" {
			logger.CallbackLog.Infof("[AMF] Timezone report for SUPI=%s: %s", report.Supi, report.Timezone)
		}

		// Handle access type report
		if report.Type == models.AmfEventType_ACCESS_TYPE_REPORT && len(report.AccessTypeList) > 0 {
			logger.CallbackLog.Infof("[AMF] Access type report for SUPI=%s: %+v", report.Supi, report.AccessTypeList)
		}

		// Handle subscribed data report
		if report.Type == models.AmfEventType_SUBSCRIBED_DATA_REPORT && report.SubscribedData != nil {
			logger.CallbackLog.Infof("[AMF] Subscribed data report for SUPI=%s: %+v", report.Supi, report.SubscribedData)
		}

		// Store the latest event by type (backward compat)
		nwdafCtx.AmfEvents.Store(eventType, &received)

		// Store in event history
		nwdafCtx.EventStore.AddAmfEvent(received)
		logger.CallbackLog.Infof("[AMF] Stored event %s (total=%d)", eventType, nwdafCtx.EventStore.GetTotalEventCount())
	}
}

func (p *Processor) HandleSmfEventNotification(notification *models.NsmfEventExposureNotification) {
	logger.CallbackLog.Infof("[SMF] Processing event notification, notifId=%s, events=%d",
		notification.NotifId, len(notification.EventNotifs))

	nwdafCtx := p.Context()

	for _, eventNotif := range notification.EventNotifs {
		eventType := string(eventNotif.Event)
		logger.CallbackLog.Infof("[SMF] Event: %s, SUPI: %s, PduSessionId: %d",
			eventType, eventNotif.Supi, eventNotif.PduSeId)

		received := nwdaf_context.ReceivedSmfEvent{
			EventType:  eventType,
			Timestamp:  time.Now(),
			NotifId:    notification.NotifId,
			Supi:       eventNotif.Supi,
			PduSeId:    eventNotif.PduSeId,
			SourceDnai: eventNotif.SourceDnai,
			TargetDnai: eventNotif.TargetDnai,
			UeIpv4:     eventNotif.TargetUeIpv4Addr,
		}

		// Handle UP path change
		if eventNotif.Event == models.SmfEvent_UP_PATH_CH {
			logger.CallbackLog.Infof("[SMF] UP path change for SUPI=%s: sourceDnai=%s targetDnai=%s",
				eventNotif.Supi, eventNotif.SourceDnai, eventNotif.TargetDnai)
		}

		// Handle UE IP change
		if eventNotif.Event == models.SmfEvent_UE_IP_CH {
			logger.CallbackLog.Infof("[SMF] UE IP change for SUPI=%s: targetIpv4=%s",
				eventNotif.Supi, eventNotif.TargetUeIpv4Addr)
		}

		// Store the latest event by type (backward compat)
		nwdafCtx.SmfEvents.Store(eventType, &received)

		// Store in event history
		nwdafCtx.EventStore.AddSmfEvent(received)
		logger.CallbackLog.Infof("[SMF] Stored event %s for SUPI=%s (total=%d)",
			eventType, eventNotif.Supi, nwdafCtx.EventStore.GetTotalEventCount())
	}
}
