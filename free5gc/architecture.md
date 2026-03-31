# System Architecture: LLM-Driven Autonomous 5G Network Management via NWDAF

## Overview

This system combines a fully functional 5G core network with an LLM agent layer that autonomously monitors, analyzes, detects faults, and identifies security threats in real time. Three MCP (Model Context Protocol) servers act as the bridge between the LLM agents and the live network, translating agent tool calls into authenticated SBI API requests, analytics queries, and structured log access.

## Layers

### 1. RAN Layer (UERANSIM)

Simulated radio access network with a gNB and multiple UEs. The gNB connects to the core via N2 (control plane signaling to AMF) and N3 (user plane traffic to UPF). UEs register, authenticate, and establish PDU sessions through this interface.

### 2. 5G Core Network (free5gc)

Eleven network functions written in Go, communicating over HTTP/2 SBI interfaces with OAuth2 authentication managed by the NRF:

- **NRF** — NF registration, discovery, and OAuth2 token issuing. Backed by MongoDB.
- **AMF** — Access and mobility management. Handles UE registration, authentication orchestration, and connection management.
- **SMF** — Session management. Creates and manages PDU sessions, controls the user plane via PFCP.
- **UPF** — User plane forwarding. Runs a GTP5G kernel module for packet processing.
- **AUSF, UDM, UDR** — Authentication and subscriber data. UDR stores subscription profiles in MongoDB.
- **PCF** — Policy control for sessions and access.
- **NSSF** — Network slice selection.
- **CHF** — Charging (CDR generation).
- **NWDAF** — Custom-built network data analytics function. This is the key addition to the standard free5gc deployment.

#### NWDAF (Custom NF)

The NWDAF subscribes to events from the AMF (location, registration, connectivity state) and SMF (PDU session establishment/release, path changes) at startup. Incoming events are stored in ring buffers (1000 per event type, 100 per UE). An analytics engine computes NF load, UE mobility patterns, UE communication statistics, and anomaly scores on demand. A prediction layer provides session load forecasting, mobility prediction, and a recommendation engine that aggregates findings into actionable outputs.

The NWDAF exposes its analytics through a standard 3GPP REST API with five primary event types: NF_LOAD, UE_MOBILITY, UE_COMMUNICATION, ABNORMAL_BEHAVIOUR, and NWDAF_RECOMMENDATION.

### 3. MCP Server Layer

Two Python servers built with FastMCP provide structured tool interfaces for the AI agents:

- **SBI Gateway** — An OAuth2-aware proxy to the NRF, AMF, and SMF SBI APIs. On startup, it registers itself as an NF with the NRF, then automatically acquires and caches OAuth2 tokens for each target service. Agents call tools like `get_registered_ues` or `check_nrf_integrity` without needing to handle authentication. Provides 8 tools covering NF discovery, UE context, user plane info, and security auditing.

- **NWDAF Analytics** — A lightweight proxy to the NWDAF analytics API. Exposes 6 tools that query individual analytics endpoints or fetch all five in parallel. No authentication required (direct HTTP to the NWDAF).

- **Log Server** — Structured access to the free5gc shared log file. Provides tools to query recent logs, filter errors by NF, search for patterns, and get log statistics. Converts raw log lines into structured entries with NF attribution, severity levels, and timestamps.

All three servers are defined in the project's `.mcp.json` and are automatically available to any Claude Code session or agent working in the project directory.

### 4. AI Agent Layer (Claude Agent Team: 5gc-ops)

A team of Claude agents that operate the network collaboratively:

- **Team Lead** — Coordinates the three specialist agents, synthesizes their findings into consolidated reports, and interfaces with the user. Runs in the main Claude Code conversation.

- **Monitor** — Checks NF process health (via `pgrep`), log activity (file growth and error patterns), and live UE/NF status (via MCP tools). Detects when NFs crash, UEs connect or disconnect, and whether the network is active or idle.

- **Analyst** — Queries all NWDAF analytics endpoints via MCP to assess NF load levels, UE mobility and communication patterns, and anomaly status. Cross-references NWDAF data with AMF registrations to verify consistency.

- **Fault Detector** — Runs three detection modes: process audit (are all 11 NFs alive?), log scan (classify errors by NF and severity), and NRF integrity check (detect rogue NF registrations, unexpected IPs, or profile hijacking). Falls back to log-based auditing when APIs are unavailable.

- **Threat Detector** — The adversarial counterpart to the Fault Detector. Receives the same data but applies a security lens: every anomaly is treated as a potential indicator of compromise until proven benign. Runs a 4-phase investigation protocol: (1) threat surface scan, (2) hypothesis testing against an 8-pattern 5G attack taxonomy (DoS floods, rogue gNB/NF, profile hijack, SUPI spoofing, session hijacking, slice breach, low-and-slow exfiltration), (3) NRF integrity audit for rogue registrations, and (4) UE behavioral analysis for registration patterns and IMSI enumeration. Challenges the fault agent's benign explanations — where fault sees a "race condition," threat considers a registration flood; where fault sees a "misconfiguration," threat considers a rogue NF injection. Outputs structured threat assessments with threat level (GREEN→RED), kill-chain stage, IoCs, and recommended actions.

Agents operate in timed rounds. In each round, all four run in parallel, then share findings with each other via cross-agent messaging. Each agent incorporates intel from the others — the threat agent specifically challenges benign explanations from the fault agent, while fault provides timing and error data that enriches threat hypotheses. A shared task board tracks task dependencies and ownership across the team.

## Data Flow Summary

```
UEs → gNB → AMF/UPF (registration + data sessions)
              ↓
        AMF/SMF → NWDAF (event subscriptions)
              ↓                          ↓
        NWDAF (analytics engine)    Log Files
              ↓                          ↓
        ┌─────────── MCP Servers ───────────┐
        │ SBI Gateway │ NWDAF Analytics │ Log │
        └───────────────────────────────────┘
                          ↓
        ┌──── LLM Agent Team (5gc-ops) ────┐
        │ Monitor │ Analyst │ Fault │ Threat │
        │         ← intel sharing →          │
        │        [Shared Task Board]         │
        └────────────────────────────────────┘
                          ↓
                    Team Lead → Operator
```

## Key Design Decisions

- **MCP as the bridge, not custom APIs.** Agents access the network through MCP tools rather than raw HTTP. This gives them structured responses, automatic OAuth2 handling, and a tool interface that integrates natively with Claude's function calling.

- **Agents use both MCP and bash.** MCP tools provide clean structured data from APIs, while bash access (pgrep, grep, stat) gives agents direct visibility into process health and log files that no API exposes.

- **Multi-round cross-referencing.** Agents don't just collect data — they challenge each other's findings. A UE count discrepancy between monitor and analyst gets investigated in Round 2. A high load reading gets cross-checked against actual process CPU usage.

- **NWDAF as the analytics backbone.** Rather than having agents parse raw logs for analytics, the NWDAF collects events in real time and computes metrics server-side. Agents query pre-computed analytics, making their analysis faster and more reliable.
