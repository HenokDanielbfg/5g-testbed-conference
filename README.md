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

### Option 1. Download the file "setup.sh", make it executable(chmod +x setup.sh) and run it(./setup.sh)

### Option 2. Manual System setup

1. **Update packages & install basic tools**
   ```bash
   sudo apt update
   sudo apt install -y git wget curl gnupg software-properties-common
   ```

2. **Clone repository**
   ```bash
   cd ~
   git clone https://github.com/HenokDanielbfg/5g-testbed-conference.git
   cd 5g-testbed-conference
   ```

3. **Install Go**
   ```bash
   wget https://dl.google.com/go/go1.21.8.linux-amd64.tar.gz
   sudo tar -C /usr/local -zxvf go1.21.8.linux-amd64.tar.gz
   mkdir -p ~/go/{bin,pkg,src}
   ```

4. **Configure Go environment**
   Add the following lines to `~/.bashrc`:
   ```bash
   export GOPATH=$HOME/go
   export GOROOT=/usr/local/go
   export PATH=$PATH:$GOPATH/bin:$GOROOT/bin
   export GO111MODULE=auto
   ```
   Then reload:
   ```bash
   source ~/.bashrc
   ```

5. **Install build dependencies**
   ```bash
   sudo apt install -y gcc g++ cmake autoconf libtool pkg-config libmnl-dev libyaml-dev libsctp-dev lksctp-tools iproute2
   sudo snap install cmake --classic
   ```

6. **Install MongoDB**
   ```bash
   curl -fsSL https://www.mongodb.org/static/pgp/server-7.0.asc | \
     sudo gpg -o /usr/share/keyrings/mongodb-server-7.0.gpg --dearmor
   echo "deb [arch=amd64,arm64 signed-by=/usr/share/keyrings/mongodb-server-7.0.gpg] https://repo.mongodb.org/apt/ubuntu focal/mongodb-org/7.0 multiverse" | \
     sudo tee /etc/apt/sources.list.d/mongodb-org-7.0.list
   sudo apt update
   sudo apt install -y mongodb-org
   sudo systemctl start mongod
   sudo systemctl enable mongod
   ```

7. **Install Node.js & Yarn**
   ```bash
   curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
   sudo apt install -y nodejs
   sudo corepack enable
   ```

8. **Build Prometheus C++ client**
   ```bash
   cd ~/5g-testbed-conference
   git clone https://github.com/jupp0r/prometheus-cpp.git
   cd prometheus-cpp
   git submodule init && git submodule update
   sudo apt install -y zlib1g-dev libcurl4-openssl-dev
   mkdir _build && cd _build
   cmake .. -DBUILD_SHARED_LIBS=ON
   make --jobs $(nproc)
   sudo make install
   ```

9. **Install Prometheus server**
   ```bash
   cd ~/5g-testbed-conference
   wget https://github.com/prometheus/prometheus/releases/download/v2.41.0/prometheus-2.41.0.linux-amd64.tar.gz
   tar -xvf prometheus-2.41.0.linux-amd64.tar.gz
   ```
   replace the "prometheus.yml" file here with the one in /5g-testbed-conference/prometheus

10. **Patch Free5GC SMF event**
    ```bash
    nano ~/5g-testbed-conference/go/pkg/mod/github.com/free5gc/openapi@v1.0.8/models/model_smf_event.go
    # Add the below line to the const variables:
    SmfEvent_PDU_SES_EST SmfEvent = "PDU_SES_EST"
    ```

11. **(Optional) Reinstall CMake**
    ```bash
    sudo apt remove --purge -y cmake
    sudo add-apt-repository -y ppa:kitware/ppa
    sudo apt update
    sudo apt install -y cmake
    cmake --version
    ```

12. **Build UERANSIM**
    ```bash
    cd ~/5g-testbed-conference/UERANSIM
    git checkout e4c492d
    make
    ```

13. **Build Free5GC**
    ```bash
    cd ~/5g-testbed-conference/free5gc
    make
    make webconsole
    ```

14. **Install GTP5G**
    ```bash
    cd ~/5g-testbed-conference
    git clone -b v0.8.7 https://github.com/free5gc/gtp5g.git
    cd gtp5g
    make && sudo make install
    ```
---
## subscribing new UEs to the core
To subscribe UEs to the core, run webconsole
```bash
cd ~/free5gc/webconsole
./bin/webconsole
```	
This will run the webconsole on the address http://your-local-address:5000
Open the link and login using “admin” for username and “free5gc” for password. Then go to subscribers and add new subscribers. Just change the supi (so its unique) and change the ‘operator code type’ to OP. Subscribe 5 new UEs with SUPI values 208930000000001 – 208930000000005. For more information, check out section 4 of this link (https://free5gc.org/guide/5-install-ueransim/#4-use-webconsole-to-add-an-ue )
After subscribing the UEs, you can terminate the webcosole using ctrl+c.
Note: the subscribed UEs should have their own configuration files in UERANSIM's config directory

---
## Configuration files (.yaml/yml files)
Core Network: You can find the config files for the Network functions in the config directory of free5gc. They can be edited to fit your needs. The config file for nwdaf is found on the NWDAF directory

gNB and UE: both the GNBs and UEs have configuration files. The setup is loaded with a couple of GNBs and UEs. If you want to add more, you just need to create a new config file. Note that new UEs must be subscribed to the core first (using WebConsole), if they’re not already, before they can connect. When adding new config files, make sure to not duplicate the identifier fields (for e.g the ‘nci’ for GNBs and ‘supi’ for UEs). The IP addresses should also be unique.
To add a new GNB, you can duplicate an existing configuration file, give it a different name, and change the value of the following fields to make them unique.
	 * nci, linkIp, ngapIp, gtpIp, the x,y coordinates
Then add the GNB ip address to the ‘gnbSearchList’ field of all the UEs config files.

To add a new UE, you can do the same as for GNB and change the value of the following fields to make them unique.
* supi, imei(if supi is not used), the x,y,x1,y1 coordinates. The x,y is the home location of the UE and where it is initially deployed when joining while the x1,y1 is the work location of the UE.

---
## Starting the Testbed

### Core

1. Navigate to your Free5GC directory and run:
   ```bash
   cd /path/to/free5gc
   ./run.sh
   ```

2. Navigate to your NWDAF directory and run:
   ```bash
   cd /path/to/nwdaf
   go run nwdaf.go
   ```

3. Navigate to your Prometheus directory and run:
   ```bash
   cd /path/to/prometheus
   sudo prometheus --config.file=prometheus.yml --storage.tsdb.retention.time=10y
   ```
   Prometheus will start on <http://localhost:9090>.

### RAN

1. Navigate to your UERANSIM directory:
   ```bash
   cd /path/to/UERANSIM
   ```
2. For each gNB you want to run, execute:
   ```bash
   build/nr-gnb -c <path-to-gNB-config-file>
   ```

### UEs

1. In the UERANSIM directory, run:
   ```bash
   sudo build/nr-ue -c <path-to-UE-config-file>
   ```
2. Repeat for each UE.

### Optional: Automated UE Join/Leave

If you want UEs to join/leave the network dynamically, you can use the `join_leavev2.py` script:
```bash
cd /path/to/UERANSIM
python3 join_leavev2.py
```
By default, the script will manage 3 UEs; adjust the script to change this number.

For more details on UERANSIM usage, refer to the [UERANSIM Wiki](https://github.com/aligungr/UERANSIM/wiki/Usage).

---
👥 Acknowledgments

    Free5GC

    UERANSIM

    Prometheus

    NWDAF Base
