package processor

import (
	"github.com/google/uuid"

	nwdaf_context "github.com/free5gc/nwdaf/internal/context"
	"github.com/free5gc/nwdaf/internal/logger"
	"github.com/free5gc/openapi/models"
)

// NwdafEventsSubscription is the request/response body for subscription
// create and update operations.
type NwdafEventsSubscription struct {
	SubscriptionId     string                   `json:"subscriptionId,omitempty"`
	EventSubscriptions []NwdafEventSubscription `json:"eventSubscriptions"`
	NotifUri           string                   `json:"notifUri"`
	NotifCorrId        string                   `json:"notifCorrId,omitempty"`
	NwdafId            string                   `json:"nwdafId,omitempty"`
}

// NwdafEventSubscription describes a single event that the consumer wants to
// subscribe to, including an optional reporting period.
type NwdafEventSubscription struct {
	Event     string `json:"event"`
	RepPeriod int32  `json:"repPeriod,omitempty"`
}

// HandleCreateSubscription processes POST /nnwdaf-eventssubscription/v1/subscriptions.
// It validates the request, assigns a new subscription ID, persists the
// subscription in the NWDAF context, and returns the completed subscription.
// The HTTP 201 status code is set by the SBI (api) layer.
func (p *Processor) HandleCreateSubscription(
	sub *NwdafEventsSubscription,
) (*NwdafEventsSubscription, *models.ProblemDetails) {
	logger.ProducerLog.Infof("[Subscription] HandleCreateSubscription")

	if sub.NotifUri == "" {
		logger.ProducerLog.Warnf("[Subscription] Create rejected: NotifUri is empty")
		return nil, &models.ProblemDetails{
			Status: 400,
			Title:  "Bad Request",
			Detail: "notifUri must not be empty",
		}
	}

	sub.SubscriptionId = uuid.New().String()

	p.Context().Subscriptions.Store(
		sub.SubscriptionId,
		&nwdaf_context.NwdafSubscription{
			SubscriptionId: sub.SubscriptionId,
			NfConsumerUri:  sub.NotifUri,
			Events:         extractEvents(sub),
		},
	)

	logger.ProducerLog.Infof("[Subscription] Created subscription id=%s notifUri=%s",
		sub.SubscriptionId, sub.NotifUri)

	return sub, nil
}

// HandleDeleteSubscription processes DELETE /nnwdaf-eventssubscription/v1/subscriptions/{subscriptionId}.
// Returns nil on success or a ProblemDetails value when the subscription is
// not found.
func (p *Processor) HandleDeleteSubscription(subscriptionId string) *models.ProblemDetails {
	logger.ProducerLog.Infof("[Subscription] HandleDeleteSubscription id=%s", subscriptionId)

	if _, ok := p.Context().Subscriptions.Load(subscriptionId); !ok {
		logger.ProducerLog.Warnf("[Subscription] Delete failed: id=%s not found", subscriptionId)
		return &models.ProblemDetails{
			Status: 404,
			Title:  "Not Found",
			Detail: "subscription " + subscriptionId + " not found",
		}
	}

	p.Context().Subscriptions.Delete(subscriptionId)
	logger.ProducerLog.Infof("[Subscription] Deleted subscription id=%s", subscriptionId)
	return nil
}

// HandleUpdateSubscription processes PUT /nnwdaf-eventssubscription/v1/subscriptions/{subscriptionId}.
// It replaces the event list of an existing subscription with the values
// supplied in sub and returns the updated subscription.
func (p *Processor) HandleUpdateSubscription(
	subscriptionId string,
	sub *NwdafEventsSubscription,
) (*NwdafEventsSubscription, *models.ProblemDetails) {
	logger.ProducerLog.Infof("[Subscription] HandleUpdateSubscription id=%s", subscriptionId)

	stored, ok := p.Context().Subscriptions.Load(subscriptionId)
	if !ok {
		logger.ProducerLog.Warnf("[Subscription] Update failed: id=%s not found", subscriptionId)
		return nil, &models.ProblemDetails{
			Status: 404,
			Title:  "Not Found",
			Detail: "subscription " + subscriptionId + " not found",
		}
	}

	nwdafSub := stored.(*nwdaf_context.NwdafSubscription)
	nwdafSub.Events = extractEvents(sub)
	p.Context().Subscriptions.Store(subscriptionId, nwdafSub)

	sub.SubscriptionId = subscriptionId

	logger.ProducerLog.Infof("[Subscription] Updated subscription id=%s", subscriptionId)
	return sub, nil
}

// extractEvents returns a flat slice of event-name strings taken from the
// EventSubscriptions list of sub. It is used when persisting a subscription
// to the NWDAF context.
func extractEvents(sub *NwdafEventsSubscription) []string {
	var events []string
	for _, e := range sub.EventSubscriptions {
		events = append(events, e.Event)
	}
	return events
}
