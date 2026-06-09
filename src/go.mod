// *******************************************************************
// src/go.mod
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

module github.com/iedon/peerapi-agent

go 1.24

replace github.com/iedon/peerapi-agent/bird => ./bird

require (
	github.com/BurntSushi/toml v1.5.0
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/matishsiao/goInfo v0.0.0-20241216093258-66a9250504d6
	github.com/oschwald/geoip2-golang v1.11.0
	golang.org/x/crypto v0.31.0
	golang.org/x/sys v0.28.0
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
)

require github.com/oschwald/maxminddb-golang v1.13.0 // indirect
