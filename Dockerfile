# Build the React UI
FROM node:20-alpine AS build
WORKDIR /app
COPY ui/package*.json ./
RUN npm install
COPY ui/ ./
RUN npm run build

# Create the minimal Extension image
FROM alpine:latest
LABEL org.opencontainers.image.title="Baleen"
LABEL org.opencontainers.image.description="P2P Docker Image Sharing"
LABEL org.opencontainers.image.vendor="Baleen"
LABEL com.docker.desktop.extension.api.version="0.3.4"
LABEL com.docker.desktop.extension.icon="baleen.svg"

# Copy the manifest, icon, and the compiled React app
COPY metadata.json .
COPY baleen.svg .
COPY --from=build /app/dist /ui

COPY metadata.json .
COPY baleen.svg .
COPY --from=build /app/dist /ui

# Copy the compiled Baleen binaries for different platforms
COPY build/baleen-darwin-amd64 /bin/darwin/amd64/baleen
COPY build/baleen-darwin-arm64 /bin/darwin/arm64/baleen
COPY build/baleen-linux-amd64 /bin/linux/amd64/baleen
COPY build/baleen-linux-arm64 /bin/linux/arm64/baleen
COPY build/baleen-windows-amd64.exe /bin/windows/amd64/baleen.exe