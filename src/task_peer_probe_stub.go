// *******************************************************************
// src/task_peer_probe_stub.go
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

//go:build !linux

package main

import (
	"context"
	"log"
	"sync"
)

func peerProbeTask(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Println("[PeerProbe] Task disabled: peer probes require linux-specific networking features")
}
