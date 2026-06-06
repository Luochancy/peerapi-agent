#!/bin/bash
set -e

# Create BIRD directories if they don't exist
mkdir -p /var/run/bird /etc/bird/peers

# Start BIRD2 in background
echo "[entrypoint] Starting BIRD2..."
bird -s /var/run/bird/bird.ctl

# Wait for BIRD socket
for i in $(seq 1 10); do
    [ -S /var/run/bird/bird.ctl ] && break
    echo "[entrypoint] Waiting for BIRD socket... ($i/10)"
    sleep 0.5
done

if [ ! -S /var/run/bird/bird.ctl ]; then
    echo "[entrypoint] WARNING: BIRD socket not found, agent may fail to connect"
fi

# Start peerapi-agent
echo "[entrypoint] Starting peerapi-agent..."
exec peerapi-agent -c /opt/peerapi-agent/config.json "$@"
