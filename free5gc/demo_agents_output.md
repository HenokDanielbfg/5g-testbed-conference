============================================================
  AGENTIC 5G MONITORING DEMO
  (powered by Claude Code)
============================================================

>>> ACT 1: Startup Detection

============================================================
  AGENT: MONITOR
============================================================
  Task: Check the current status of the 5G core network. Is it running? How many NFs are online? Any UEs connected?
  Running...

  [monitor] REPORT:
Here's the full status report:

---

## 🟢 5G Core Network Status: **RUNNING**

**Current run started:** `2026-03-09 13:02:01` (today)
**Log:** Active — last entry at `13:03:51`, static for ~1 min (network is idle, not crashed)

---

### 📡 Network Functions Online: **11 / 11**

| NF | PID | Status |
|---|---|---|
| UPF | 390258 | ✅ Running |
| NRF | 390295 | ✅ Running |
| AMF | 390312 | ✅ Running |
| SMF | 390334 | ✅ Running |
| UDR | 390348 | ✅ Running |
| PCF | 390374 | ✅ Running |
| UDM | 390395 | ✅ Running |
| NSSF | 390409 | ✅ Running |
| AUSF | 390424 | ✅ Running |
| CHF | 390441 | ✅ Running |
| NWDAF | 390460 | ✅ Running |

---

### 📱 Connected UEs: **2 UEs**

| SUPI | Last Seen Activity |
|---|---|
| `imsi-208930000000001` | Registered |
| `imsi-208930000000003` | Active — PDU session setup confirmed (AMF → SMF → UPF PFCP at 13:03:51) |

---

### ⚠️ Notable Issues in Log
- **NRF x509 cert warning** — self-signed cert trust issue (known/recurring)
- **NWDAF** is actively receiving AMF event callbacks (`REGISTRATION_STATE_REPORT`, `CONNECTIVITY_STATE_REPORT`) and storing them — your custom NF is working 🎉
- UE `imsi-208930000000003` just completed a PDU session modification (PFCP session mod accepted by UPF)

**Summary:** All 11 NFs healthy, 2 UEs registered, NWDAF collecting analytics. Network is live but momentarily idle.


>>> ACT 2: Network Analytics

============================================================
  AGENT: ANALYST
============================================================
  Task: Query all available NWDAF analytics endpoints and report on the current state of the network: NF loads, UE activity, and
  Running...

  [analyst] REPORT:
All four endpoints responded successfully. Here is the full network state report:

---

## 📡 NWDAF Network Analytics Report
**Snapshot time:** 2026-03-09 09:04 UTC | **Valid until:** 10:04 UTC

---

### 1. 🖥️ NF Load Levels

| NF | Overall Load | CPU Load | Memory Load | Event Rate | Active UEs |
|----|:-----------:|:--------:|:-----------:|:----------:|:----------:|
| **AMF** | 26% | 21% | 23% | 1/s | 2 |
| **SMF** | ⚠️ 39% | ⚠️ **71%** | ⚠️ **80%** | 2/s | 2 |
| **UPF** | 28% | 44% | 57% | — | — |

**Key observations:**
- **SMF is the most loaded NF** — CPU at 71% and memory at 80% are notably high for just 2 active UEs. This warrants monitoring; if a third UE connects, SMF could become a bottleneck.
- **UPF** shows moderate CPU (44%) and memory (57%) — consistent with its role as the data-plane forwarder. Likely elevated due to the known `queryMultiURR` usage-reporting loop.
- **AMF** is healthy — all metrics well below threshold with 2 registered UEs.

---

### 2. 📶 UE Activity

#### Mobility (UE_MOBILITY)
- **2 unique UEs tracked**, each with **1 location change** event
- Both UEs are **connected but not fully registered** (`isRegistered: false`, `isConnected: true`)

| SUPI | Location Changes | Last Report |
|------|:---------------:|-------------|
| `imsi-208930000000001` | 1 | 13:03:23 local |
| `imsi-208930000000003` | 1 | 13:03:51 local |

> ⚠️ **Note:** Both UEs show `isRegistered: false` — this could indicate they are in a transitional state (e.g., after handover or partial de-registration) or a reporting inconsistency between AMF event data and registration state.

#### Communication Sessions (UE_COMMUNICATION)
- **2 total session establishments**, **1 release**, **1 currently active**

