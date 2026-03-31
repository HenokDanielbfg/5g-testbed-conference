package consumer

import (
	"fmt"
	"net/http"
	"sync"

	nwdaf_context "github.com/free5gc/nwdaf/internal/context"
	"github.com/free5gc/nwdaf/internal/logger"
	"github.com/free5gc/openapi/Nsmf_EventExposure"
	"github.com/free5gc/openapi/models"
)

type nsmfService struct {
	consumer   *Consumer
	mu         sync.RWMutex
	smfClients map[string]*Nsmf_EventExposure.APIClient
}

func (s *nsmfService) getSmfEventClient(uri string) *Nsmf_EventExposure.APIClient {
	if uri == "" {
		return nil
	}
	s.mu.RLock()
	client, ok := s.smfClients[uri]
	if ok {
		s.mu.RUnlock()
		return client
	}
	s.mu.RUnlock()

	configuration := Nsmf_EventExposure.NewConfiguration()
	configuration.SetBasePath(uri)
	client = Nsmf_EventExposure.NewAPIClient(configuration)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.smfClients[uri] = client
	return client
}

func (s *nsmfService) SubscribeSmfEvents(smfUri string, events []string) (string, error) {
	nwdafCtx := nwdaf_context.GetSelf()
	client := s.getSmfEventClient(smfUri)
	if client == nil {
		return "", fmt.Errorf("SMF client not found for URI: %s", smfUri)
	}

	callbackUri := nwdafCtx.GetIPv4Uri() + "/nnwdaf-callback/v1/smf-event-notify"

	eventSubs := make([]models.EventSubscription, 0, len(events))
	for _, evt := range events {
		eventSubs = append(eventSubs, models.EventSubscription{
			Event: models.SmfEvent(evt),
		})
	}

	subReq := models.NsmfEventExposure{
		NotifId:  "nwdaf-smf-" + nwdafCtx.NfId,
		NotifUri: callbackUri,
		AnyUeInd: true,
		EventSubs: eventSubs,
	}

	logger.ConsumerLog.Infof("[SMF] Subscribing to events %v at %s", events, smfUri)

	ctx, pd, err := nwdaf_context.GetSelf().GetTokenCtx(models.ServiceName_NSMF_EVENT_EXPOSURE, models.NfType_SMF)
	if err != nil {
		return "", fmt.Errorf("SMF token error: %v, problem: %v", err, pd)
	}

	created, res, err := client.DefaultApi.SubscriptionsPost(ctx, subReq)
	if err != nil {
		return "", fmt.Errorf("SMF event subscription failed: %v", err)
	}
	if res != nil {
		defer res.Body.Close()
		if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusOK {
			return "", fmt.Errorf("SMF event subscription returned status %d", res.StatusCode)
		}
	}

	subscriptionId := created.SubId
	nwdafCtx.SmfSubscriptionId = subscriptionId
	logger.ConsumerLog.Infof("[SMF] Event subscription created, id=%s", subscriptionId)

	return subscriptionId, nil
}

func (s *nsmfService) UnsubscribeSmfEvents(smfUri string, subscriptionId string) error {
	client := s.getSmfEventClient(smfUri)
	if client == nil {
		return fmt.Errorf("SMF client not found for URI: %s", smfUri)
	}

	logger.ConsumerLog.Infof("[SMF] Unsubscribing event subscription id=%s", subscriptionId)

	ctx, _, err := nwdaf_context.GetSelf().GetTokenCtx(models.ServiceName_NSMF_EVENT_EXPOSURE, models.NfType_SMF)
	if err != nil {
		return fmt.Errorf("SMF token error: %v", err)
	}

	res, err := client.DefaultApi.SubscriptionsSubIdDelete(ctx, subscriptionId)
	if err != nil {
		return fmt.Errorf("SMF event unsubscribe failed: %v", err)
	}
	if res != nil {
		defer res.Body.Close()
	}

	nwdaf_context.GetSelf().SmfSubscriptionId = ""
	logger.ConsumerLog.Infof("[SMF] Event subscription deleted, id=%s", subscriptionId)
	return nil
}
