# III. Proposed Architecture

The proposed system establishes a three-layer architecture that bridges a 3GPP-compliant 5G core network with LLM-based autonomous agents through a structured data mediation layer. Figure 1 illustrates the high-level design: (1) a live 5G Core Network instrumented with event exposure, (2) a custom Network Data Analytics Function (NWDAF) that aggregates, analyzes, and exposes network intelligence via standardized interfaces, and (3) a multi-agent LLM layer that consumes that intelligence to reason, diagnose, and act. The layers communicate exclusively through well-defined APIs — the 3GPP Service-Based Interface (SBI) internally, and Model Context Protocol (MCP) servers at the LLM boundary — ensuring that no agent has unmediated access to core network state.

---

## A. Layer 1: 5G Core Network with Event Exposure

The foundation of the architecture is a fully functional 5G Standalone (SA) core network consisting of eleven Network Functions (NFs): NRF, AMF, SMF, UPF, UDM, UDR, AUSF, PCF, NSSF, CHF, and NWDAF. Each NF runs as an independent microservice, registers with the Network Repository Function (NRF) at startup, and communicates over HTTP/2-based SBI interfaces secured with OAuth2 tokens. This design follows the 3GPP Rel-16 service-based architecture specification.

Two NFs are of particular importance for event-driven intelligence collection: the Access and Mobility Management Function (AMF) and the Session Management Function (SMF). The AMF generates events spanning the full UE lifecycle — registration, deregistration, location updates, connectivity state changes, reachability reports, access type changes, and timezone notifications. The SMF generates complementary session-layer events: PDU session establishment and release, user-plane path changes, and UE IP address changes. Both NFs were extended with dedicated notifier modules that evaluate active event subscriptions and dispatch HTTP POST notifications to subscriber endpoints whenever a matching trigger fires. The NWDAF registers subscriptions with both functions at startup, and thereafter receives a continuous real-time event stream without polling.

This push-based event model is significant: rather than periodically querying NF state, the NWDAF maintains an up-to-date picture of the network derived from actual signaling events, making it responsive to changes at the millisecond timescale.

---

## B. Layer 2: NWDAF — Analytics and Intelligence Engine

The NWDAF is the central intelligence component of the architecture. It implements the 3GPP Nnwdaf service-based interface and performs three distinct roles: data collection, analytics production, and subscription management.

**Data Collection.** Upon startup, the NWDAF registers with the NRF and establishes event subscriptions with the AMF (via `Namf_EventExposure_Subscribe`) and the SMF (via `Nsmf_EventExposure_Subscribe`). Incoming notifications arrive at two callback endpoints: `POST /nnwdaf-callback/v1/amf-event-notify` and `POST /nnwdaf-callback/v1/smf-event-notify`. A dedicated handler parses each notification and writes it into the EventStore.

**EventStore.** The EventStore is an in-memory ring-buffer store indexed by both event type and subscriber identity (SUPI). It maintains a capacity of 1,000 events per event type and 100 events per UE, providing a sliding temporal window of recent network activity. The store exposes query primitives for filtering by type or UE, computing event rates over configurable time windows, enumerating active UEs, and returning aggregate counts. This design ensures constant memory usage regardless of network activity volume.

**Analytics Production.** Analytics are produced on-demand when an NF Consumer (or MCP server) issues a `GET /nnwdaf-analyticsinfo/v1/analytics?event-id=<ID>` request. The NWDAF currently supports five analytics types:

- *NF_LOAD*: Derives per-NF CPU and memory utilisation directly from Linux process metrics, combined with event-throughput rates and active UE counts from the EventStore. In the absence of NF self-reported load metrics via NRF — not implemented in the open-source free5gc baseline — direct process measurement serves as an equivalent observable for the purposes of this evaluation.
- *UE_MOBILITY*: Aggregates location report events per UE, surfacing movement history and deriving mobility patterns including predicted next location using a last-*k* trajectory model.
- *UE_COMMUNICATION*: Summarizes session establishment and release patterns, data volume indicators, and PDU session durations per UE.
- *ABNORMAL_BEHAVIOUR*: Applies anomaly scoring across the event stream, flagging statistical deviations in registration rates, session frequency, and connectivity state transitions. A fixed observation window is used, and threshold-based scoring classifies anomalies by severity (LOW, MEDIUM, HIGH).
- *NWDAF_RECOMMENDATION*: Synthesizes the outputs of the preceding analytics into actionable recommendations. When anomaly scores exceed defined thresholds, the recommendation engine generates structured advisories — including the implicated NF or UE, the anomaly type, severity level, and a suggested remediation action.

This analytics pipeline transforms raw signaling events into structured, human-interpretable (and LLM-interpretable) intelligence without requiring agents to parse low-level protocol messages.

**Subscription Management.** The NWDAF also exposes `POST /nnwdaf-eventssubscription/v1/subscriptions`, allowing downstream consumers to register for analytics-push notifications. This service is currently implemented for subscription registration; push delivery to external consumers is a designated extension point for future work.

---

## C. Layer 3: MCP Servers — Structured Tool Interface for LLM Agents

The Model Context Protocol (MCP) layer translates NWDAF and SBI interfaces into tool-callable functions that LLM agents can invoke without knowledge of the underlying HTTP API semantics. Three MCP servers are deployed, each scoped to a distinct data domain:

