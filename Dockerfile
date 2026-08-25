# Build the React UI
FROM node:20-alpine AS build
WORKDIR /app
COPY ui/package*.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

# Create the minimal Extension image
FROM alpine:3.21
ARG VERSION="1.0.2"
LABEL org.opencontainers.image.title="Baleen"
LABEL org.opencontainers.image.description="High-speed, P2P Docker image sharing over your local network."
LABEL org.opencontainers.image.vendor="Baleen"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.source="https://github.com/BGYKanishka/baleen-engine"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL com.docker.desktop.extension.api.version="0.3.4"
LABEL com.docker.desktop.extension.icon="https://raw.githubusercontent.com/BGYKanishka/baleen-engine/main/baleen.svg"
LABEL com.docker.extension.screenshots='[{"alt":"Peers — Auto-discovered nodes on your local network via mDNS","url":"https://raw.githubusercontent.com/BGYKanishka/baleen-engine/main/screenshots/peers.png"}]'
LABEL com.docker.extension.detailed-description="Baleen is a local-first, P2P Docker image sharing engine. Bypass cloud registries and sync images directly between machines on your local network over encrypted TLS connections. Features: mDNS auto-discovery, delta transfers, real-time controls, and approval workflows."
LABEL com.docker.extension.publisher-url="https://github.com/BGYKanishka/baleen-engine"
LABEL com.docker.extension.changelog="v1.0.2 — Update notifications and bug fixes"
LABEL com.docker.extension.categories='["networking","utility"]'

ARG TARGETARCH

# Copy the manifest, icon, license, and the compiled React app
COPY metadata.json .
COPY baleen.svg .
COPY LICENSE .
COPY --from=build /app/dist /ui

# Copy the compiled Baleen binaries specifically for the target architecture
COPY build/baleen-darwin-${TARGETARCH} /darwin/baleen
COPY build/baleen-linux-${TARGETARCH} /linux/baleen
COPY build/baleen-windows-amd64.exe /windows/baleen.exe
RUN chmod +x /darwin/baleen /linux/baleen || true