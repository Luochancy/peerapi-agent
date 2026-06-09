// *******************************************************************
// src/task_rtt.go
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
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Map to cache recently failed ping attempts to reduce timeout for known bad IPs
var failedPingCache = make(map[string]time.Time)

// measureRTT tries to use the most recently successful IP protocol first, before falling back to others
func measureRTT(sessionUUID, ipv4, ipv6, ipv6ll string) int {
	// Check if we have a preferred protocol for this session
	rttMutex.RLock()
	tracker, exists := rttTrackers[sessionUUID]
	rttMutex.RUnlock()

	// Order of attempts based on previous success
	var attemptOrder []string

	if exists && tracker.PreferredProtocol != "" {
		// If we have a recently successful protocol, try it first
		// Only fall back to other protocols if it fails
		attemptOrder = []string{tracker.PreferredProtocol}
		// Only add fallback protocols if the preferred one has been failing recently
		if tracker.LastRTT == -1 {
			// Add other protocols as fallbacks
			for _, proto := range []string{"ipv6ll", "ipv6", "ipv4"} {
				if proto != tracker.PreferredProtocol {
					attemptOrder = append(attemptOrder, proto)
				}
			}
		}
	} else {
		// Default order: IPv6 link-local first (usually fastest), then IPv6, then IPv4
		attemptOrder = []string{"ipv6ll", "ipv6", "ipv4"}
	}

	// Try protocols in determined order
	for _, proto := range attemptOrder {
		switch proto {
		case "ipv6ll":
			if ipv6ll != "" {
				rtt, loss := pingRTT(ipv6ll)
				if rtt != -1 {
					updateRTTTracker(sessionUUID, "ipv6ll", rtt, loss)
					return rtt
				}
			}
		case "ipv6":
			if ipv6 != "" {
				rtt, loss := pingRTT(ipv6)
				if rtt != -1 {
					updateRTTTracker(sessionUUID, "ipv6", rtt, loss)
					return rtt
				}
			}
		case "ipv4":
			if ipv4 != "" {
				rtt, loss := pingRTT(ipv4)
				if rtt != -1 {
					updateRTTTracker(sessionUUID, "ipv4", rtt, loss)
					return rtt
				}
			}
		}
	}

	// If all attempts fail, update tracker with failure and return -1
	updateRTTTracker(sessionUUID, "", -1, 1.0)
	return -1
}

// updateRTTTracker updates the RTT tracking information for a session
func updateRTTTracker(sessionUUID, preferredProtocol string, rtt int, loss float64) {
	rttMutex.Lock()
	defer rttMutex.Unlock()

	tracker, exists := rttTrackers[sessionUUID]
	if !exists {
		tracker = &RTTTracker{
			LastRTT:    -1,
			LastLoss:   1.0,
			Metric:     make([]int, 0),
			LossMetric: make([]float64, 0),
			AvgLoss:    1.0,
		}
		rttTrackers[sessionUUID] = tracker
	}

	tracker.LastRTT = rtt
	tracker.LastLoss = loss

	if preferredProtocol != "" {
		// which means we have a successful ping
		tracker.PreferredProtocol = preferredProtocol
	}

	// Record the current LastRTT to the metric array
	tracker.Metric = append(tracker.Metric, tracker.LastRTT)
	// Maintain maxMetricsHistory limit - drop oldest entries if exceeded
	if len(tracker.Metric) > cfg.Metric.MaxRTTMetricsHistroy {
		// Remove oldest entries to maintain the limit
		tracker.Metric = tracker.Metric[1:]
	}

	// Record the current loss to the loss metric array
	tracker.LossMetric = append(tracker.LossMetric, tracker.LastLoss)
	// Maintain maxMetricsHistory limit - drop oldest entries if exceeded
	if len(tracker.LossMetric) > cfg.Metric.MaxRTTMetricsHistroy {
		// Remove oldest entries to maintain the limit
		tracker.LossMetric = tracker.LossMetric[1:]
	}

	// Calculate average loss rate based on RTT measurements
	tracker.AvgLoss = calculateAvgLossRate(tracker.LossMetric)
}

// calculateAvgLossRate calculates the average packet loss rate from RTT measurements
// Returns a value between 0.0 (no loss) and 1.0 (100% loss)
func calculateAvgLossRate(metrics []float64) float64 {
	if len(metrics) == 0 {
		return 0.0
	}

	var totalLoss float64
	for _, loss := range metrics {
		totalLoss += loss
	}

	return totalLoss / float64(len(metrics))
}

