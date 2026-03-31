# Multi-Agent Team Design for Live 5GC Control

## Approach: Claude Agent Teams (not Go services)
Agents interact with live core via curl, log reading, and codebase exploration.
No code deployment needed — agents reason about the network using LLM capabilities.

## Team: 5gc-ops

### Team Lead (main conversation)
- Coordinates agents, creates/assigns tasks
- Reports findings to user
- Can direct any agent to investigate specific issues

### Agent: monitor (general-purpose, background)
- Watches log directory: /home/henokbfg/Documents/5g-testbed-conference/free5gc/log/
- Detects: core start/stop (new directory + file growth), UE connections (SUPI in logs), NF crashes (deregistration messages)
- Checks every 30 seconds: latest dir, file size, last modified time
- Reports state changes to team lead via SendMessage
- Key patterns to watch:
  - New log directory = core started
  - File size stopped growing = core stopped or idle
  - "Stored event" lines = UE activity via NWDAF
  - NF="<name>" appearing/disappearing = NF status

### Agent: analyst (general-purpose, background)
- Queries NWDAF APIs when core is running:
  - curl http://127.0.0.47:8000/nnwdaf-analyticsinfo/v1/analytics?event-id=UE_MOBILITY
  - curl http://127.0.0.47:8000/nnwdaf-analyticsinfo/v1/analytics?event-id=UE_COMMUNICATION
  - curl http://127.0.0.47:8000/nnwdaf-analyticsinfo/v1/analytics?event-id=NF_LOAD
  - curl http://127.0.0.47:8000/nnwdaf-analyticsinfo/v1/analytics?event-id=ABNORMAL_BEHAVIOUR
  - curl http://127.0.0.47:8000/nnwdaf-analyticsinfo/v1/analytics?event-id=NWDAF_RECOMMENDATION
- Also queries AMF OAM and SMF OAM for active sessions
- Interprets trends, compares with previous readings

### Agent: fault (general-purpose, background)
Three detection modes — must run ALL three every round:

**Mode 1: Process Audit**
- Run: `pgrep -a -f 'bin/(nrf|amf|smf|upf|udr|pcf|udm|nssf|ausf|chf|nwdaf)'`
- Compare to expected 11 NFs. Any missing = CRITICAL (NF crash / killed)
- If an NF is missing, attempt restart: `./bin/<nf> -c ./config/<nf>cfg.yaml -l <log_path>/free5gc.log &`

**Mode 2: Log Scan**
- Parse errors/warnings from active log file
- Group by NF, count occurrences, identify new vs recurring
- Known recurring issues (classify as MEDIUM, not new):
  - UPF queryMultiURR invalid argument (every 10s, usage reporting broken)
  - NRF x509 certificate warnings (self-signed cert trust)
  - CHF FTP connection refused (billing server not running)

**Mode 3: NRF Integrity Check**
- Query NRF for all registered NFs: `curl http://127.0.0.10:8000/nnrf-nfm/v1/nf-instances`
- For each NF instance, check via: `curl http://127.0.0.10:8000/nnrf-nfm/v1/nf-instances/<uuid>`
- Flag as CRITICAL:
  - Unknown/suspicious NF UUIDs (e.g. deadbeef patterns)
  - NFs with IPs outside expected range (expected: 127.0.0.x). Attacker IP like 10.0.0.99 = rogue
  - NFs with status "UNDISCOVERABLE" (profile hijack indicator)
  - Duplicate NF type registrations (e.g. two SMFs = rogue NF injection)
  - NF services pointing to unexpected IPs/ports
- Expected legitimate NF IPs: NRF=127.0.0.10, AMF=127.0.0.18, SMF=127.0.0.2, UPF=127.0.0.8, UDR/UDM/AUSF/PCF/NSSF/CHF/NWDAF=127.0.0.x

**Severity classification:** CRITICAL (NF crash, rogue NF, profile hijack), HIGH (feature broken), MEDIUM (degraded/known), LOW (cosmetic)

### Agent: threat (general-purpose, background)
Agent definition: `.claude/agents/threat.md`

Adversarial counterpart to the fault agent. Same data sources, different lens: assumes an attacker may be present and asks "who's attacking?" not "what's broken?"

