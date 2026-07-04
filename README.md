# Baleen Engine 🐳

![Status](https://img.shields.io/badge/Status-Ongoing_Development-orange)

Baleen Engine is a high-speed, local-first, peer-to-peer Docker image sharing engine. It completely bypasses cloud registries and internet bottlenecks, allowing you to synchronize Docker images directly between machines on your local network.

> **🚧 Project Status:** This project is currently in **active development (ongoing)**. The core engine is functional, but features, architecture, and APIs are subject to change as development continues. 

## 🚀 Key Features

*   **Peer-to-Peer Transfers:** Direct, high-speed TCP socket transfers over ephemeral TLS — no internet connection or cloud registry required.
*   **Autonomous Discovery:** Built-in `zeroconf` seamlessly handles machine discovery on your local network.
*   **Ledger:** Uses a historical ledger powered by `bbolt` to track image synchronization history and cache layers.
*   **Delta Transfer Engine:** Smart layer pruning and stitching. Only the layers that the peer is missing are transferred.
*   **Smart Architecture Detection:** Automatically handles multi-architecture image resolution across different platforms via `docker buildx`.
*   **Dual-Mode Interface:** Run as a lightweight background daemon with an interactive hybrid CLI, or use the fully integrated Docker Desktop Extension with a visual React-based UI.
*   **Cross-Platform Support:** Written in Go with native Docker SDK integration, supporting deployment on Windows, macOS, and Linux.

## 🛠️ Tech Stack

*   **Core Engine:** Go 1.22+
*   **Container Integration:** Docker Engine SDK
*   **Storage / Ledger:** `bbolt`
*   **Network Discovery:** `zeroconf` (mDNS/DNS-SD)
*   **UI / Extension:** React, TypeScript, Vite, Tailwind CSS

## ⚙️ Architecture Overview

The Baleen "engine" serves as the core logic for this distributed distribution system. It leverages Go's concurrency model for maximum throughput during image extraction and export. 

```mermaid
graph TD
    %% Node Styling (Muted, professional colors)
    classDef ui fill:#dbeafe,stroke:#93c5fd,color:#1e3a8a;
    classDef core fill:#f1f5f9,stroke:#cbd5e1,color:#0f172a;
    classDef db fill:#fef3c7,stroke:#fcd34d,color:#451a03;
    classDef ext fill:#dcfce7,stroke:#86efac,color:#14532d;
    classDef svc fill:#f3e8ff,stroke:#c084fc,color:#3b0764;

    %% Environment Boundaries Styling
    style DockerExt fill:none,stroke:#38bdf8,stroke-width:2px,stroke-dasharray: 4 4
    style HostOS fill:none,stroke:#94a3b8,stroke-width:2px,stroke-dasharray: 4 4
    style Core fill:none,stroke:#cbd5e1,stroke-width:1px
    style RemoteMachine fill:none,stroke:#86efac,stroke-width:2px,stroke-dasharray: 4 4

    subgraph HostOS [Host Machine]
        CLI[CLI REPL]:::ui

        subgraph DockerExt [Docker Extension Environment]
            UI[Extension UI React]:::ui
        end

        subgraph Core [Baleen Core Engine / Daemon]
            Service[Service Manager]:::svc
            API[API Daemon]:::core
            EngineCtx[Engine Context]:::core
            Net[Network & Discovery]:::core
            Trans[Delta Sync Manager]:::core
            Doc[Docker Integrator]:::core
            Ledger[(State Ledger)]:::db
            Config[Configuration Manager]:::core
        end

        Daemon[Local Docker Daemon]:::ext
    end

    subgraph RemoteMachine [Remote Environment]
        Peer[Remote Peer]:::ext
    end

    %% UI & CLI connect to API via HTTP
    UI --->|HTTP / SSE Bridge| API
    CLI --->|HTTP| API

    %% CLI reads daemon state (port, token) from Service Manager
    CLI -->|reads service.json| Service

    %% Startup: Config & Service bootstrap the Engine Context
    Config -->|paths & node name| EngineCtx
    Service -->|state & lock| EngineCtx

    %% EngineContext is injected into the API at startup (not the other way around)
    EngineCtx -->|injected into| API

    %% EngineContext wires all subsystems together
    EngineCtx --> Doc
    EngineCtx --> Ledger
    EngineCtx --> Net
    EngineCtx --> Trans

    %% Transfer reads/writes Ledger for hashing and history
    Trans --> Ledger

    %% Docker Integrator talks to local Docker Daemon
    Doc --->|Unix Socket / Named Pipe| Daemon

    %% Network Connections
    Net ---->|mDNS| Peer
    Trans ---->|TLS Delta Stream| Peer
```

### Core Components (`internal/`)

- **`cli`**: Interactive REPL loop for pushing images, viewing peers, checking history, and pruning. Connects to the running daemon via HTTP when a background service is already active.
- **`api`**: HTTP daemon server providing endpoints for the UI to monitor transfers, peers, images, and live logs (SSE).
- **`config`**: Core node setup, application paths, and generated node names.
- **`network`**: Peer discovery via `zeroconf` mDNS and secure connections via ephemeral RSA-2048 self-signed TLS certificates.
- **`transfer`**: Delta stream engine that negotiates layer diffs, sending only missing layers and verifying integrity via SHA-256.
- **`ledger`**: `bbolt` powered key-value store for history and local layer caching.
- **`docker`**: Integration with Docker SDK to inspect, export, buildx (cross-compile), and load images.
- **`service`**: Daemon lifecycle manager — handles process locking, writing/reading the `service.json` state file (port, token, PID), and spawning the background daemon process.

### Docker Desktop Extension (`ui/`)

A React-based UI that runs inside Docker Desktop, communicating with the Baleen background daemon. It visualizes local peers, tracks real-time transfers, and displays the ledger of sharing history.

