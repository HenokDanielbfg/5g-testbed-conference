5G Testbed Conference

This repository provides a comprehensive 5G testbed environment integrating Free5GC, UERANSIM, and NWDAF components. It is designed to simulate and analyze 5G network behaviors, including mobility management, handovers, and network data analytics. The setup is suitable for research, development, and demonstration purposes in 5G networking.​
Features

    Free5GC Core Network: Implements the 5G core network functionalities.

    UERANSIM: Simulates User Equipment (UE) and Radio Access Network (RAN) components.

    NWDAF (Network Data Analytics Function): Provides analytics capabilities for network data.

    Prometheus Integration: Monitors and visualizes network metrics.

    Live Visualization Tools:

        handover_live_visual.py: Visualizes handover events in real-time.

        mobility_live_visual.py: Displays UE mobility patterns dynamically.

    Automation Scripts:

        setup.sh: Automates the setup of the entire testbed environment.

        cleanup.sh: Cleans up and resets the testbed environment.​

Repository Structure

5g-testbed-conference/
├── free5gc/                  # Free5GC core network components
├── UERANSIM/                 # UE and RAN simulation tools
├── mnc_NWDAF-main/           # NWDAF implementation
├── prometheus/               # Monitoring and visualization setup
├── core dataset/             # Core network datasets
├── handover_live_visual.py   # Real-time handover visualization script
├── mobility_live_visual.py   # Real-time mobility visualization script
├── setup.sh                  # Environment setup script
├── cleanup.sh                # Environment cleanup script
└── README.md                 # Project documentation

Prerequisites

    Operating System: Ubuntu 20.04 or later

    Docker & Docker Compose: For containerized deployment

    Python 3.8+: Required for running visualization scripts

    Prometheus & Grafana: For monitoring and visualization​

Installation

    Clone the Repository:

    git clone https://github.com/HenokDanielbfg/5g-testbed-conference.git
    cd 5g-testbed-conference

    Run the Setup Script:

    chmod +x setup.sh
    ./setup.sh

This script will set up the Free5GC core, UERANSIM, NWDAF, and Prometheus components.
Usage

    Start the Testbed Environment:

  ./setup.sh

    Run Visualization Scripts:

        Handover Visualization:

python3 handover_live_visual.py

Mobility Visualization:

        python3 mobility_live_visual.py

    Access Prometheus Dashboard:

    Navigate to http://localhost:9090 in your web browser to access the Prometheus dashboard.​

Cleanup

To stop and remove all running containers and clean up the environment:​

chmod +x cleanup.sh
./cleanup.sh

License

This project is licensed under the MIT License. See the LICENSE file for details.​
Acknowledgments

    Free5GC

    UERANSIM

    Prometheus

    Grafana​
