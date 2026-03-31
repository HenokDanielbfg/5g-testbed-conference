---
name: journal-paper-plan
description: Journal paper plan — LLM-driven autonomous 5G network management via NWDAF, vision/architecture paper with security validation scenarios
type: project
---

## Paper Framing
- **Title direction:** "LLM-Driven Autonomous Network Management via NWDAF"
- **Paper type:** Vision/architecture paper — present proposed architecture, validate with use case scenarios, call for future research
- **Thesis:** LLM agents, connected to standardized 3GPP interfaces (NWDAF), can perform network diagnostics and operational tasks that traditionally require human engineers
- **Novel contribution:** Using MCP as the bridge between a live 3GPP-compliant NWDAF and LLM agents that autonomously reason about network state

## Paper Structure (agreed)
1. **Introduction** — 5G complexity, NWDAF gap (produces analytics but humans still interpret), vision of LLM agents as NWDAF consumers
2. **Background & Related Work** — NWDAF (Rel-16/17), LLMs for networking (mostly offline today), MCP as emerging standard, gap: nobody connects all three live
3. **Proposed Architecture** — 5G Core → NWDAF → MCP servers → LLM Agent Team; detail each layer, design principles
4. **Implementation** — free5gc + custom NWDAF, modified AMF/SMF, UERANSIM, Go + Python MCP servers, Claude agent teams
5. **Validation Scenario** — security-focused (see below)
6. **Discussion & Future Research** — closed-loop control, multi-model architectures, security of LLM+SBI access, standardization (MCP in 3GPP?), scalability

## Validation Approach
Security-focused: common 5G attack types ranked easy → hard, with root cause analysis, benchmarked against existing solutions.

### Attack Scenarios (easy → hard)
1. **DoS / Registration flood** — mass UE registrations overwhelm AMF → NF_LOAD spike, abnormal registration rate
2. **Rogue gNB / MITM** — fake gNB connects to AMF → unexpected N2 source, UE_MOBILITY anomalies
3. **UE identity spoofing (SUPI/SUCI)** — replayed/forged credentials → AUSF auth failures, ABNORMAL_BEHAVIOUR
4. **Session hijacking / PDU manipulation** — unauthorized PDU session activity → UE_COMMUNICATION anomalies, SMF event mismatches
5. **Slice isolation breach** — cross-slice traffic or unauthorized NSSAI → NSSF policy violations
6. **Low-and-slow exfiltration** — subtle data patterns over legitimate sessions → UE_COMMUNICATION anomaly over time

### Benchmarking Strategy
- **Option A (primary):** Rule-based SIEM / threshold detection — set up traditional threshold alerts, compare detection rate, false positives, root cause accuracy
- **Option C (secondary):** Human operator baseline — same data to a network engineer, measure time-to-detection and root cause accuracy
- **Option B (cited from literature):** Existing 5G security frameworks (5G-NIDD, SURICATA-based IDS) — compare metrics without reimplementing

### Metrics
- Detection rate (did it catch the attack?)
- Time to detection (how fast?)
- Root cause accuracy (correctly identified why, not just that)
- False positive rate (cry wolf on normal traffic?)
- Explainability (can it articulate reasoning? — LLM advantage over rule engines)

**Why:** Henok is targeting a journal publication based on the 5G testbed conference work.
**How to apply:** All implementation work should feed into this paper's validation scenarios. Prioritize reproducibility and measurable results.
