package main

import (
	"context"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// WireGuardHandshakeInfo stores handshake information for a WireGuard peer
type WireGuardHandshakeInfo struct {
	PublicKey       string
	LastHandshake   int64
	InterfaceName   string
	SessionEndpoint string
}

// wireGuardDNSTask runs periodically to update WireGuard endpoints based on DNS resolution
func wireGuardDNSTask(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	// Use configured interval, default to 300 seconds (5 minutes) if not set
	intervalSeconds := 300
	if cfg.WireGuard.DNSUpdateInterval > 0 {
		intervalSeconds = cfg.WireGuard.DNSUpdateInterval
	}

	log.Printf("[WireGuardDNS] Running with DNS update interval of %d seconds", intervalSeconds)
	interval := time.Duration(intervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			shutdownStart := time.Now()
			log.Println("[WireGuardDNS] Shutting down WireGuard DNS update task...")

			// Perform any WireGuard-specific cleanup
			log.Println("[WireGuardDNS] WireGuard DNS update task cleanup completed")

			log.Printf("[WireGuardDNS] WireGuard DNS update task shutdown completed in %v", time.Since(shutdownStart))
			return
		case <-ticker.C:
			performWireGuardDNSUpdate()
		}
	}
}

// performWireGuardDNSUpdate checks all active WireGuard sessions and updates endpoints if needed
func performWireGuardDNSUpdate() {
	// Get all active WireGuard sessions
	sessionMutex.RLock()
	wireguardSessions := make([]BgpSession, 0)
	for _, session := range localSessions {
		// Only check active WireGuard sessions
		if (session.Status == PEERING_STATUS_ENABLED || session.Status == PEERING_STATUS_PROBLEM) &&
			session.Type == "wireguard" && session.Interface != "" && session.Credential != "" {
			wireguardSessions = append(wireguardSessions, session)
		}
	}
	sessionMutex.RUnlock()

	if len(wireguardSessions) == 0 {
		// log.Println("[WireGuardDNS] No active WireGuard sessions to check")
		return
	}

	// log.Printf("[WireGuardDNS] Checking %d WireGuard sessions for DNS updates", len(wireguardSessions))

	// Check each WireGuard session
	for _, session := range wireguardSessions {
		checkAndUpdateWireGuardEndpoint(&session)
	}
}

// checkAndUpdateWireGuardEndpoint checks if a WireGuard session needs endpoint update
func checkAndUpdateWireGuardEndpoint(session *BgpSession) {
	if session.Endpoint == "" {
		// log.Printf("[WireGuardDNS] <%s> No endpoint configured, skipping", session.UUID)
		return
	}

	// Get latest handshake information for this interface
	handshakeTime, err := getWireGuardLastHandshake(session.Interface, session.Credential)
	if err != nil {
		log.Printf("[WireGuardDNS] <%s> Failed to get handshake info for interface %s: %v",
			session.UUID, session.Interface, err)
		return
	}

	// Check if handshake is older than 135 seconds (similar to the bash script)
	currentTime := time.Now().Unix()
	if (currentTime - handshakeTime) > 135 {
		// log.Printf("[WireGuardDNS] <%s> Last handshake was %d seconds ago, updating endpoint for interface %s",
		//	session.UUID, currentTime-handshakeTime, session.Interface)

		err := updateWireGuardEndpoint(session.Interface, session.Credential, session.Endpoint)
		if err != nil {
			log.Printf("[WireGuardDNS] <%s> Failed to update endpoint for interface %s: %v",
				session.UUID, session.Interface, err)
		} else {
			// log.Printf("[WireGuardDNS] <%s> Successfully updated endpoint for interface %s to %s",
			//	session.UUID, session.Interface, session.Endpoint)
		}
	}
	// else {
	//	log.Printf("[WireGuardDNS] <%s> Recent handshake (%d seconds ago), no update needed for interface %s",
	//		session.UUID, currentTime-handshakeTime, session.Interface)
	// }
}

// getWireGuardLastHandshake gets the last handshake time for a specific peer on an interface
func getWireGuardLastHandshake(interfaceName, publicKey string) (int64, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Run 'wg show <interface> latest-handshakes'
	cmd := exec.CommandContext(ctx, cfg.WireGuard.WGCommandPath, "show", interfaceName, "latest-handshakes")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, err
	}

	// Parse the output to find the handshake time for our public key
	// Output format: <public_key>\t<timestamp>
	lines := strings.SplitSeq(string(output), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			continue
		}

		if strings.TrimSpace(parts[0]) == publicKey {
			timestamp, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
			if err != nil {
				return 0, err
			}
			return timestamp, nil
		}
	}

	// If we don't find the public key, return 0 (no handshake)
	return 0, nil
}

// updateWireGuardEndpoint updates the endpoint for a WireGuard peer
func updateWireGuardEndpoint(interfaceName, publicKey, endpoint string) error {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Run 'wg set <interface> peer <public_key> endpoint <endpoint>'
	cmd := exec.CommandContext(ctx, cfg.WireGuard.WGCommandPath, "set", interfaceName,
		"peer", publicKey, "endpoint", endpoint)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[WireGuardDNS] Error while running wg set, output: %s", strings.TrimSpace(string(output)))
		return err
	}

	return nil
}