**4-phase investigation protocol:**
1. **Threat surface scan** — full telemetry pull (NWDAF analytics, SBI overview, errors, log stats)
2. **Hypothesis testing** — for each anomaly, generate attack hypotheses and query for confirming/denying evidence
3. **NRF integrity audit** — check every registered NF for rogue UUIDs, attacker IPs, UNDISCOVERABLE status, duplicate types
4. **UE behavioral analysis** — registration burst patterns, IMSI sequencing, NWDAF anomaly correlation

**8 attack patterns (mapped to journal paper scenarios):**
1. DoS / Registration flood — burst detection, AMF overload, IMSI pattern analysis
2. Rogue gNB / MITM — unexpected SCTP sources, unknown cell IDs
3. Rogue NF injection — unknown UUIDs in NRF, IPs outside 127.0.0.x, duplicate NF types
4. NF Profile hijack — UNDISCOVERABLE status, redirected service endpoints
5. UE identity spoofing — AUSF auth failures, credential replay patterns
6. Session hijacking — unauthorized PDU modifications, SMF event mismatches
7. Slice isolation breach — cross-slice traffic, unauthorized S-NSSAI
8. Low-and-slow exfiltration — subtle communication patterns over time

**Output:** Structured threat assessment with threat level (GREEN/YELLOW/ORANGE/RED), kill-chain stage, IoCs, and recommended actions.

**Key distinction from fault agent:** Same AMF burst errors → fault says "race condition bug", threat says "possible registration flood DoS." Same unknown NF → fault says "misconfiguration", threat says "rogue NF injection." The fault agent's benign explanations are challenged with adversarial reasoning.

## Demo Scenario (updated 2026-03-30)
Attack scenario: **Registration flood** (not NRF poisoning).
3-act script: (1) baseline/healthy, (2) normal UE activity, (3) registration flood attack.

## Operating Mode: Split-Pane (not background)

All agents run in their own **split pane** so the operator can visually monitor each agent's output in real time. They are NOT background agents — each gets a visible terminal pane.

Agents run in **loop mode** — they continuously collect data, analyze, share findings, then loop again. No time limits. Agents keep running until the team lead sends a shutdown request.

### Loop cadence (all agents: 30s)
- **monitor**: collect baseline → share → sleep 30s → re-collect → compare with previous → share deltas → repeat
- **analyst**: run NWDAF analytics → assess health → share → sleep 30s → repeat (compare with previous round)
- **fault**: run all 3 detection modes → share → sleep 30s → repeat (track error count trends across rounds)
- **threat**: run 4-phase protocol → share assessment → sleep 30s → repeat (maintain running IoC watchlist across rounds)

### Cross-round state
Each agent should maintain state across loops:
- Previous round's data for comparison (detect deltas, trends, new events)
- Running totals (error counts, UE counts, threat indicators)
- Only report **changes** after the first round — don't repeat the full report every loop

### Alert escalation
- If any agent detects a **significant change** between rounds (new errors, UE count spike, new NF registered, threat level change), immediately message team-lead AND relevant agents — don't wait for the loop to finish
- Routine round summaries go to teammates only; team-lead gets escalations and periodic digests

## Coordination Flow
1. All 4 agents start simultaneously, do their first round in parallel
2. After round 1, all agents share findings with each other
3. Agents loop independently at their own cadence
4. Cross-agent intel: each agent reads messages from others and incorporates into their next round
5. Escalations go to team-lead immediately
6. Team lead sends shutdown_request when operator says stop
- All agents share findings with each other via SendMessage
- Shared taskboard for task tracking and dependencies

## Log Monitoring Technique (proven in session)
```bash
# Check if core is running
LATEST=$(ls -t /home/henokbfg/Documents/5g-testbed-conference/free5gc/log/ | head -1)
SIZE=$(stat -c%s .../log/$LATEST/free5gc.log)
# sleep 3, check size again — if grew, core is active

# Count NFs in log
python3 -c "import re; from collections import Counter; ..."

# Check NWDAF events
grep 'Stored event' .../log/$LATEST/free5gc.log

# Count errors
grep -c 'level=.warning.\|level=.error.' .../log/$LATEST/free5gc.log
```
