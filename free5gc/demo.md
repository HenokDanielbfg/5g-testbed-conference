# Agentic 5G Core Network Security Demo

## Overview
- **Audience:** Telecom / academic conference
- **Duration:** ~10 minutes (live or recorded)
- **Takeaway:** Multi-agent AI can autonomously monitor, detect attacks, and remediate faults in a live 5G core — covering both infrastructure and SBA-layer threats

## Architecture
- **Implementation:** Claude Code Agent Teams (interactive, not scripted)
- **Agents:** 3 (monitor, analyst, fault) coordinated by a team lead
- **Tools:** Bash, Read, Glob, Grep — agents decide what to run autonomously
- **Attack scripts:** `attack_inject.sh` for NRF poisoning (inject + restore)

## Agent Roles

| Agent | Role | Checks |
|-------|------|--------|
| **Monitor** | Process health, UE activity, log growth | `pgrep` NF processes, log inspection, IMSI detection |
| **Analyst** | Network analytics via NWDAF REST APIs | NF_LOAD, UE_MOBILITY, UE_COMMUNICATION, ABNORMAL_BEHAVIOUR |
| **Fault** | Fault detection, NRF integrity, auto-remediation | Process comparison, NRF registration audit, log error classification |

## Demo Script

### Act 1: Healthy Network Baseline (~2 min)

**Setup:** Core running, UEs connected via UERANSIM.

1. Spin up agent team (monitor, analyst, fault)
2. All 3 agents report independently:
   - Monitor: "11/11 NFs running, N UEs connected"
   - Analyst: "NF loads normal (AMF ~68%, SMF ~62%, UPF ~56%), no anomalies"
   - Fault: "9 NRF registrations (expected), no errors beyond known issues"
3. Establishes clean baseline for the audience

**Talking point:** "Three AI agents autonomously assess the network — no dashboards, no manual checks."

### Act 2: Infrastructure Attack — NF Crash (~3 min)

**Attack:** Kill the SMF process.
```bash
kill $(pgrep -f 'bin/smf')
```

**Detection & remediation sequence:**
1. Fault agent detects SMF missing on next check cycle (~20s)
2. Fault agent classifies as CRITICAL — session management down
3. Fault agent auto-restarts SMF:
   ```bash
   ./bin/smf -c ./config/smfcfg.yaml -l <log_path>/free5gc.log &
   ```
4. Monitor independently confirms the outage and recovery
5. Subsequent checks confirm stability on new PID

**Talking point:** "Fault detection and autonomous remediation in under 30 seconds — no human intervention."

### Act 3: SBA Attack — NRF Poisoning (~3 min)

**Attack:** Two NRF manipulation attacks from the academic literature on 5G SBA vulnerabilities.
```bash
./attack_inject.sh attack
```

This injects:
1. **Rogue NF Registration** — registers a fake SMF (`deadbeef-...`) at attacker IP `10.0.0.99`
2. **NF Profile Hijack** — overwrites the real SMF's NRF profile, changes its IP to `10.0.0.99` and status to `UNDISCOVERABLE`

Both exploit the unauthenticated PUT endpoint on the NRF (`/nnrf-nfm/v1/nf-instances/{uuid}`).

**Detection sequence:**
1. Fault agent's NRF integrity check finds 10 registrations vs expected 9
2. Flags the late PUT requests (~2 min after startup burst)
3. Matches attacker patterns: `deadbeef` UUID, `10.0.0.99` IP
4. Distinguishes: HTTP 201 (new rogue NF) vs HTTP 200 (profile overwrite)
5. Identifies root cause: PUT has no OAuth, while GET/DELETE do

**Key contrast:** Monitor agent sees all 11 NFs still running (all green). The SBA attack is **invisible to process monitoring** — only the NRF integrity check catches it.

**Talking point:** "This is a known vulnerability in the 5G SBA architecture. Traditional monitoring misses it entirely. Multi-layer AI monitoring detects what process checks cannot."

**Cleanup:**
```bash
./attack_inject.sh restore
```

### Wrap-Up (~2 min)