| SUPI | Sessions Created | Released | Active | Last Activity |
|------|:---------------:|:--------:|:------:|---------------|
| `imsi-208930000000001` | 1 | 0 | ✅ **1** | 13:03:24 local |
| `imsi-208930000000003` | 1 | 1 | ❌ 0 | 13:04:18 local |

- `imsi-...0001` has an **active PDU session** — data plane likely up.
- `imsi-...0003` **established then released** its session — normal transient behaviour or UE went idle/detached.

---

### 3. 🚨 Abnormal Behaviour

```
Total Anomalies: 0
Analysis Window: 5 minutes
```

✅ **No abnormal behaviour detected.** The NWDAF found zero anomalies in the last 5-minute rolling window.

---

### 📋 Summary & Recommendations

| Severity | Item | Action |
|----------|------|--------|
| ⚠️ **Medium** | SMF CPU 71% / Mem 80% — high for 2 UEs | Monitor closely; investigate if SMF is retaining stale session state from released sessions |
| ⚠️ **Low** | Both UEs show `isRegistered: false` yet are connected | Cross-check with AMF OAM (`/namf-oam/v1/registered-ue-context`) to confirm actual registration state |
| ℹ️ **Info** | UPF moderate load (44% CPU) | Likely linked to known `queryMultiURR` loop — expected behaviour until that bug is resolved |
| ✅ **OK** | No anomalies detected | Network operating within normal parameters |


>>> ACT 3: Fault Detection

============================================================
  AGENT: FAULT
============================================================
  Task: Scan the latest log file for errors and warnings. Group by NF, classify severity, and report any issues found. Distingui
  Running...

  [fault] REPORT:
I now have everything I need. Here is the full fault diagnosis report.

---

## 🔍 FAULT AGENT — Diagnostic Report
**Log:** `20260309_130201/free5gc.log` | **Session:** 13:02:01 → 13:05:34 | **Lines:** 1,377

---

### ✅ NF Status Overview

All **11 NFs started successfully** — no crashes, no panics, no `FATAL` entries detected.

| NF | Started | Errors | Warnings | Status |
|---|---|---|---|---|
| UPF | ✅ | 0 | 11 | MEDIUM (recurring) |
| NRF | ✅ | 0 | 146 | MEDIUM (recurring) |
| AMF | ✅ | 0 | 3 | ⚠️ MEDIUM (new) |
| SMF | ✅ | 0 | 1 | ⚠️ MEDIUM (new) |
| UDM | ✅ | 0 | 0 | 🟢 Clean |
| UDR | ✅ | 0 | 0 | 🟢 Clean |
| AUSF | ✅ | 0 | 0 | 🟢 Clean |
| PCF | ✅ | 0 | 0 | 🟢 Clean |
| NSSF | ✅ | 0 | 3 | ⚠️ MEDIUM (new) |
| CHF | ✅ | 3 | 3 | MEDIUM (recurring) |
| NWDAF | ✅ | 0 | 0 | 🟢 Clean |

---

### 🔴 CRITICAL Issues
> None. No NF crashes detected.

---

### 🟠 HIGH Issues
> None detected.

---

### 🟡 MEDIUM Issues — **NEW** (require attention)

#### 1. `NSSF` — Missing TAC entries in slice configuration
```
[13:03:23] No TA {"plmnId":{"mcc":"208","mnc":"93"},"tac":"000004"} in NSSF configuration
[13:03:51] No TA {"plmnId":{"mcc":"208","mnc":"93"},"tac":"000003"} in NSSF configuration
[13:04:32] No TA {"plmnId":{"mcc":"208","mnc":"93"},"tac":"000003"} in NSSF configuration
```
**Diagnosis:** TACs `000003` and `000004` are not defined in `nssfcfg.yaml`. UEs registering from those tracking areas can't get proper network slice selection from NSSF.

#### 2. `AMF` — NSSF slice lookup failing, falling back to default NRF
```
[13:03:23] nsiInformation is still nil, use default NRF[http://127.0.0.10:8000] — imsi-208930000000001
[13:03:51] nsiInformation is still nil, use default NRF[http://127.0.0.10:8000] — imsi-208930000000003
[13:04:32] nsiInformation is still nil, use default NRF[http://127.0.0.10:8000] — imsi-208930000000003
```
**Diagnosis:** Direct consequence of the NSSF TAC mis-config above. AMF gracefully falls back to the default NRF, so registration/sessions ultimately succeed — but **slice-aware routing is bypassed**. For a conference demo, this means slice selection is effectively disabled for any UE coming from TAC 3 or 4.

