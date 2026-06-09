// *******************************************************************
// src/types.go
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
	"encoding/json"
)

// Response formats
type AgentApiResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type PeerApiResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type NodePassthroughRequest struct {
	ASN                  uint `json:"asn"`
	ExistingSessionCount int  `json:"existingSessionCount"`
	Data                 struct {
		LinkType      string   `json:"linkType"`
		BGPExtensions []string `json:"bgpExtensions"`
	} `json:"data"`
}

// Agent API Session types
type BgpSession struct {
	UUID          string          `json:"uuid"`
	ASN           uint            `json:"asn"`
	Status        int             `json:"status"`
	IPv4          string          `json:"ipv4"`
	IPv6          string          `json:"ipv6"`
	IPv6LinkLocal string          `json:"ipv6LinkLocal"`
	Type          string          `json:"type"`
	Extensions    []string        `json:"extensions"`
	Interface     string          `json:"interface"`
	Endpoint      string          `json:"endpoint"`
	Credential    string          `json:"credential"`
	Data          json.RawMessage `json:"data"`
	MTU           int             `json:"mtu"`
	Policy        int             `json:"policy"`
}

type BgpSessionsResponse struct {
	BgpSessions []BgpSession `json:"bgpSessions"`
}

// Session Modify request type
type SessionModifyRequest struct {
	Status  int    `json:"status"`
	Session string `json:"session"`
}

type SessionReportRequest struct {
	Sessions []SessionMetric `json:"sessions"`
}

type SessionMetric struct {
	UUID      string          `json:"uuid"`
	ASN       uint            `json:"asn"`
	Timestamp int64           `json:"timestamp"`
	BGP       []BGPMetric     `json:"bgp"`
	Interface InterfaceMetric `json:"interface"`
	RTT       RTT             `json:"rtt"`
}

const (
	BGP_SESSION_TYPE_IPV4  = "ipv4"
	BGP_SESSION_TYPE_IPV6  = "ipv6"
	BGP_SESSION_TYPE_MPBGP = "mpbgp"
)

type BGPMetric struct {
	Name   string          `json:"name"`
	State  string          `json:"state"`
	Info   string          `json:"info"`
	Type   string          `json:"type"` // BGP_SESSION_TYPE_IPV4, BGP_SESSION_TYPE_IPV6, or BGP_SESSION_TYPE_MPBGP
	Since  string          `json:"since"`
	Routes BGPRoutesMetric `json:"routes"`
}

type BGPRoutesMetric struct {
	IPv4 RouteMetricStruct `json:"ipv4"`
	IPv6 RouteMetricStruct `json:"ipv6"`
}

type RouteMetricStruct struct {
	Imported RouteMetrics `json:"imported"`
	Exported RouteMetrics `json:"exported"`
}

type RouteMetrics struct {
	Current int `json:"current"`
}

type InterfaceMetric struct {
	IPv4          string                 `json:"ipv4"`
	IPv6          string                 `json:"ipv6"`
	IPv6LinkLocal string                 `json:"ipv6LinkLocal"`
	MAC           string                 `json:"mac"`
	MTU           int                    `json:"mtu"`
	Status        string                 `json:"status"`
	Traffic       InterfaceTrafficMetric `json:"traffic"`
}

type InterfaceTrafficMetric struct {
	Total   []int64 `json:"total"`   // [Tx, Rx]
	Current []int64 `json:"current"` // [Tx, Rx]
}

type RTT struct {
	Current int     `json:"current"`
	Loss    float64 `json:"loss"` // Average packet loss rate (0.0 = no loss, 1.0 = 100% loss)
}

// RTTTracker holds information about the best protocol to use for RTT measurements
type RTTTracker struct {
	PreferredProtocol string    // "ipv4", "ipv6", or "ipv6ll"
	LastRTT           int       // Last measured RTT value
	LastLoss          float64   // Last measured packet loss rate
	Metric            []int     // RTT records(each time LastRTT is archived here)
	LossMetric        []float64 // Packet loss records (each time LastRTT is archived here)
	AvgLoss           float64   // Average loss rate of RTT measurements
}
