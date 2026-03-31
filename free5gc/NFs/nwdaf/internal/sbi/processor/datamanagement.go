package processor

import (
	"github.com/free5gc/openapi/models"
)

// NfSubscriptionStatus is returned by HandleGetNfSubscriptions.
type NfSubscriptionStatus struct {
	Amf NfSubscriptionEntry `json:"amf"`
	Smf NfSubscriptionEntry `json:"smf"`
}

// NfSubscriptionEntry describes the current state of one outbound subscription.
type NfSubscriptionEntry struct {
	Subscribed     bool     `json:"subscribed"`
	SubscriptionId string   `json:"subscriptionId,omitempty"`
	NfUri          string   `json:"nfUri,omitempty"`
	Events         []string `json:"events,omitempty"`
}

// NfSubscribeRequest is the optional request body for subscribe endpoints.
// If Events is empty the default event list from config is used.
type NfSubscribeRequest struct {
	Events []string `json:"events,omitempty"`
}

// HandleGetNfSubscriptions returns the current outbound subscription status
// for AMF and SMF.
func (p *Processor) HandleGetNfSubscriptions() (*NfSubscriptionStatus, *models.ProblemDetails) {
	ctx := p.Context()
	cfg := p.Config()
	eventSubCfg := cfg.GetEventSubscription()

	status := &NfSubscriptionStatus{}

	if eventSubCfg != nil {
		if amfCfg := eventSubCfg.AmfEventSubscription; amfCfg != nil {
			status.Amf.NfUri = amfCfg.NfUri
			status.Amf.Events = amfCfg.Events
		}
		if smfCfg := eventSubCfg.SmfEventSubscription; smfCfg != nil {
			status.Smf.NfUri = smfCfg.NfUri
			status.Smf.Events = smfCfg.Events
		}
	}

	if ctx.AmfSubscriptionId != "" {
		status.Amf.Subscribed = true
		status.Amf.SubscriptionId = ctx.AmfSubscriptionId
	}
	if ctx.SmfSubscriptionId != "" {
		status.Smf.Subscribed = true
		status.Smf.SubscriptionId = ctx.SmfSubscriptionId
	}

	return status, nil
}
