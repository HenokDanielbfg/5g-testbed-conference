# Free5GC SBI API Endpoints (for agent use)

## NRF (127.0.0.10:8000)
- PUT /nnrf-nfm/v1/nf-instances/{id} — Register NF (NO AUTH REQUIRED)
- DELETE /nnrf-nfm/v1/nf-instances/{id} — Deregister NF (OAuth2)
- GET /nnrf-nfm/v1/nf-instances/{id} — Get NF profile (OAuth2)
- PATCH /nnrf-nfm/v1/nf-instances/{id} — Update NF profile (OAuth2)
- GET /nnrf-disc/v1/nf-instances?nfType=AMF — Discover NFs (OAuth2)
- POST /oauth2/token — Get OAuth2 token

## AMF (127.0.0.18:8000)
- GET /namf-oam/v1/registered-ue-context — List all active UEs (read-only)
- GET /namf-oam/v1/registered-ue-context/:supi — Get specific UE context
- POST /namf-evts/v1/subscriptions — Subscribe to AMF events
- POST /namf-comm/v1/ue-contexts — UE context management

## SMF (127.0.0.2:8000)
- GET /nsmf-oam/v1/ue-pdu-session-info/:ref — Get PDU session info
- GET /nsmf-oam/v1/user-plane-info/ — Get user plane info
- POST /nsmf-event-exposure/v1/subscriptions — Subscribe to SMF events
- POST /nsmf-pdusession/v1/sm-contexts — Session management

## NWDAF (127.0.0.47:8000)
- GET /nnwdaf-analyticsinfo/v1/analytics?event-id=NF_LOAD
- GET /nnwdaf-analyticsinfo/v1/analytics?event-id=UE_MOBILITY
- GET /nnwdaf-analyticsinfo/v1/analytics?event-id=UE_COMMUNICATION
- GET /nnwdaf-analyticsinfo/v1/analytics?event-id=ABNORMAL_BEHAVIOUR
- GET /nnwdaf-analyticsinfo/v1/analytics?event-id=NWDAF_RECOMMENDATION
- GET /nnwdaf-analyticsinfo/v1/analytics?event-id=NETWORK_PERFORMANCE
- GET /nnwdaf-analyticsinfo/v1/analytics?event-id=SLICE_LOAD_LEVEL
- POST /nnwdaf-eventssubscription/v1/subscriptions — Subscribe to analytics

## PCF (127.0.0.7:8000)
- POST /npcf-smpolicycontrol/v1/sm-policies — Create SM policy
- POST /npcf-ampolicycontrol/v1/am-policies — Create AM policy
- GET /npcf-oam/v1/am-policy/:supi — Get AM policy for UE
- POST /npcf-policyauthorization/v1/app-sessions — App session policies

## UDR
- GET/PUT/DELETE /nudr-dr/v1/subscription-data/:supi/... — Subscriber data CRUD
- GET/PUT/DELETE /nudr-dr/v1/policy-data/... — Policy data CRUD

## Webconsole (0.0.0.0:5000)
- POST /api/login — JWT authentication
- GET/POST/PUT/DELETE /api/subscriber — Subscriber management
- GET /api/registered-ue-context — Active UE sessions

## NF Event Exposure (who sends events to whom)
- AMF → NWDAF: location, registration, connectivity, reachability, timezone, access type
- SMF → NWDAF: PDU session establish/release, UP path change, UE IP change
- UDM: data change notifications, deregistration notifications
- PCF: AF events (access type change, PLMN change, QoS, resource allocation, usage)
- NSSF: NSSAI availability (stub only, no push)
