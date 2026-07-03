// *******************************************************************
// src/version.go
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

package main

import (
	"fmt"
	"runtime"
)

const (
	SERVER_NAME    = "PeerHub-Agent"
	SERVER_VERSION = "1.0.0"
)

var GIT_COMMIT string // Set at build time via -ldflags "-X main.GIT_COMMIT=$(git rev-parse --short HEAD)"
var SERVER_SIGNATURE = fmt.Sprintf("%s (%s; %s; %s; %s)", SERVER_NAME+"/"+SERVER_VERSION, func() string {
	if GIT_COMMIT != "" {
		return GIT_COMMIT
	}
	return "unknown"
}(), runtime.GOOS, runtime.GOARCH, runtime.Version())
