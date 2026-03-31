# NWDAF Architecture

## Source: NFs/nwdaf/

## Event Subscription (Data Collection)
- Subscribes to AMF events at startup: LOCATION_REPORT, REGISTRATION_STATE_REPORT, CONNECTIVITY_STATE_REPORT, REACHABILITY_REPORT, TIMEZONE_REPORT, ACCESS_TYPE_REPORT, SUBSCRIBED_DATA_REPORT
- Subscribes to SMF events: PDU_SES_EST, PDU_SES_REL, UP_PATH_CH, UE_IP_CH
- Config: `config/nwdafcfg.yaml` specifies NF URIs and event lists
- AMF URI: http://127.0.0.18:8000, SMF URI: http://127.0.0.2:8000

## Callback Endpoints (receives events)
- POST /nnwdaf-callback/v1/amf-event-notify → processor/callback.go HandleAmfEventNotification
- POST /nnwdaf-callback/v1/smf-event-notify → processor/callback.go HandleSmfEventNotification

## EventStore (context/event_store.go)
- Ring buffers: 1000 events per type, 100 per UE (SUPI)
- Query methods: GetAmfEventsByType, GetSmfEventsByType, GetAmfEventsForSupi, GetSmfEventsForSupi
- GetEventRate(eventType, window) → events/sec
- GetUniqueSupis(), GetTotalEventCount()

## Analytics API (producer)
- GET /nnwdaf-analyticsinfo/v1/analytics?event-id=<ID>
- Event IDs: NF_LOAD, NETWORK_PERFORMANCE, UE_MOBILITY, UE_COMMUNICATION, ABNORMAL_BEHAVIOUR, SLICE_LOAD_LEVEL, NWDAF_RECOMMENDATION, QOS_SUSTAINABILITY
- processor/analytics_info.go — builds analytics from EventStore data
- processor/prediction.go — mobility prediction, session load forecast, anomaly scoring
- processor/recommendation.go — generates recommendations from predictions

## Events Subscription Service (consumer-facing)
- POST /nnwdaf-eventssubscription/v1/subscriptions — create subscription
- DELETE/PUT /nnwdaf-eventssubscription/v1/subscriptions/:id — manage
- Stores subscriptions but does NOT push notifications yet (not implemented)

## OAuth2 Fix (applied in this session)
- context.go: GetTokenCtx now calls oauth.GetTokenCtx(NfType_NWDAF, ...) when OAuth2Required=true
- amf_service.go: uses GetTokenCtx(ServiceName_NAMF_EVTS, NfType_AMF) before API calls
- smf_service.go: uses GetTokenCtx(ServiceName_NSMF_EVENT_EXPOSURE, NfType_SMF) before API calls
- Previous error was: "401 is not a valid status code in CreateSubscription"

## AMF Side (how events are sent to NWDAF)
- amf/internal/sbi/processor/notifier/subscription.go: SendEventNotification()
- Iterates all stored EventSubscriptions, matches event type, POSTs to EventNotifyUri
- Triggered from: ngap/handler.go (UE Initial Context Setup), gmm/handler.go (registration), gmm/common/user_profile.go (deregistration)
- Henok added LOCATION_REPORT trigger at gmm/handler.go:2232

## SMF Side
- smf/internal/sbi/processor/notifier.go: SendEventNotification()
- Triggered from: pdu_session.go:242 (PDU_SES_EST), pdu_session.go:957 (PDU_SES_REL)