#### 3. `SMF` — PDU session state mismatch for `imsi-208930000000003`
```
[13:04:18] Unexpected state, expect: [InActive], actual:[Active] — pdu_session_id=1, supi=imsi-208930000000003
```
**Diagnosis:** SMF attempted to create a PDU session for a UE that already had an active one (UE re-attached without clean deregistration). The session was **successfully re-established** at 13:04:32 (confirmed by `create smContext Success`), so this is a transient race condition — no lasting impact, but worth monitoring if it recurs.

---

### 🔵 MEDIUM Issues — **Known / Recurring** (no action needed)

| NF | Issue | Count | Classification |
|---|---|---|---|
| **UPF** | `queryMultiURR invalid argument` — usage reporting broken every ~10s | 11 | Known, MEDIUM |
| **NRF** | `x509: certificate signed by unknown authority` — self-signed cert | 146 | Known, MEDIUM |
| **CHF** | `Login to Webconsole FTP fail` + `Charging gateway fail to send CDR` — billing FTP at 127.0.0.1:2121 not running | 6 | Known, MEDIUM |

---

### ✅ NWDAF Health Check
NWDAF is **fully operational** — no errors or warnings. Actively receiving and storing AMF event notifications:
- `REGISTRATION_STATE_REPORT`, `CONNECTIVITY_STATE_REPORT` events flowing in
- 14 events stored as of end of log session
- Callback endpoint at `http://127.0.0.47:8000/nnwdaf-callback/v1/amf-event-notify` responding `204 No Content` ✅

---

### 🔧 Actions Taken
**No NF restarts required** — all 11 NFs are running and healthy.

---

### 📋 Recommended Follow-Up Actions

| Priority | Action | Target |
|---|---|---|
| 🟡 Medium | Add TACs `000003` and `000004` to `config/nssfcfg.yaml` under `supportedTAList` | NSSF |
| 🟡 Medium | Monitor `imsi-208930000000003` — if PDU session state mismatch recurs repeatedly, force UE re-registration | SMF |
| 🔵 Low | Start FTP billing server on `127.0.0.1:2121` if CDR archiving is needed for the demo | CHF |


>>> ACT 4: Fault Injection
   Killing SMF process...

============================================================
  AGENT: FAULT
