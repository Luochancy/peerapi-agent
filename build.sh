// *******************************************************************
// build.sh
//
// Copyright (C) 2024 iEdon
// Copyright (C) 2026 Luochancy
//
// This file is part of a project derived from peerapi-agent.
// Modified by Luochancy on 2026-06.
//
// Licensed under the GNU General Public License v3.0.
// See the LICENSE file in the project root for details.
// *******************************************************************

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
