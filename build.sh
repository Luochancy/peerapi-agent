#!/bin/sh
set -e
echo "Building peerapi-agent for Linux AMD64..."

export GOOS=linux
export GOARCH=amd64

rm -rf dist || true
mkdir dist

cd src
go mod tidy
go build -o ../dist/peerapi-agent -ldflags="-X main.GIT_COMMIT=$(git rev-parse --short HEAD)"

cd ..
mkdir -p ./dist/config
cp config/config.toml ./dist/config/config.toml
cp config/server.toml.default config/bird.toml.default config/sysctl.toml.default ./dist/config/
[ -f config.json.example ] && cp config.json.example ./dist/config/

echo "Build completed."
