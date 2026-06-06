# ---- Build stage ----
FROM golang:1.25-bookworm AS builder

ARG GOPROXY=https://proxy.golang.org,direct

WORKDIR /build
COPY src/ ./

RUN go env -w GOPROXY=${GOPROXY} && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /build/peerapi-agent .

# ---- Runtime stage ----
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    bird2 \
    wireguard-tools \
    iproute2 \
    iputils-ping \
    procps \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy agent binary
COPY --from=builder /build/peerapi-agent /usr/local/bin/peerapi-agent

# Copy template
COPY templates/ /opt/peerapi-agent/templates/

# Default working directory
WORKDIR /opt/peerapi-agent

# Expose agent HTTP port
EXPOSE 8080

# Entrypoint: start BIRD in background, then run agent
COPY entrypoint.sh /opt/peerapi-agent/entrypoint.sh
RUN chmod +x /opt/peerapi-agent/entrypoint.sh

ENTRYPOINT ["/opt/peerapi-agent/entrypoint.sh"]
