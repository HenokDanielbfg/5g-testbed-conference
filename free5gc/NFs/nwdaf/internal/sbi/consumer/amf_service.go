package consumer

import (
	"fmt"
	"net/http"
	"sync"

	nwdaf_context "github.com/free5gc/nwdaf/internal/context"
	"github.com/free5gc/nwdaf/internal/logger"
	"github.com/free5gc/openapi/Namf_EventExposure"
	"github.com/free5gc/openapi/models"
)

type namfService struct {
	consumer   *Consumer
	mu         sync.RWMutex
	amfClients map[string]*Namf_EventExposure.APIClient
}

func (s *namfService) getAmfEventClient(uri string) *Namf_EventExposure.APIClient {
	if uri == "" {
		return nil
	}
	s.mu.RLock()
	client, ok := s.amfClients[uri]
	if ok {
		s.mu.RUnlock()
		return client
	}
	s.mu.RUnlock()

	configuration := Namf_EventExposure.NewConfiguration()
	configuration.SetBasePath(uri)
	client = Namf_EventExposure.NewAPIClient(configuration)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.amfClients[uri] = client
	return client
}

func (s *namfService) SubscribeAmfEvents(amfUri string, events []string) (string, error) {
	nwdafCtx := nwdaf_context.GetSelf()
	client := s.getAmfEventClient(amfUri)
	if client == nil {
		return "", fmt.Errorf("AMF client not found for URI: %s", amfUri)
	}

	callbackUri := nwdafCtx.GetIPv4Uri() + "/nnwdaf-callback/v1/amf-event-notify"

	eventList := make([]models.AmfEvent, 0, len(events))
	for _, evt := range events {
		eventList = append(eventList, models.AmfEvent{
			Type:          models.AmfEventType(evt),
			ImmediateFlag: false,
		})
	}

	createReq := models.AmfCreateEventSubscription{
		Subscription: &models.AmfEventSubscription{
			EventList:           &eventList,
			EventNotifyUri:      callbackUri,
			NotifyCorrelationId: "nwdaf-amf-" + nwdafCtx.NfId,
			NfId:                nwdafCtx.NfId,
			AnyUE:               true,
			Options: &models.AmfEventMode{
				Trigger: models.AmfEventTrigger_CONTINUOUS,
			},
		},
	}

	logger.ConsumerLog.Infof("[AMF] Subscribing to events %v at %s", events, amfUri)

	ctx, pd, err := nwdaf_context.GetSelf().GetTokenCtx(models.ServiceName_NAMF_EVTS, models.NfType_AMF)
	if err != nil {
		return "", fmt.Errorf("AMF token error: %v, problem: %v", err, pd)
	}

	created, res, err := client.SubscriptionsCollectionDocumentApi.CreateSubscription(
		ctx, createReq)
	if err != nil {
		return "", fmt.Errorf("AMF event subscription failed: %v", err)
	}
	if res != nil {
		defer res.Body.Close()
		if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusOK {
			return "", fmt.Errorf("AMF event subscription returned status %d", res.StatusCode)
		}
	}

	subscriptionId := created.SubscriptionId
	nwdafCtx.AmfSubscriptionId = subscriptionId
	logger.ConsumerLog.Infof("[AMF] Event subscription created, id=%s", subscriptionId)

	if len(created.ReportList) > 0 {
		logger.ConsumerLog.Infof("[AMF] Received %d immediate reports", len(created.ReportList))
	}

	return subscriptionId, nil
}

func (s *namfService) UnsubscribeAmfEvents(amfUri string, subscriptionId string) error {
	client := s.getAmfEventClient(amfUri)
	if client == nil {
		return fmt.Errorf("AMF client not found for URI: %s", amfUri)
	}

	logger.ConsumerLog.Infof("[AMF] Unsubscribing event subscription id=%s", subscriptionId)

	ctx, _, err := nwdaf_context.GetSelf().GetTokenCtx(models.ServiceName_NAMF_EVTS, models.NfType_AMF)
	if err != nil {
		return fmt.Errorf("AMF token error: %v", err)
	}

	res, err := client.IndividualSubscriptionDocumentApi.DeleteSubscription(
		ctx, subscriptionId)
	if err != nil {
		return fmt.Errorf("AMF event unsubscribe failed: %v", err)
	}
	if res != nil {
		defer res.Body.Close()
	}

	nwdaf_context.GetSelf().AmfSubscriptionId = ""
	logger.ConsumerLog.Infof("[AMF] Event subscription deleted, id=%s", subscriptionId)
	return nil
}