============================================================
  Task: ALERT: A network function may have crashed. Check which NFs are running (pgrep), compare to expected list (nrf, amf, smf
  Running...

  [fault] REPORT:
SMF is back up and healthy — registered with NRF and re-associated with UPF. Here is the full fault report:

---

## 🔍 Fault Report — 2026-03-09 13:07 UTC+4

### NF Process Inventory
| NF | Expected | Was Running | Status |
|---|---|---|---|
| NRF | ✅ | ✅ | OK |
| AMF | ✅ | ✅ | OK |
| **SMF** | ✅ | ❌ | **⚠️ DOWN → RESTARTED** |
| UPF | ✅ | ✅ | OK |
| UDR | ✅ | ✅ | OK |
| PCF | ✅ | ✅ | OK |
| UDM | ✅ | ✅ | OK |
| NSSF | ✅ | ✅ | OK |
| AUSF | ✅ | ✅ | OK |
| CHF | ✅ | ✅ | OK |
| NWDAF | ✅ | ✅ | OK |

---

### Error Summary by NF

| NF | Severity | Issue | Classification |
|---|---|---|---|
| **SMF** | 🔴 **CRITICAL** | Process not running — graceful `SIGTERM` shutdown at 13:06:22, no panic | **NEW — Crashed/Killed** |
| UPF | 🟡 MEDIUM | `queryMultiURR invalid argument` every ~10s (usage reporting loop) | *Known recurring* |
| NRF | 🟡 MEDIUM | `x509: certificate signed by unknown authority` — self-signed cert | *Known recurring* |
| CHF | 🟡 MEDIUM | `connection refused` to `127.0.0.1:2121` — billing FTP not running | *Known recurring* |
| NSSF | 🟡 MEDIUM | `No TA {"tac":"000003"}` in configuration — non-configured TAC | Minor / possibly expected |

---

### New / Critical Issues

- **SMF was down** — terminated gracefully at `13:06:22` via `SIGTERM`. No panic or runtime error was found in the log; this appears to have been a clean signal-induced shutdown (possibly manual kill or OS signal). **Not a code crash.**

---

### ✅ Actions Taken

1. **Detected**: SMF absent from `pgrep` output (10 of 11 NFs running)
2. **Diagnosed**: Log shows clean `SIGTERM` shutdown — SMF deregistered from NRF gracefully at 13:06:22
3. **Restarted**: `./bin/smf -c ./config/smfcfg.yaml -l ./log/20260309_130201/free5gc.log &` → **PID 393745**
4. **Verified**: SMF re-registered with NRF, PFCP re-associated with UPF (`127.0.0.8`), SBI listening on `127.0.0.2:8000` ✅

All 11 NFs are now operational.


>>> ACT 5: Recovery Confirmation

============================================================
  AGENT: MONITOR
============================================================
  Task: Verify that all NFs are back online after a recent crash recovery. Check processes and log activity.
  Running...

  [monitor] REPORT:
Here's the full post-crash recovery report:

---

## ✅ 5G Core Status: **RUNNING** (11/11 NFs Online)

### 🟢 Active Processes — All NFs Confirmed Up
| NF | PID | Status |
|---|---|---|
| UPF | 390258 | ✅ Running |
| NRF | 390295 | ✅ Running |
| AMF | 390312 | ✅ Running |
| UDR | 390348 | ✅ Running |
| PCF | 390374 | ✅ Running |
| UDM | 390395 | ✅ Running |
| NSSF | 390409 | ✅ Running |
| AUSF | 390424 | ✅ Running |
| CHF | 390441 | ✅ Running |
| NWDAF | 390460 | ✅ Running |
| SMF | 393745 | ✅ Running (last to register) |

> ⚠️ Note: SMF has a notably higher PID (`393745` vs ~`390xxx` for others) — it was the **last NF to come online**, consistent with its dependency on NRF + UPF being ready first.

---

### 📄 Active Log Run: `20260309_130201`
- **Log file:** `/log/20260309_130201/free5gc.log`
- **Growth delta (2s sample):** `0 bytes` — log is **static/idle**, no active traffic at this moment. This is normal post-startup with no UE activity.
- **Latest log entry:** `13:07:02` — SMF ↔ UPF **PFCP Association Setup** completed successfully ✅

---

### 📱 UE Registrations Found
| SUPI | Status |
|---|---|
| `imsi-208930000000001` | Previously registered (in log history) |
| `imsi-208930000000003` | Previously registered (in log history) |

> ⚠️ These SUPIs appear in log history from **before the crash**. No **new** UE registrations have occurred in this session yet — both UEs are likely **disconnected** and would need to re-attach.

---

### 🔍 Startup Health Notes
- **SMF → NRF registration:** ✅ `201 Created` (profile `58f32dab...`)
- **SMF → UPF PFCP association:** ✅ `Association Setup Accepted`
- **UPF session cleanup:** Old pre-crash sessions (`CPSEID 0x1, 0x3`) were **properly cleared** on restart — clean slate confirmed.
- **Known recurring issues still present:** UPF `queryMultiURR` / CHF FTP / NRF cert warnings are expected per known issues.

---

**Summary:** Full recovery confirmed. All 11 NFs are up and NRF-registered. The core is idle, awaiting UE re-registration. No new UEs have connected since restart.


============================================================
  CONSOLIDATED REPORT
============================================================

--- Monitor ---
Here's the full status report:

---

## 🟢 5G Core Network Status: **RUNNING**

**Current run started:** `2026-03-09 13:02:01` (today)
**Log:** Active — last entry at `13:03:51`, static for ~1 min (network is idle, not crashed)

---

### 📡 Network Functions Online: **11 / 11**

| NF | PID | Status |
|---|---|---|
| UPF | 390258 | ✅ Running |
| NRF | 390295 | ✅ Running |
| AMF | 390312 | ✅ Running |
| SMF | 390334 | ✅ Running |
| UDR | 390348 | ✅ Running |
| PCF | 390374 | ✅ Running |
| UDM | 390395 | ✅ Running |
| NSSF | 390409 | ✅ Running |
| AUSF | 390424 | ✅ Running |
| CHF | 390441 | ✅ Running |
| NWDAF | 390460 | ✅ Running |

---

### 📱 Connected UEs: **2 UEs**

| SUPI | Last Seen Activity |
|---|---|
| `imsi-208930000000001` | Registered |
| `imsi-208930000000003` | Active — PDU session setup confirmed (AMF → SMF → UPF PFCP at 13:03:51) |

---

### ⚠️ Notable Issues in Log
- **NRF x509 cert warning** — self-signed cert trust issue (known/recurring)
- **NWDAF** is actively receiving AMF event callbacks (`REGISTRATION_STATE_REPORT`, `CONNECTIVITY_STATE_REPORT`) and storing them — your custom NF is working 🎉
- UE `imsi-208930000000003` just completed a PDU session modification (PFCP session mod accepted by UPF)

**Summary:** All 11 NFs healthy, 2 UEs registered, NWDAF collecting analytics. Network is live but momentarily idle.

--- Analyst ---
All four endpoints responded successfully. Here is the full network state report:

---

## 📡 NWDAF Network Analytics Report
**Snapshot time:** 2026-03-09 09:04 UTC | **Valid until:** 10:04 UTC

---

### 1. 🖥️ NF Load Levels

| NF | Overall Load | CPU Load | Memory Load | Event Rate | Active UEs |
|----|:-----------:|:--------:|:-----------:|:----------:|:----------:|
| **AMF** | 26% | 21% | 23% | 1/s | 2 |
| **SMF** | ⚠️ 39% | ⚠️ **71%** | ⚠️ **80%** | 2/s | 2 |
| **UPF** | 28% | 44% | 57% | — | — |

**Key observations:**
- **SMF is the most loaded NF** — CPU at 71% and memory at 80% are notably high for just 2 active UEs. This warrants monitoring; if a third UE connects, SMF could become a bottleneck.
- **UPF** shows moderate CPU (44%) and memory (57%) — consistent with its role as the data-plane forwarder. Likely elevated due to the known `queryMultiURR` usage-reporting loop.
- **AMF** is healthy — all metrics well below threshold with 2 registered UEs.

---

### 2. 📶 UE Activity

#### Mobility (UE_MOBILITY)
- **2 unique UEs tracked**, each with **1 location change** event
- Both UEs are **connected but not fully registered** (`isRegistered: false`, `isConnected: true`)

| SUPI | Location Changes | Last Report |
|------|:---------------:|-------------|
| `imsi-208930000000001` | 1 | 13:03:23 local |
| `imsi-208930000000003` | 1 | 13:03:51 local |

> ⚠️ **Note:** Both UEs show `isRegistered: false` — this could indicate they are in a transitional state (e.g., after handover or partial de-registration) or a reporting inconsistency between AMF event data and registration state.

#### Communication Sessions (UE_COMMUNICATION)
- **2 total session establishments**, **1 release**, **1 currently active**

| SUPI | Sessions Created | Released | Active | Last Activity |
|------|:---------------:|:--------:|:------:|---------------|
| `imsi-208930000000001` | 1 | 0 | ✅ **1** | 13:03:24 local |
| `imsi-208930000000003` | 1 | 1 | ❌ 0 | 13:04:18 local |

- `imsi-...0001` has an **active PDU session** — data plane likely up.
- `imsi-...0003` **established then released** its session — normal transient behaviour or UE went idle/detached.

---

### 3. 🚨 Abnormal Behaviour

```
Total Anomalies: 0
Analysis Window: 5 minutes
```

✅ **No abnormal behaviour detected.** The NWDAF found zero anomalies in the last 5-minute rolling window.

---

### 📋 Summary & Recommendations

| Severity | Item | Action |
|----------|------|--------|
| ⚠️ **Medium** | SMF CPU 71% / Mem 80% — high for 2 UEs | Monitor closely; investigate if SMF is retaining stale session state from released sessions |
| ⚠️ **Low** | Both UEs show `isRegistered: false` yet are connected | Cross-check with AMF OAM (`/namf-oam/v1/registered-ue-context`) to confirm actual registration state |
| ℹ️ **Info** | UPF moderate load (44% CPU) | Likely linked to known `queryMultiURR` loop — expected behaviour until that bug is resolved |
| ✅ **OK** | No anomalies detected | Network operating within normal parameters |

--- Fault ---
I now have everything I need. Here is the full fault diagnosis report.

---

## 🔍 FAULT AGENT — Diagnostic Report
**Log:** `20260309_130201/free5gc.log` | **Session:** 13:02:01 → 13:05:34 | **Lines:** 1,377

---

### ✅ NF Status Overview

All **11 NFs started successfully** — no crashes, no panics, no `FATAL` entries detected.

| NF | Started | Errors | Warnings | Status |
|---|---|---|---|---|
| UPF | ✅ | 0 | 11 | MEDIUM (recurring) |
| NRF | ✅ | 0 | 146 | MEDIUM (recurring) |
| AMF | ✅ | 0 | 3 | ⚠️ MEDIUM (new) |
| SMF | ✅ | 0 | 1 | ⚠️ MEDIUM (new) |
| UDM | ✅ | 0 | 0 | 🟢 Clean |
| UDR | ✅ | 0 | 0 | 🟢 Clean |
| AUSF | ✅ | 0 | 0 | 🟢 Clean |
| PCF | ✅ | 0 | 0 | 🟢 Clean |
| NSSF | ✅ | 0 | 3 | ⚠️ MEDIUM (new) |
| CHF | ✅ | 3 | 3 | MEDIUM (recurring) |
| NWDAF | ✅ | 0 | 0 | 🟢 Clean |

---

### 🔴 CRITICAL Issues
> None. No NF crashes detected.

---

### 🟠 HIGH Issues
> None detected.

---

### 🟡 MEDIUM Issues — **NEW** (require attention)

#### 1. `NSSF` — Missing TAC entries in slice configuration
```
[13:03:23] No TA {"plmnId":{"mcc":"208","mnc":"93"},"tac":"000004"} in NSSF configuration
[13:03:51] No TA {"plmnId":{"mcc":"208","mnc":"93"},"tac":"000003"} in NSSF configuration
[13:04:32] No TA {"plmnId":{"mcc":"208","mnc":"93"},"tac":"000003"} in NSSF configuration
```
**Diagnosis:** TACs `000003` and `000004` are not defined in `nssfcfg.yaml`. UEs registering from those tracking areas can't get proper network slice selection from NSSF.

#### 2. `AMF` — NSSF slice lookup failing, falling back to default NRF
```
[13:03:23] nsiInformation is still nil, use default NRF[http://127.0.0.10:8000] — imsi-208930000000001
[13:03:51] nsiInformation is still nil, use default NRF[http://127.0.0.10:8000] — imsi-208930000000003
[13:04:32] nsiInformation is still nil, use default NRF[http://127.0.0.10:8000] — imsi-208930000000003
```
**Diagnosis:** Direct consequence of the NSSF TAC mis-config above. AMF gracefully falls back to the default NRF, so registration/sessions ultimately succeed — but **slice-aware routing is bypassed**. For a conference demo, this means slice selection is effectively disabled for any UE coming from TAC 3 or 4.

#### 3. `SMF` — PDU session state mismatch for `imsi-208930000000003`
```
[13:04:18] Unexpected state, expect: [InActive], actual:[Active] — pdu_session_id=1, supi=imsi-208930000000003
```
**Diagnosis:** SMF attempted to create a PDU session for a UE that already had an active one (UE re-attached without clean deregistration). The session was **successfully re-established** at 13:04:32 (confirmed by `create smContext Success`), so this is a transient race condition — no lasting impact, but worth monitoring if it recurs.

---

### 🔵 MEDIUM Issues — **Known / Recurring** (no action needed)

| NF | Issue | Count | Classification |
|---|---|---|---|
| **UPF** | `queryMultiURR invalid argument` — usage reporting broken every ~10s | 11 | Known, MEDIUM |
| **NRF** | `x509: certificate signed by unknown authority` — self-signed cert | 146 | Known, MEDIUM |
| **CHF** | `Login to Webconsole FTP fail` + `Charging gateway fail to send CDR` — billing FTP at 127.0.0.1:2121 not running | 6 | Known, MEDIUM |

---

### ✅ NWDAF Health Check
NWDAF is **fully operational** — no errors or warnings. Actively receiving and storing AMF event notifications:
- `REGISTRATION_STATE_REPORT`, `CONNECTIVITY_STATE_REPORT` events flowing in
- 14 events stored as of end of log session
- Callback endpoint at `http://127.0.0.47:8000/nnwdaf-callback/v1/amf-event-notify` responding `204 No Content` ✅

---

### 🔧 Actions Taken
**No NF restarts required** — all 11 NFs are running and healthy.

---

### 📋 Recommended Follow-Up Actions

| Priority | Action | Target |
|---|---|---|
| 🟡 Medium | Add TACs `000003` and `000004` to `config/nssfcfg.yaml` under `supportedTAList` | NSSF |
| 🟡 Medium | Monitor `imsi-208930000000003` — if PDU session state mismatch recurs repeatedly, force UE re-registration | SMF |
| 🔵 Low | Start FTP billing server on `127.0.0.1:2121` if CDR archiving is needed for the demo | CHF |

============================================================
  DEMO COMPLETE
============================================================

