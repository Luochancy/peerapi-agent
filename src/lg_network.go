// *******************************************************************
// src/lg_network.go
//
// Copyright (C) 2026 Luochancy
//
// Licensed under the GNU General Public License v3.0.
// See LICENSE in the project root.
// *******************************************************************

package main

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// PingResult represents the parsed output of a ping command
type PingResult struct {
	Target    string  `json:"target"`
	PacketsTx int     `json:"packets_tx"`
	PacketsRx int     `json:"packets_rx"`
	LossPct   float64 `json:"loss_pct"`
	MinRTT    float64 `json:"min_rtt_ms,omitempty"`
	AvgRTT    float64 `json:"avg_rtt_ms,omitempty"`
	MaxRTT    float64 `json:"max_rtt_ms,omitempty"`
}

// TraceHop represents a single hop in a traceroute
type TraceHop struct {
	Hop  int     `json:"hop"`
	IP   string  `json:"ip,omitempty"`
	RTT  float64 `json:"rtt_ms,omitempty"`
	Loss bool    `json:"loss"`
}

// TracerouteResult represents the parsed output of a traceroute command
type TracerouteResult struct {
	Target string     `json:"target"`
	Hops   []TraceHop `json:"hops"`
}

var (
	pingStatsRe = regexp.MustCompile(`(\d+) packets transmitted, (\d+) received, (\d+(?:\.\d+)?)% packet loss`)
	pingRTTRe   = regexp.MustCompile(`rtt min/avg/max/mdev = (\d+(?:\.\d+)?)/(\d+(?:\.\d+)?)/(\d+(?:\.\d+)?)/(\d+(?:\.\d+)?)`)
	// traceroute hop: " 1  10.0.0.1  1.234 ms" or " 1  * * *"
	traceHopRe  = regexp.MustCompile(`^\s*(\d+)\s+(.+)$`)
	traceRTTRe  = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*ms`)
	traceStarRe = regexp.MustCompile(`^\s*\d+\s+\*`)
)

// isValidTarget checks that a target string looks like a valid IP or hostname
func isValidTarget(target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	// reject shell metacharacters
	if strings.ContainsAny(target, ";&|`$(){}[]<>!\\\"'") {
		return false
	}
	// reject whitespace inside
	if strings.ContainsAny(target, " \t\n\r") {
		return false
	}
	return true
}

// runPing executes ping and returns parsed result
func runPing(target string) (*PingResult, error) {
	if !isValidTarget(target) {
		return nil, fmt.Errorf("invalid target: %s", target)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ping", "-c", "4", "-W", "2", target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// ping returns non-zero on any loss — still parse output
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, fmt.Errorf("ping failed: %w", err)
		}
	}

	result := &PingResult{Target: target}
	out := string(output)

	if m := pingStatsRe.FindStringSubmatch(out); len(m) > 3 {
		result.PacketsTx, _ = strconv.Atoi(m[1])
		result.PacketsRx, _ = strconv.Atoi(m[2])
		result.LossPct, _ = strconv.ParseFloat(m[3], 64)
	}

	if m := pingRTTRe.FindStringSubmatch(out); len(m) > 4 {
		result.MinRTT, _ = strconv.ParseFloat(m[1], 64)
		result.AvgRTT, _ = strconv.ParseFloat(m[2], 64)
		result.MaxRTT, _ = strconv.ParseFloat(m[3], 64)
	}

	return result, nil
}

// runTraceroute executes traceroute and returns parsed result
func runTraceroute(target string) (*TracerouteResult, error) {
	if !isValidTarget(target) {
		return nil, fmt.Errorf("invalid target: %s", target)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "traceroute", "-n", "-w", "2", "-q", "1", "-m", "30", target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, fmt.Errorf("traceroute failed: %w", err)
		}
	}

	result := &TracerouteResult{Target: target}
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "traceroute to") {
			continue
		}

		m := traceHopRe.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}

		hopNum, _ := strconv.Atoi(m[1])
		rest := strings.TrimSpace(m[2])

		hop := TraceHop{Hop: hopNum}

		if traceStarRe.MatchString(line) || strings.Contains(rest, "*") {
			hop.Loss = true
		} else {
			// Extract IP and RTT: "10.0.0.1  1.234 ms" -> IP=10.0.0.1, RTT=1.234
			parts := strings.Fields(rest)
			if len(parts) >= 1 && !strings.Contains(parts[0], "*") {
				hop.IP = parts[0]
			}
			if len(parts) >= 2 {
				if rttMatch := traceRTTRe.FindStringSubmatch(parts[len(parts)-2] + " " + parts[len(parts)-1]); len(rttMatch) > 1 {
					hop.RTT, _ = strconv.ParseFloat(rttMatch[1], 64)
				}
			}
		}

		result.Hops = append(result.Hops, hop)
	}

	return result, nil
}