// pingRTT performs actual implementation of ping RTT measurement using ICMP ping
func pingRTT(ip string) (int, float64) {
	// Track recently failed destinations to use shorter timeouts
	rttMutex.RLock()
	lastKnownFailedIP, exists := failedPingCache[ip]
	rttMutex.RUnlock()

	// If this IP has failed recently, force using a shorter timeout for the next attempt
	timeout := cfg.Metric.PingTimeout
	pingCount := cfg.Metric.PingCount

	if exists && time.Since(lastKnownFailedIP) < 10*time.Minute {
		pingCount = cfg.Metric.PingCountOnFail // Just do PingCountOnFail attempts for recently failed destinations
	}

	// Use ICMP ping instead of TCP ping
	rtt, loss := icmpPingAverage(ip, pingCount, timeout)

	// Update the failed IP cache
	rttMutex.Lock()
	if rtt <= 0 {
		// Remember this as a failed IP
		failedPingCache[ip] = time.Now()
	} else {
		// Remove from failed cache if it succeeds
		delete(failedPingCache, ip)
	}
	rttMutex.Unlock()

	// log.Printf("[RTT] Ping RTT for %s: %d ms (timeout: %d s, count: %d)\n", ip, rtt, timeout, pingCount)
	return rtt, loss
}

// batchMeasureRTT processes multiple RTT measurements in parallel with context cancellation support
// This function is meant to be called as a background task before the regular metric collection cycle
func batchMeasureRTT(ctx context.Context) {
	sessionMutex.RLock()
	sessions := make([]BgpSession, 0, len(localSessions))
	for _, session := range localSessions {
		if session.Status == PEERING_STATUS_ENABLED || session.Status == PEERING_STATUS_PROBLEM {
			sessions = append(sessions, session)
		}
	}
	sessionMutex.RUnlock()

	if len(sessions) == 0 {
		return
	}

	log.Printf("[RTT] Starting batch RTT measurement for %d sessions...", len(sessions))
	startTime := time.Now()

	processBatchRTT(ctx, sessions)

	duration := time.Since(startTime)
	log.Printf("[RTT] Completed batch RTT measurement for %d sessions using up to %d workers in %v",
		len(sessions), cfg.Metric.PingWorkerCount, duration)
}

// processBatchRTT processes a single batch of RTT measurements
func processBatchRTT(ctx context.Context, sessions []BgpSession) {
	if len(sessions) == 0 {
		return
	}

	// Create a worker pool with a reasonable number of workers
	workerCount := min(len(sessions), cfg.Metric.PingWorkerCount)
	if workerCount == 0 {
		workerCount = min(len(sessions), 8) // Default fallback
	}

	// Create channels for work distribution
	jobs := make(chan BgpSession, len(sessions))
	results := make(chan struct{}, len(sessions))
	for range workerCount {
		go rttWorker(ctx, jobs, results)
	}

	// Send sessions to be processed
	for _, session := range sessions {
		select {
		case jobs <- session:
		case <-ctx.Done():
			close(jobs)
			return
		}
	}
	close(jobs)
	// Wait for all jobs to complete or context cancellation
	completedJobs := 0
	for completedJobs < len(sessions) {
		select {
		case <-results:
			completedJobs++
		case <-ctx.Done():
			return
		}
	}
}

