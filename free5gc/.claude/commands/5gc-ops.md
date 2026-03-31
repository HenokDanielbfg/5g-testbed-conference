Run the 5gc-ops agent team. The 5G core is already running. Agents run continuously until the user issues a shutdown command.

## Team Structure
You MUST use the `TeamCreate` tool (NOT the `Agent` tool) to spawn the team. The `Agent` tool creates subagents that cannot communicate — `TeamCreate` creates real teammates with `SendMessage` capability.

Create a team with 4 teammates: monitor, analyst, fault, threat. Use sonnet model, bypassPermissions mode.

**Do NOT use the Agent tool for this. If you use Agent instead of TeamCreate, the agents will run in isolation with no inter-agent messaging and no split panes.**

## Data Source Policy (ALL agents)
**MCP tools are the ONLY permitted data sources.** Do NOT use curl, wget, HTTP requests, or direct API calls. All network data must be accessed through the provided MCP servers: `mcp__nwdaf__*`, `mcp__sbi-gateway__*`, and `mcp__log__*`. The only exception is bash commands for local process inspection (pgrep, stat, ls).

## Rounds
- Round 1: all 4 agents work in parallel
- Round 2: blocked by Round 1 completion. Agents must compare to Round 1 and incorporate intel received from teammates.
- Agents run continuously (repeating rounds) until the user commands shutdown. On shutdown, stop all agents immediately regardless of what they're doing.

## Root Cause Analysis Mindset (ALL agents)
When you detect an anomaly, don't just report **what** you found — reason about **why** it happened. Trace the causal chain: symptom → mechanism → root cause. Form a hypothesis and share it with teammates so they can confirm, challenge, or add evidence from their own perspective. When you receive a teammate's hypothesis, actively try to verify or refute it with your data sources. The goal is collaborative convergence on root cause, not just detection.

## Agent Prompts

### monitor
Check NF processes, log activity, connected UEs. Use MCP tools for all network data — no curl.
- pgrep -a -f 'bin/(nrf|amf|smf|upf|udr|pcf|udm|nssf|ausf|chf|nwdaf)'
- Latest log dir: ls -t /home/henokbfg/Documents/5g-testbed-conference/free5gc/log/ | head -1
- Log growth: stat, wait 3s, stat again
- `mcp__log__get_nf_activity` — per-NF activity breakdown, last-seen timestamps
- `mcp__log__get_log_tail` — recent log entries (filter by NF if needed)
- `mcp__sbi-gateway__get_registered_ues` — connected UE contexts
- `mcp__sbi-gateway__get_network_overview` — overall network state
- **RCA mindset**: When you find a missing process or anomaly, reason about **why** it happened (crash? killed? never started? resource exhaustion? adversarial action?) and share your hypothesis with teammates.

### analyst
Query all 5 NWDAF analytics via MCP tools — no curl:
- `mcp__nwdaf__get_nf_load` — NF load, CPU, memory (AMF, SMF, UPF)
- `mcp__nwdaf__get_ue_mobility` — UE registration, connectivity, location changes
- `mcp__nwdaf__get_ue_communication` — PDU session analytics
- `mcp__nwdaf__get_anomalies` — abnormal behaviour detection
- `mcp__nwdaf__get_recommendations` — prediction model recommendations
- Or use `mcp__nwdaf__get_all_analytics` to query all 5 in one call
- **RCA mindset**: When analytics show anomalies (load spikes, abnormal behaviour, unusual UE patterns), reason about what network condition or action could produce those numbers. Share your causal hypothesis with teammates — e.g., "AMF load spiked to 95% which suggests a registration burst, possibly DoS" not just "AMF load is high."

### fault (3 detection modes — must run ALL three)
Use MCP tools for all network data — no curl.

**Mode 1 — Process Audit:** pgrep all 11 NFs. Any missing = CRITICAL. Restart missing NFs: ./bin/<nf> -c ./config/<nf>cfg.yaml -l <log_path>/free5gc.log &

**Mode 2 — Log Scan:** Use `mcp__log__get_errors` for errors/warnings grouped by NF. Use `mcp__log__search_log` for specific patterns. Known recurring (MEDIUM): UPF queryMultiURR, NRF x509 cert, CHF FTP refused.

