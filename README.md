# 5G Testbed Conference

This repository provides a modular and fully functional 5G testbed built for research and experimentation. It integrates open-source tools like Free5GC, UERANSIM, and Prometheus to simulate a real-world 5G environment capable of handling mobility, session management, and advanced network analytics.

---

## 🚀 Project Objectives

- Simulate realistic 5G core and RAN behaviors.
- Test UE mobility, handovers, and session management.
- Collect and analyze network data.
- Provide a research-ready, extendable platform.

---

## 🧩 Key Components

- **Free5GC**: Implements the 5G Core (AMF, SMF, UPF, PCF, UDM, etc.).
- **UERANSIM**: Simulates the gNB and UE behaviors including handover and movement.
- **NWDAF**: Performs real-time analytics on registration state, mobility, and PDU sessions.
- **Prometheus**: Scrapes and exposes NWDAF metrics for visualization.

---

## 🖥️ System Requirements

- **OS**: Ubuntu 20.04 (Kernel: 5.0.0–5.4.x recommended)
- **Languages**: Go (v1.21.8), Python, C++
- **Dependencies**:
  - Prometheus 2.41+
  - gtp5g v0.86–<0.9.0

---

## 🏗️ Architecture Overview
```
   +-------------+       +-------------+ 
   |   UERANSIM  | <---> |   Free5GC   | 
   | (gNB & UE)  |       |   Core NF   | 
   +-------------+       |     NWDAF   |
                         +-------------+

```
- UEs register and move in a defined region.
- AMF/SMF expose events to NWDAF.
- NWDAF publishes metrics to Prometheus.

---

## 📦 Repository Structure
```
5g-testbed-conference/ 
├── free5gc/ # Core Network 
├── UERANSIM/ # RAN & UE Simulation 
├── mnc_NWDAF-main/ # NWDAF with Prometheus integration 
├── prometheus/ # Prometheus configs 
├── core dataset/ # Collected metrics 
├── handover_live_visual.py # Handover visualization tool 
├── mobility_live_visual.py # Mobility visualization tool 
├── join_leavev2.py # Dynamic UE attach/detach simulation 
├── setup.sh # Environment setup script 
├── cleanup.sh # Cleanup/reset script 
└── README.md
```

---

## ⚙️ Installation Guide

### 1. Clone the repository

```bash
git clone https://github.com/HenokDanielbfg/5g-testbed-conference.git
cd 5g-testbed-conference

2. Install & configure Free5GC

    Follow steps in Free5GC Install Guide

    Use gtp5g version 0.86–<0.9.0

    Edit model_smf_event.go to add custom SMF events.

    Run with:

cd free5gc
./run.sh

3. Install & run UERANSIM

cd UERANSIM
git checkout e4c492d
make

    Configure multiple gNBs and UEs via YAML files.

    Subscribe UEs using Free5GC WebConsole.

4. Install Prometheus

wget https://github.com/prometheus/prometheus/releases/download/v2.41.0/prometheus-2.41.0.linux-amd64.tar.gz
tar -xvf prometheus-2.41.0.linux-amd64.tar.gz
cd prometheus-2.41.0.linux-amd64
sudo ./prometheus --config.file=prometheus.yml --storage.tsdb.retention.time=10y

5. Start NWDAF

cd mnc_NWDAF-main/NWDAF
go run nwdaf.go

🛠️ Event Subscription System

    AMF: Events like registration, mobility, location, and access-type changes.

    SMF: PDU session events and QoS updates.

NWDAF uses RESTful APIs to subscribe/unsubscribe to these events and processes real-time notifications for analytics.
📊 Prometheus Metrics

Sample metrics exposed by NWDAF:
Metric Name	Description	Labels
amf_ue_registration_state	UE registration state (1 = active)	supi
active_UEs	Number of active UEs	state
UE_location_report	Logs UE handover/mobility events	supi, NrCellId, time, tac
active_pdu_sessions	Number of active PDU sessions	state
total_pdu_session_events	PDU session event counts	supi, PDU_ID, Est_or_Rel
🧠 Mobility Model: ABMM

An Activity-Based Mobility Model (ABMM) simulates realistic user behavior. UEs:

    Choose destinations (home, work, cafes, etc.) based on time-of-day

    Move with realistic speeds

    Trigger handovers and location updates

Mobility logic is handled in udp_task.cpp with Prometheus metrics for every transition.
🔧 Usage
Start the Testbed

# Core
cd free5gc && ./run.sh

# NWDAF
cd mnc_NWDAF-main/NWDAF && go run nwdaf.go

# Prometheus
cd prometheus && sudo ./prometheus --config.file=prometheus.yml --storage.tsdb.retention.time=10y

# RAN (GNB)
build/nr-gnb -c config/gnb.yaml

# UE
sudo build/nr-ue -c config/ue.yaml

# Or dynamic attach/detach
python3 join_leavev2.py

📈 Visualization Tools

    handover_live_visual.py – Live display of UE handovers.

    mobility_live_visual.py – Tracks UE location over time.

📂 Configuration Notes

    Free5GC config: free5gc/config/

    NWDAF config: mnc_NWDAF-main/NWDAF/config.yaml

    UE & gNB: UERANSIM/config/

Make sure identifiers like SUPI (UE) and NCI (gNB) are unique.
🧪 Sample Prometheus Query (Python)

from prometheus_api_client import PrometheusConnect

prom = PrometheusConnect(url="http://localhost:9090", disable_ssl=True)
response = prom.custom_query_range(
    query="amf_ue_registration_state",
    start_time=start_time,
    end_time=end_time,
    step="10m"
)

👥 Acknowledgments

    Free5GC

    UERANSIM

    Prometheus

    NWDAF Base