// rttWorker is a worker goroutine that processes RTT measurements with context cancellation support
func rttWorker(ctx context.Context, jobs <-chan BgpSession, results chan<- struct{}) {
	for {
		select {
		case session, ok := <-jobs:
			if !ok {
				return
			}

			// Check if context is cancelled before processing
			select {
			case <-ctx.Done():
				return
			default:
			}

			ipv6LinkLocal := session.IPv6LinkLocal
			if ipv6LinkLocal != "" {
				ipv6LinkLocal = fmt.Sprintf("%s%%%s", session.IPv6LinkLocal, session.Interface)
			}

			// Perform RTT measurement
			measureRTT(
				session.UUID,
				session.IPv4,
				session.IPv6,
				ipv6LinkLocal,
			)

			// Signal that this job is done
			select {
			case results <- struct{}{}:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// cleanupRTTCache periodically cleans up the RTT trackers and failed ping cache
// to prevent memory leaks from accumulating over time
func cleanupRTTCache(ctx context.Context) {
	// Run this cleanup task every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			shutdownStart := time.Now()
			log.Println("[RTT] Shutting down RTT cache cleanup task...")

			// Perform one final cleanup during shutdown with timeout
			cleanupDone := make(chan struct{})
			go func() {
				performRTTCacheCleanup()
				close(cleanupDone)
			}()

			// Wait with timeout
			select {
			case <-cleanupDone:
				log.Printf("[RTT] Final RTT cache cleanup completed in %v", time.Since(shutdownStart))
			case <-time.After(2 * time.Second):
				log.Printf("[RTT] Final RTT cache cleanup timed out after %v", time.Since(shutdownStart))
			}

			return
		case <-ticker.C:
			performRTTCacheCleanup()
		}
	}
}

// performRTTCacheCleanup does the actual work of cleaning up RTT caches
func performRTTCacheCleanup() {
	cleanupTime := time.Now()

	// Get the current list of active session UUIDs
	sessionMutex.RLock()
	activeUUIDs := make(map[string]bool)
	for _, s := range localSessions {
		activeUUIDs[s.UUID] = true
	}
	sessionMutex.RUnlock()

	// Cleanup RTT trackers
	rttMutex.Lock()
	defer rttMutex.Unlock()

	// Track cleanup stats
	trackersRemoved := 0
	cacheEntriesRemoved := 0

	for uuid := range rttTrackers {
		// Remove trackers for sessions that no longer exist
		if !activeUUIDs[uuid] {
			delete(rttTrackers, uuid)
			trackersRemoved++
		}
	}

	// Cleanup failed ping cache (remove entries older than 12 hours)
	for ip, lastFailTime := range failedPingCache {
		if time.Since(lastFailTime) > 12*time.Hour {
			delete(failedPingCache, ip)
			cacheEntriesRemoved++
		}
	}

	log.Printf("[RTT] Cleaned up RTT caches at %s (removed %d trackers, %d failed cache entries)",
		cleanupTime.Format(time.RFC3339), trackersRemoved, cacheEntriesRemoved)
}

// batchRTTTask runs periodic RTT measurements as a background task
func batchRTTTask(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	// Start the RTT cache cleanup routine with context
	cleanupCtx, cleanupCancel := context.WithCancel(ctx)
	var cleanupWg sync.WaitGroup
	cleanupWg.Add(1)
	go func() {
		defer cleanupWg.Done()
		cleanupRTTCache(cleanupCtx)
	}()

	// Make sure to wait for cleanup goroutine to finish
	defer func() {
		shutdownStart := time.Now()
		log.Println("[RTT] Canceling RTT cache cleanup routine...")

		// Cancel cleanup task
		cleanupCancel()

		// Set a timeout for cleanup
		cleanupDone := make(chan struct{})
		go func() {
			cleanupWg.Wait()
			close(cleanupDone)
		}()

		// Wait for cleanup with timeout
		select {
		case <-cleanupDone:
			log.Printf("[RTT] RTT cache cleanup completed in %v", time.Since(shutdownStart))
		case <-time.After(3 * time.Second):
			log.Printf("[RTT] RTT cache cleanup timed out after %v", time.Since(shutdownStart))
		}
	}()

	// Create a ticker for RTT measurement interval
	// RTT measurements should be more frequent than metric collection to provide fresh data
	rttInterval := max(time.Duration(cfg.PeerAPI.MetricInterval)*time.Second, 60*time.Second)

	ticker := time.NewTicker(rttInterval)
	defer ticker.Stop()

	// Perform initial RTT measurement
	log.Printf("[RTT] Starting RTT measurement task with interval %v", rttInterval)
	batchMeasureRTT(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("[RTT] RTT measurement task shutting down...")
			return
		case <-ticker.C:
			batchMeasureRTT(ctx)
		}
	}
}

// getRTTValue retrieves the RTT value for a session from the RTT tracker
func getRTTValue(sessionUUID string) (int, float64) {
	rttMutex.RLock()
	defer rttMutex.RUnlock()

	tracker, exists := rttTrackers[sessionUUID]
	if exists {
		return tracker.LastRTT, tracker.AvgLoss
	}

	return -1, 1.0 // Default to -1 RTT and 100% loss if no tracker exists
}
