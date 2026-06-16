# Baleen Engine 🐳

![Status](https://img.shields.io/badge/Status-Ongoing_Development-orange)

Baleen Engine is a high-speed, local-first, peer-to-peer Docker image sharing engine. It completely bypasses cloud registries and internet bottlenecks, allowing you to synchronize Docker images directly between machines on your local network.

> **🚧 Project Status:** This project is currently in **active development (ongoing)**. The core engine is functional, but features, architecture, and APIs are subject to change as development continues. 

## 🚀 Key Features

*   **Peer-to-Peer Transfers:** Direct, high-speed TCP socket transfers over ephemeral TLS — no internet connection or cloud registry required.
*   **Autonomous Discovery:** Built-in `zeroconf` seamlessly handles machine discovery on your local network.
*   **Immutable Ledger:** Uses a Git-like historical ledger powered by `bbolt` to track image synchronization history and cache layers.
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
    classDef ui fill:#e2e8f0,stroke:#94a3b8,color:#0f172a;
    classDef core fill:#f1f5f9,stroke:#cbd5e1,color:#0f172a;
    classDef db fill:#fef3c7,stroke:#fcd34d,color:#451a03;
    classDef ext fill:#dcfce7,stroke:#86efac,color:#14532d;

    %% Subgraph Styling (Transparent backgrounds, subtle borders)
    style Interfaces fill:none,stroke:#94a3b8,stroke-width:2px,stroke-dasharray: 5 5
    style Core fill:none,stroke:#cbd5e1,stroke-width:2px,stroke-dasharray: 5 5
    style External fill:none,stroke:#86efac,stroke-width:2px,stroke-dasharray: 5 5

    subgraph Interfaces [User Interfaces]
        direction TB
        UI[Docker Desktop Extension]:::ui
        CLI[CLI REPL]:::ui
    end

    subgraph Core [Baleen Core Engine]
        direction TB
        API[internal/api]:::core
        CLI_PKG[internal/cli]:::core
        Net[internal/network]:::core
        Trans[internal/transfer]:::core
        Doc[internal/docker]:::core
        Ledger[(internal/ledger)]:::db
        Config[internal/config]:::core
    end

    subgraph External [External Systems]
        direction TB
        Daemon[Local Docker Daemon]:::ext
        Peer[Remote Peer]:::ext
    end

    UI -->|HTTP / SSE| API
    CLI --> CLI_PKG

    %% API and CLI share the EngineContext
    API -.->|shares context| CLI_PKG
    
    %% CLI orchestrates the core subsystems
    CLI_PKG --> Net
    CLI_PKG --> Trans
    CLI_PKG --> Doc
    CLI_PKG --> Ledger
    
    %% Transfer relies on Config and Ledger
    Trans --> Config
    Trans --> Ledger

    %% Longer arrows (--->) enforce a higher rank separation to push External Systems to the bottom
    Net --->|mDNS| Peer
    Trans --->|TLS Delta Stream| Peer
    Doc --->|Unix Socket| Daemon
```

### Core Components (`internal/`)

- **`cli`**: Interactive REPL loop for pushing images, viewing peers, checking history, and pruning.
- **`api`**: HTTP daemon server providing endpoints for the UI to monitor transfers, peers, images, and live logs (SSE).
- **`network`**: Peer discovery via `zeroconf` mDNS and secure connections via ephemeral RSA-2048 self-signed TLS certificates.
- **`transfer`**: Delta stream engine that negotiates layer diffs, sending only missing layers and verifying integrity via SHA-256.
- **`ledger`**: `bbolt` powered key-value store for commit history and local layer caching.
- **`docker`**: Integration with Docker SDK to inspect, export, buildx (cross-compile), and load images.

### Docker Desktop Extension (`ui/`)

A React-based UI that runs inside Docker Desktop, communicating with the Baleen background daemon. It visualizes local peers, tracks real-time transfers, and displays the immutable ledger of sharing history.