**Mode 3 — NRF Integrity Check:** Use `mcp__sbi-gateway__get_network_overview` to list registered NFs, then `mcp__sbi-gateway__get_nf_profile` for each NF's details. Flag CRITICAL: unknown UUIDs (e.g. deadbeef), IPs outside 127.0.0.x, status UNDISCOVERABLE, duplicate NF types. Expected IPs: NRF=127.0.0.10, AMF=127.0.0.18, SMF=127.0.0.2, UPF=127.0.0.8. If SBI gateway returns auth errors, fall back to `mcp__log__search_log` for PUT /nnrf-nfm/v1/nf-instances entries.

**RCA mindset**: When you find integrity issues, trace the root cause: How did this entry get into the NRF? What API was exploited? What authentication gap allowed it? Share your causal analysis with teammates — don't just flag the symptom.

### threat
Adversarial analysis of the live 5G core. Use MCP tools exclusively — no curl. You are the paranoid one: every anomaly is a potential indicator of compromise until proven benign.

**Phase 1 — Threat Surface Scan:**
- `mcp__nwdaf__get_all_analytics` — full NWDAF snapshot
- `mcp__sbi-gateway__get_network_overview` — current UE/session/NF state
- `mcp__log__get_errors` — error patterns by NF
- `mcp__log__get_log_stats` — volume anomalies

**Phase 2 — Threat Hypothesis Testing:**
For each anomaly, generate adversarial hypotheses. What attack could produce this signal? Query specific MCP tools to confirm/deny.

**Phase 3 — NRF Integrity Audit (critical — SBA trust anchor):**
- `mcp__sbi-gateway__get_nf_profile` for each NF — check for unknown UUIDs, IPs outside 127.0.0.x, UNDISCOVERABLE status, duplicate NF types, rogue service endpoints.
- Expected IPs: NRF=127.0.0.10, AMF=127.0.0.18, SMF=127.0.0.2, UPF=127.0.0.8

**Phase 4 — UE Behavioral Analysis:**
- `mcp__nwdaf__get_anomalies` — RAPID_REREGISTRATION, SESSION_FLAPPING
- `mcp__sbi-gateway__get_registered_ues` — check for sequential IMSIs, burst patterns
- Cross-reference UE count vs baseline from monitor agent

**Attack taxonomy** (what you're looking for): DoS/registration flood, rogue gNB/MITM, rogue NF injection, NF profile hijack, SUPI spoofing, session hijacking, slice isolation breach, low-and-slow exfiltration.

**RCA mindset**: Don't just detect — trace the kill chain: reconnaissance → access → exploitation → persistence → impact. When you find an IoC, reason backwards: what vulnerability was exploited, what access was required, what was the attacker's likely method. Share causal hypotheses with teammates. When a fault or monitor agent says "benign", challenge it with adversarial reasoning.

**Output**: Produce a Threat Assessment with threat level (GREEN/YELLOW/ORANGE/RED), active threats with confidence and kill chain stage, IoCs, and cleared concerns.

## Cross-Agent Collaboration
Every agent MUST share findings with all teammates via SendMessage after completing each task:
- monitor -> analyst, fault, threat: UE count, NF status, log activity rate
- analyst -> monitor, fault, threat: UE session status, NF load data, abnormal behaviour alerts, recommendations
- fault -> monitor, analyst, threat: crashed NFs, restarts, rogue NFs, integrity issues
- threat -> monitor, analyst, fault: threat level, IoCs, attack hypotheses, kill chain analysis

When an agent RECEIVES a message from a teammate, it must incorporate that intel into its next round of work. Cross-reference, verify, and challenge each other's findings. If a teammate shares a root cause hypothesis, actively try to confirm or refute it from your own data sources and share what you find back. The threat agent should challenge "benign" explanations; the fault agent should provide alternative non-adversarial explanations to threat findings.

## Consolidated Report
After shutdown, deliver a single consolidated report with sections: Network Status, Analytics, Faults/Security, Threat Assessment, Cross-Agent Findings, Root Cause Analysis (for any anomalies: the causal chain from symptom → mechanism → root cause, noting which agents contributed which evidence), Outstanding Risks.
