# Baleen Engine 🐳

![Status](https://img.shields.io/badge/Status-Ongoing_Development-orange)

Baleen Engine is a high-speed, local-first, peer-to-peer Docker image sharing engine. It completely bypasses cloud registries and internet bottlenecks, allowing you to synchronize Docker images directly between machines on your local network.

> **🚧 Project Status:** This project is currently in **active development (ongoing)**. The core engine is functional, but features, architecture, and APIs are subject to change as development continues. 

## 🚀 Key Features

*   **Peer-to-Peer Transfers:** Direct, high-speed TCP socket transfers—no internet connection or cloud registry required.
*   **Autonomous Discovery:** Built-in `zeroconf` seamlessly handles machine discovery on your local network.
*   **Immutable Ledger:** Uses a Git-like historical ledger powered by `bbolt` to track image synchronization history.
*   **Smart Architecture Detection:** Automatically handles multi-architecture image resolution across different platforms.
*   **Dual-Mode Interface:** Run as a lightweight background daemon with a hybrid CLI, or use the fully integrated Docker Desktop Extension with a visual React-based UI.
*   **Cross-Platform Support:** Written in Go with native Docker SDK integration, supporting deployment on Windows, macOS, and Linux.

## 🛠️ Tech Stack

*   **Core Engine:** Go (Golang)
*   **Container Integration:** Docker Engine SDK
*   **Storage / Ledger:** `bbolt`
*   **Network Discovery:** `zeroconf`
*   **UI / Extension:** React, Node.js

## ⚙️ Architecture Overview

The Baleen "engine" serves as the core, heavy-lifting logic for this distributed distribution system. It leverages Go's concurrency model for maximum throughput during image extraction and export. The engine handles complex low-level TCP socket programming and autonomous architecture detection independently, while pairing perfectly with its Node-based UI wrapper for visual management.