Summary points:
- **Two attack classes demonstrated:** availability (NF crash) and integrity (NRF poisoning)
- **Multi-layer detection:** process monitoring alone is insufficient; registry-level auditing is critical
- **Autonomous remediation:** agents don't just detect, they fix (SMF restart)
- **Real vulnerabilities:** NRF unauthenticated PUT is documented in academic literature
- **Real fixes:** agents surfaced the gtp5g UPF bug, leading to an actual kernel module upgrade (v0.8.6 → v0.8.10)

## Attack Details

### Attack 1: Rogue NF Registration
- **Academic basis:** SBA trust model attacks — rogue NF registration via unauthenticated NRF API
- **Method:** `PUT /nnrf-nfm/v1/nf-instances/{uuid}` with no auth token
- **Impact:** Fake SMF appears in service discovery; NFs could route traffic to attacker
- **Detection:** Registration count mismatch, late timing, attacker UUID pattern

### Attack 2: NF Profile Hijack
- **Academic basis:** NRF as single point of failure — profile tampering via unprotected HTTP PUT
- **Method:** Overwrite legitimate SMF's profile with attacker IP and UNDISCOVERABLE status
- **Impact:** Real SMF becomes invisible to service discovery; new PDU sessions fail silently
- **Detection:** Legitimate UUID re-PUT outside normal heartbeat cycle, HTTP 200 (update) vs 201 (create)

### Root Cause
NRF enforces OAuth on GET/DELETE but **not on PUT**. Any entity with network access can register or modify NF profiles. This is an asymmetric auth vulnerability in the free5gc NRF implementation, consistent with findings in the academic literature on 5G SBA security.

## Key Files

| File | Purpose |
|------|---------|
| `run.sh` | Start the 5G core |
| `attack_inject.sh` | Inject/restore NRF attacks |
| `config/nrfcfg.yaml` | NRF configuration (SBI at 127.0.0.10:8000) |
| `config/smfcfg.yaml` | SMF configuration |
| `log/<timestamp>/free5gc.log` | Unified log file (new dir per run) |

## API Endpoints

| Endpoint | Used By | Auth |
|----------|---------|------|
| `http://127.0.0.47:8000/nnwdaf-analyticsinfo/v1/analytics?event-id=NF_LOAD` | Analyst | None |
| `http://127.0.0.47:8000/nnwdaf-analyticsinfo/v1/analytics?event-id=UE_MOBILITY` | Analyst | None |
| `http://127.0.0.47:8000/nnwdaf-analyticsinfo/v1/analytics?event-id=UE_COMMUNICATION` | Analyst | None |
| `http://127.0.0.47:8000/nnwdaf-analyticsinfo/v1/analytics?event-id=ABNORMAL_BEHAVIOUR` | Analyst | None |
| `PUT http://127.0.0.10:8000/nnrf-nfm/v1/nf-instances/{uuid}` | Attack script | **None (vulnerability)** |
| `DELETE http://127.0.0.10:8000/nnrf-nfm/v1/nf-instances/{uuid}` | Attack script | OAuth required |

## Prerequisites
- free5gc built and running (`sudo ./run.sh`)
- UERANSIM configured for UE connections
- Claude Code installed with agent teams support
- gtp5g kernel module v0.8.10 installed (fixes UPF queryMultiURR on kernel 6.8)

## Known Issues (Non-Critical)
- **NRF x509 cert warning** — self-signed certificate, causes OAuth token warnings (MEDIUM)
- **CHF billing FTP down** — 127.0.0.1:2121 not running (MEDIUM)
- **UPF queryMultiURR** — FIXED by upgrading gtp5g v0.8.6 → v0.8.10

## References
- [5G Core Security: Insider Threat Assessment (2024)](https://www.researchgate.net/publication/380264652_5G_Core_Security_An_Insider_Threat_Vulnerability_Assessment)
- [Cross-Service Token Attacks in 5G Core (2025)](https://arxiv.org/html/2509.08992)
- [Intruding 5G SA Core Networks](https://penthertz.com/blog/Intruding-5G-core-networks-from-outside-and_inside.html)
- [Vulnerability Assessment of Open-Source 5G Core](https://www.mdpi.com/1999-5903/16/1/1)
- [OAuth 2.0 in 5G Core Networks](https://www.lfdecentralizedtrust.org/blog/oauth-2.0-authorization-in-5g-core-networks-architecture-workflows-and-security-challenges)
