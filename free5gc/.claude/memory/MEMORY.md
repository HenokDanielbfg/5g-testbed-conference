# Free5GC Project Memory

## Project Overview
- free5gc core network at `/home/henokbfg/Documents/5g-testbed-conference/free5gc/`
- Custom NWDAF NF built at `NFs/nwdaf/` (written by us)
- 11 NFs total: NRF, AMF, SMF, UPF, UDM, UDR, AUSF, PCF, NSSF, CHF, NWDAF
- Go 1.21, openapi v1.0.8

## Key Files & Paths
- Logs: `free5gc/log/<YYYYMMDD_HHMMSS>/free5gc.log` (new dir per run)
- NWDAF config: `config/nwdafcfg.yaml`
- NWDAF source: `NFs/nwdaf/internal/`
- Startup: `run.sh`, build: `Makefile`
- Webconsole: `webconsole/` (port 5000)

## NWDAF Architecture
- See [nwdaf-architecture.md](nwdaf-architecture.md) for full details
- Subscribes to AMF + SMF events, stores in EventStore (ring buffers)
- Exposes analytics via REST: NF_LOAD, UE_MOBILITY, UE_COMMUNICATION, ABNORMAL_BEHAVIOUR, RECOMMENDATION
- OAuth2 fix applied: `context.go` now calls `oauth.GetTokenCtx()` properly

## SBI API Endpoints (Live Network)
- See [api-endpoints.md](api-endpoints.md) for full inventory
- NWDAF analytics: `http://127.0.0.47:8000/nnwdaf-analyticsinfo/v1/analytics?event-id=...`
- NRF registration (no auth): `PUT http://127.0.0.10:8000/nnrf-nfm/v1/nf-instances/{id}`
- AMF OAM: `http://127.0.0.18:8000/namf-oam/v1/registered-ue-context`
- SMF OAM: `http://127.0.0.2:8000/nsmf-oam/v1/user-plane-info/`

## Multi-Agent Control Plan
- See [agent-team-design.md](agent-team-design.md)
- Using Claude Agent Teams (not Go services)
- Agents interact via curl, log reading, codebase exploration
- Team: monitor, analyst, fault, threat + team lead
- Demo scenario (updated 2026-03-30): 3-act script (baseline → normal UE activity → registration flood attack)
- Agents run in split panes (not background) so operator can monitor each visually

## Current State (2026-03-26)
- NWDAF integrated into Makefile, run.sh, force_kill.sh
- MCP servers auto-start with the core (run.sh launches nwdaf-mcp.py + sbi-mcp.py)
- gtp5g kernel module upgraded to v0.8.10 (fixes queryMultiURR on kernel 6.8)
- NSSF config expanded: 4 TAIs (000001-000004) for PLMN 208/93, 3 slices each
- UERANSIM join_leavev2.py: fast 30-35s UE cycling for demos
- gNB2 linkIp changed to 192.168.56.1 (external interface)
- Journal paper Section III (Proposed Architecture) drafted in `draft.md`

## Known Issues
- ~~UPF `queryMultiURR invalid argument`~~ — FIXED: gtp5g upgraded v0.8.6 → v0.8.10
- NRF `x509 certificate signed by unknown authority` — self-signed cert trust issue
- CHF billing FTP at 127.0.0.1:2121 not running
- UPF unsubscribe on shutdown fails (race condition, NFs already down)

## Journal Paper
- See [journal-paper.md](journal-paper.md) for full plan
- Vision/architecture paper: LLM-driven autonomous 5G network management via NWDAF
- Validation: 6 attack scenarios (DoS → low-and-slow), benchmarked against rule-based SIEM + human operator
- Metrics: detection rate, time-to-detection, root cause accuracy, false positives, explainability

## User Preferences
- User is Henok, working on 5G testbed for a conference + journal paper
- Prefers discussion before implementation
- Interested in multi-agent (Claude teams) for live network control