**NWDAF MCP Server.** Exposes NWDAF capabilities as two categories of structured tools. The first is analytics: `get_nf_load`, `get_ue_mobility`, `get_ue_communication`, `get_anomalies`, `get_recommendations`, and `get_all_analytics`, each issuing the corresponding `Nnwdaf-AnalyticsInfo` request and returning a structured result. The second is subscription management: `get_subscription_status` returns the current state of NWDAF's outbound subscriptions to AMF and SMF, and `manage_nf_subscription` allows the Lead Agent to subscribe or unsubscribe from an NF's event stream at runtime, optionally overriding the default event list with a targeted subset. This enables the Lead Agent to dynamically adjust what data the NWDAF collects based on the current investigative task — for example, dropping mobility events when investigating a session-layer anomaly, or resubscribing with a reduced event set to lower collection overhead. This server abstracts all URL construction and error normalization from the agent.

**SBI Gateway MCP Server.** Provides OAuth2-aware access to NRF, AMF, and SMF management APIs. Tools include: `get_network_overview` (NRF NF registry snapshot), `get_registered_ues` (AMF OAM UE list), `get_ue_context` (per-UE AMF context), `get_pdu_session_info` (SMF session details), `get_user_plane_info` (UPF path summary), and `get_nf_profile` (individual NF registration record). This server handles token acquisition via `POST /oauth2/token` against the NRF, caches tokens, and transparently refreshes them on expiry.

**Log MCP Server.** Provides structured access to the shared free5GC log file. Tools include: `get_log_tail` (most recent N lines), `get_errors` (filtered by NF and severity), `get_log_stats` (aggregate error counts by NF), `get_nf_activity` (per-NF event timeline), and `search_log` (regex pattern search). Log access is critical for agents to observe NF behavior that is not yet surfaced through the NWDAF analytics pipeline, including process crashes, failed authentications, and certificate warnings.

Together, these three servers provide LLM agents with a complete observability surface: structured analytics from NWDAF, live NF registry and session state from the SBI gateway, and raw operational telemetry from the log layer. The MCP server boundary also serves as a natural access control point — agents cannot reach the core network except through the tool interface, and new capabilities can be exposed by adding MCP servers without modifying the agents themselves.

---

## D. Layer 4: LLM Multi-Agent Layer

The topmost layer consists of a team of LLM agents that operate collaboratively against the MCP tool interface. Rather than a fixed team composition, the architecture centers on a **Lead Agent** that dynamically orchestrates a pool of specialized agents in response to network conditions and operator directives. This design reflects a core thesis of the proposed system: the agent team itself is a configurable, adaptive resource — not a static deployment.

### D.1 Lead Agent

The Lead Agent is the sole point of contact between the system and the human operator. It receives operator requests (e.g., "investigate this UE", "check for rogue NFs", "why is the AMF load elevated?"), reasons about what investigative capability is required, and deploys the appropriate specialist agents to address the task. It is equally responsive to network-driven triggers: when the NWDAF produces a recommendation or an anomaly exceeds a severity threshold, the Lead Agent can autonomously decide to instantiate new agents tailored to the specific condition — for example, spawning a threat-focused agent when an abnormal registration rate is detected, or a session-analysis agent when PDU session flapping is observed.

Once specialist agents are running, the Lead Agent coordinates their work via a shared task board, synthesizes their findings into a coherent assessment, and reports conclusions and recommended actions to the operator. It is the only agent that communicates directly with the operator, acting as a single interface point to the multi-agent system.

This orchestration model has an important architectural property: the Lead Agent does not require prior knowledge of every possible network condition. It reasons about the task at hand and composes an agent team dynamically, which means the system can respond to novel scenarios without requiring re-engineering of the agent layer.

### D.2 Specialist Agents

Specialist agents are instantiated by the Lead Agent on demand. Each agent is given a focused role, a defined set of MCP tools it may use, and a task from the shared board. Agents operate concurrently, share findings with one another, and escalate significant observations to the Lead Agent immediately rather than waiting for a scheduled report cycle.

The architecture places no structural constraint on the number or type of specialist agents that can be deployed. For a given operator request or network condition, the Lead may deploy a single targeted agent or a coordinated team of several. In the validation scenario described in Section V, four specialist agent types are instantiated: a network monitor, a traffic analyst, a fault detector, and a threat assessor — each consuming different subsets of the MCP tool surface and contributing complementary perspectives on network state.

### D.3 Shared Task Board and Inter-Agent Communication

Agents coordinate through two mechanisms: a shared task board and direct inter-agent messaging. The task board maintains a list of active investigations, their current status, and assigned agents, providing the Lead Agent with a global view of in-progress work. Direct messaging allows agents to share findings asynchronously — an analyst observing an anomalous UE communication pattern can immediately notify a threat agent without waiting for the Lead Agent to relay the information. This reduces detection latency for multi-signal attack patterns that require cross-agent correlation.

---

## E. Design Principles

Several design principles govern the architecture:

**Separation of concerns.** Network intelligence production (NWDAF) is strictly decoupled from intelligence consumption (LLM agents). Agents never bypass the MCP tool layer to issue direct SBI calls, preserving auditability and access control.

**Dynamic agent composition.** The agent team is not statically defined. The Lead Agent reasons about what investigative capability a situation requires and composes the team accordingly, making the system adaptive to novel conditions without requiring re-engineering.

**Dual-lens analysis.** Deploying parallel agents with complementary analytical framings — for example, a fault agent and a threat agent examining the same telemetry — reduces the risk of framing bias. The same observation is evaluated under both a benign (operational failure) and adversarial (active attack) hypothesis simultaneously.

**Stateful continuous operation.** Agents maintain state across observation rounds to detect trends and deltas, reducing alert noise while improving sensitivity to gradual degradation — a key advantage over threshold-based rule engines for slow-moving attack patterns.

**Standards alignment.** The NWDAF implements the 3GPP Nnwdaf interface. The SBI gateway uses standard NRF OAuth2 token issuance. MCP is an emerging open standard for LLM tool interfaces. This alignment positions the architecture as extensible to production 5G deployments, not merely a research prototype.
