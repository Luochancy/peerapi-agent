// *******************************************************************
// src/bird/bird.go
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

package bird

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PooledConnection represents a connection in the pool
type PooledConnection struct {
	conn     *BirdConn
	lastUsed time.Time
	inUse    bool
	// id       int // Add connection ID for faster lookup
}

// BirdPool manages a pool of BIRD connections
type BirdPool struct {
	sync.RWMutex
	pool                   []*PooledConnection
	available              chan *PooledConnection // Channel for available connections
	poolSize               int
	poolSizeMax            int
	connectionMaxRetries   int
	connectionRetryDelayMs int
	socketPath             string
	// nextID      int           // For assigning connection IDs
	shutdown chan struct{} // Graceful shutdown signal

	dialMu          sync.Mutex
	lastDialFailure time.Time
	dialBackoff     time.Duration
	dialBackoffMin  time.Duration
	dialBackoffMax  time.Duration
}

// ProtocolMetrics represents the metrics for a single BGP protocol/session
type ProtocolMetrics struct {
	State      string
	Since      string
	Info       string
	IPv4Import int64
	IPv4Export int64
	IPv6Import int64
	IPv6Export int64
}

// ProtocolResult represents the result of a single protocol query
type ProtocolResult struct {
	SessionName string
	Metrics     ProtocolMetrics
	Error       error
}

// BatchQuery represents a BIRD query request
type BatchQuery struct {
	SessionName string
	Command     string
}

// BatchResult represents the result of a batch query
type BatchResult struct {
	SessionName string
	Output      string
	Error       error
}

// NewBirdPool creates a new BIRD connection pool
func NewBirdPool(socketPath string, poolSize, poolSizeMax, connectionMaxRetries, connectionRetryDelayMs int) (*BirdPool, error) {
	if poolSizeMax < poolSize {
		poolSizeMax = poolSize * 4 // Default max is 4x the base pool size
	}

	bp := &BirdPool{
		poolSize:               poolSize,
		poolSizeMax:            poolSizeMax,
		connectionMaxRetries:   connectionMaxRetries,
		connectionRetryDelayMs: connectionRetryDelayMs,
		socketPath:             socketPath,
		available:              make(chan *PooledConnection, poolSizeMax), // Buffered channel
		shutdown:               make(chan struct{}),
		dialBackoffMin:         time.Second,
		dialBackoffMax:         30 * time.Second,
	}

	// Initialize connection pool
	for i := range bp.poolSize {
		bc, err := bp.createConnection()
		if err != nil {
			bp.Close()
			return nil, fmt.Errorf("failed to initialize connection %d: %v", i, err)
		}
		pc := &PooledConnection{
			conn:     bc,
			lastUsed: time.Now(),
			// id:       bp.nextID,
		}
		// bp.nextID++
		bp.pool = append(bp.pool, pc)
		// Pre-populate available channel
		bp.available <- pc
	}

	// Start pool maintenance goroutine
	go bp.poolMaintenance()

	return bp, nil
}

func (bp *BirdPool) waitForDialWindow() {
	bp.dialMu.Lock()
	backoff := bp.dialBackoff
	lastFailure := bp.lastDialFailure
	bp.dialMu.Unlock()

	if backoff == 0 {
		return
	}

	waitUntil := lastFailure.Add(backoff)
	sleep := time.Until(waitUntil)
	if sleep > 0 {
		time.Sleep(sleep)
	}
}

func (bp *BirdPool) recordDialFailure() {
	now := time.Now()
	bp.dialMu.Lock()
	if bp.dialBackoff == 0 {
		bp.dialBackoff = bp.dialBackoffMin
	} else {
		bp.dialBackoff *= 2
		if bp.dialBackoff > bp.dialBackoffMax {
			bp.dialBackoff = bp.dialBackoffMax
		}
	}
	bp.lastDialFailure = now
	bp.dialMu.Unlock()
}

func (bp *BirdPool) resetDialBackoff() {
	bp.dialMu.Lock()
	bp.dialBackoff = 0
	bp.lastDialFailure = time.Time{}
	bp.dialMu.Unlock()
}

func (bp *BirdPool) createConnection() (*BirdConn, error) {
	// With retry logic for handling connection / temporary resource unavailability
	var lastError error
	for attempt := 0; attempt < bp.connectionMaxRetries; attempt++ {
		bp.waitForDialWindow()
		bc, err := NewBirdConnection(bp.socketPath)
		if err != nil {
			lastError = err
			bp.recordDialFailure()
			// Retry on connection failure
			if attempt < bp.connectionMaxRetries-1 {
				time.Sleep(time.Duration(bp.connectionRetryDelayMs) * time.Millisecond)
				continue
			}
			return nil, lastError
		}
		bp.resetDialBackoff()

		restricted, restrictErr := bc.Restrict()
		if restrictErr != nil || !restricted {
			if restrictErr == nil {
				restrictErr = fmt.Errorf("failed to enter restricted mode")
			}
			lastError = restrictErr
			bc.Close()
			bp.recordDialFailure()
			// Don't retry restriction failures
			return nil, restrictErr
		}

		return bc, nil
	}

	return nil, fmt.Errorf("failed to create connection after %d attempts, last error: %v", bp.connectionMaxRetries, lastError)
}

func (bp *BirdPool) GetConnection() (*PooledConnection, error) {
	// Try to get an available connection from channel first (fast path)
	select {
	case pc := <-bp.available:
		if pc != nil && pc.conn != nil {
			bp.Lock()
			pc.inUse = true
			pc.lastUsed = time.Now()
			bp.Unlock()
			return pc, nil
		}
		// Connection is invalid, fall through to create new one
	default:
		// No immediately available connection, try to create new one or wait
	}

	bp.Lock()
	currentSize := len(bp.pool)
	if currentSize < bp.poolSizeMax {
		// Create a new connection
		birdConn, err := bp.createConnection()
		if err != nil {
			bp.Unlock()
			return nil, fmt.Errorf("failed to create new connection: %v", err)
		}

		pc := &PooledConnection{
			conn:     birdConn,
			lastUsed: time.Now(),
			inUse:    true,
			// id:       bp.nextID,
		}
		// bp.nextID++
		bp.pool = append(bp.pool, pc)
		bp.Unlock()
		return pc, nil
	}
	bp.Unlock()

	// Wait for an available connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for {
		select {
		case pc := <-bp.available:
			if pc != nil && pc.conn != nil {
				bp.Lock()
				pc.inUse = true
				pc.lastUsed = time.Now()
				bp.Unlock()
				return pc, nil
			}
			// Connection is invalid, continue waiting
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for available connection")
		case <-bp.shutdown:
			return nil, fmt.Errorf("connection pool is shutting down")
		}
	}
}

func (bp *BirdPool) ReleaseConnection(pc *PooledConnection) {
	if pc == nil {
		return
	}

	// Check if pool is shutting down
	select {
	case <-bp.shutdown:
		// Pool is shutting down, close the connection directly
		if pc.conn != nil {
			pc.conn.Close()
		}
		return
	default:
		// Pool is still active
	}

	bp.Lock()
	pc.inUse = false
	pc.lastUsed = time.Now()
	bp.Unlock()

	// Return connection to available pool (non-blocking)
	select {
	case bp.available <- pc:
		// Successfully returned to pool
	default:
		// Channel is full, connection will be picked up by maintenance
	}
}

// Close closes all connections in the pool
func (bp *BirdPool) Close() {
	// Safely close shutdown channel only once
	select {
	case <-bp.shutdown:
		// Already closed, return early
		return
	default:
		close(bp.shutdown) // Signal shutdown
	}

	bp.Lock()
	defer bp.Unlock()

	// Safely close available channel
	select {
	case <-bp.available:
		// Channel might already be closed, safe to continue
	default:
		close(bp.available)
	}

	// Drain the channel and close connections
	for {
		select {
		case pc := <-bp.available:
			if pc != nil && pc.conn != nil {
				pc.conn.Close()
			}
		default:
			goto drainComplete
		}
	}
drainComplete:

	// Close remaining connections in pool
	for _, pc := range bp.pool {
		if pc != nil && pc.conn != nil {
			pc.conn.Close()
			pc.conn = nil
		}
	}
	bp.pool = nil
}

// poolMaintenance periodically checks for and removes stale connections
func (bp *BirdPool) poolMaintenance() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			bp.Lock()
			if len(bp.pool) > bp.poolSize {
				now := time.Now()
				newPool := make([]*PooledConnection, 0, bp.poolSize)
				for _, pc := range bp.pool {
					if pc == nil {
						continue
					}

					// Keep if in use or if it's part of the base pool size
					if pc.inUse || len(newPool) < bp.poolSize {
						newPool = append(newPool, pc)
						continue
					}

					// Remove if connection is old and unused
					if now.Sub(pc.lastUsed) > 5*time.Minute {
						if pc.conn != nil {
							pc.conn.Close()
							pc.conn = nil
						}
					} else {
						newPool = append(newPool, pc)
					}
				}
				bp.pool = newPool
			}
			bp.Unlock()
		case <-bp.shutdown:
			return
		}
	}
}

func (bp *BirdPool) WithConnection(fn func(conn *BirdConn) error) error {
	pc, err := bp.GetConnection()
	if err != nil {
		return err
	}
	defer bp.ReleaseConnection(pc)

	// Check if connection is nil before use
	if pc.conn == nil {
		// Try to create a new connection
		newConn, createErr := bp.createConnection()
		if createErr != nil {
			return fmt.Errorf("connection is nil and failed to create new connection: %v", createErr)
		}
		bp.Lock()
		pc.conn = newConn
		bp.Unlock()
	}

	err = fn(pc.conn)
	if err != nil {
		// Try to reconnect on error - use atomic replacement
		newConn, reconnErr := bp.createConnection()
		if reconnErr == nil {
			bp.Lock()
			oldConn := pc.conn
			pc.conn = newConn
			bp.Unlock()

			// Close old connection safely
			if oldConn != nil {
				oldConn.Close()
			}

			// Retry the operation once with the new connection
			if pc.conn != nil {
				err = fn(pc.conn)
			}
		}
	}
	return err
}

func (bp *BirdPool) ShowStatus() (string, error) {
	var output string
	err := bp.WithConnection(func(conn *BirdConn) error {
		var buf bytes.Buffer
		if err := conn.Write("show status"); err != nil {
			return err
		}
		conn.Read(&buf)
		output = buf.String()
		return nil
	})
	return output, err
}

// ShowProtocols executes "show protocols" and returns the output
func (bp *BirdPool) ShowProtocols() (string, error) {
	var output string
	err := bp.WithConnection(func(conn *BirdConn) error {
		var buf bytes.Buffer
		buf.Grow(8192) // Pre-allocate buffer for protocols output
		if err := conn.Write("show protocols"); err != nil {
			return err
		}
		conn.Read(&buf)
		output = buf.String()
		return nil
	})
	return output, err
}

// ShowProtocolsAll executes "show protocols all <name>" and returns the output
func (bp *BirdPool) ShowProtocolsAll(name string) (string, error) {
	var output string
	err := bp.WithConnection(func(conn *BirdConn) error {
		var buf bytes.Buffer
		buf.Grow(4096)
		if err := conn.Write("show protocols all " + name); err != nil {
			return err
		}
		conn.Read(&buf)
		output = buf.String()
		return nil
	})
	return output, err
}

// ShowRouteAll executes "show route all" and returns the output
func (bp *BirdPool) ShowRouteAll() (string, error) {
	var output string
	err := bp.WithConnection(func(conn *BirdConn) error {
		var buf bytes.Buffer
		buf.Grow(65536) // Route table can be large
		if err := conn.Write("show route all"); err != nil {
			return err
		}
		conn.Read(&buf)
		output = buf.String()
		return nil
	})
	return output, err
}

// ShowRouteForPrefix executes "show route for <prefix>" and returns the output
func (bp *BirdPool) ShowRouteForPrefix(prefix string) (string, error) {
	var output string
	err := bp.WithConnection(func(conn *BirdConn) error {
		var buf bytes.Buffer
		buf.Grow(8192)
		if err := conn.Write("show route for " + prefix); err != nil {
			return err
		}
		conn.Read(&buf)
		output = buf.String()
		return nil
	})
	return output, err
}

// This does not affect by pool size, always use a new conn
func (bp *BirdPool) Configure() (bool, error) {
	bp.waitForDialWindow()
	bc, err := NewBirdConnection(bp.socketPath)
	if err != nil {
		bp.recordDialFailure()
		return false, err
	}
	bp.resetDialBackoff()
	defer bc.Close()

	if err := bc.Write("configure"); err != nil {
		return false, err
	}

	// Dismiss output
	bc.Read(nil)
	return true, nil
}

// GetProtocolStatus executes "show protocols all <sessionName>" and extracts route statistics
// Returns route counts for IPv4 and IPv6 (imported and exported), along with protocol state, since time, and info
func (bp *BirdPool) GetProtocolStatus(sessionName string) (string, string, string, int64, int64, int64, int64, error) {
	var output string

	err := bp.WithConnection(func(conn *BirdConn) error {
		var buf bytes.Buffer
		buf.Grow(4096) // Pre-allocate buffer to reduce allocations

		if err := conn.Write("show protocols all " + sessionName); err != nil {
			return err
		}
		conn.Read(&buf)
		output = buf.String()
		return nil
	})
	if err != nil {
		return "", "", "", 0, 0, 0, 0, err
	}

	// Parse the output using optimized byte operations
	return parseProtocolOutput([]byte(output))
}

// parseProtocolOutput optimizes the parsing of BIRD protocol output
func parseProtocolOutput(data []byte) (string, string, string, int64, int64, int64, int64, error) {
	var (
		state      string
		since      string
		info       string
		ipv4Import int64
		ipv4Export int64
		ipv6Import int64
		ipv6Export int64
	)

	lines := bytes.Split(data, []byte("\n"))
	var currentChannel string

	// Parse first line for state, since, and info
	if len(lines) > 1 {
		dataLine := lines[1]
		fields := bytes.Fields(dataLine)

		// Fields: [Name, Proto, Table, State, Since+Date, Since+Time, Info...]
		// xxxx BGP        ---        up     2025-06-12 16:11:45  Established
		// yyyy BGP        ---        down   2025-06-12 16:11:45  Active Socket: Reason
		if len(fields) >= 7 {
			state = string(fields[3])

			// Combine date and time for since field
			since = string(fields[4])
			if len(fields) > 5 {
				since += " " + string(fields[5])
			}

			// Combine remaining fields for info
			if len(fields) > 6 {
				infoFields := fields[6:]
				infoBytes := bytes.Join(infoFields, []byte(" "))
				info = string(infoBytes)
			}
		}
	}

	// Process remaining lines for channel information
	for _, line := range lines[2:] {
		lineStr := string(bytes.TrimSpace(line))

		// Detect channel using regex
		if matches := channelRegex.FindStringSubmatch(lineStr); len(matches) > 1 {
			currentChannel = matches[1]
			continue
		}

		// Check for DOWN state using regex
		if stateDownRegex.Match(bytes.TrimSpace(line)) {
			switch currentChannel {
			case "ipv4":
				ipv4Import = 0
				ipv4Export = 0
			case "ipv6":
				ipv6Import = 0
				ipv6Export = 0
			}
			continue
		}

		// Extract route counts using regex
		if matches := routeLineRegex.FindStringSubmatch(lineStr); len(matches) > 2 {
			imported, err1 := strconv.ParseInt(matches[1], 10, 64)
			exported, err2 := strconv.ParseInt(matches[2], 10, 64)

			if err1 == nil && err2 == nil {
				switch currentChannel {
				case "ipv4":
					ipv4Import = imported
					ipv4Export = exported
				case "ipv6":
					ipv6Import = imported
					ipv6Export = exported
				}
			}
		}
	}

	return state, since, info, ipv4Import, ipv4Export, ipv6Import, ipv6Export, nil
}

// Pre-compiled regex patterns for better performance
var (
	routeLineRegex = regexp.MustCompile(`^Routes:\s+(\d+)\s+imported,(?:\s+\d+\s+filtered,)?\s+(\d+)\s+exported(?:,\s+(\d+)\s+preferred)?`)
	channelRegex   = regexp.MustCompile(`^Channel\s+(ipv[46])$`)
	stateDownRegex = regexp.MustCompile(`^State:.*DOWN`)

	// Additional optimized patterns for protocol parsing
	protocolLineRegex = regexp.MustCompile(`^(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(.+)$`)
	sinceTimeRegex    = regexp.MustCompile(`(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})`)

	// LG parsing patterns
	routePrefixRegex  = regexp.MustCompile(`^(\S+\s+\S+|\S+)\s+via\s+\S+\s+on\s+(\S+)`)
	routeUnreachable  = regexp.MustCompile(`^(\S+\s+\S+|\S+)\s+\[.*\]\s+\* \(.*\)`)

	// Protocol summary line: name proto table state since... info...
	// Example: dn42_abc456 BGP --- up 2025-06-12 16:11:45 Established
	// Example: dn42_abc456 BGP --- down 2025-06-12 16:11:45 Active
	protoSummaryRe = regexp.MustCompile(`^(\S+)\s+(\S+)\s+(\S+)\s+(up|down)\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})\s*(.*)`)

	// Route entry: prefix [prefs] * (type) info
	// 172.20.x.x/32         via 10.x.x.x on dn42_xxx [xxx 14:20 from x.x.x.x] * (BGP/...)
	routeEntryRe = regexp.MustCompile(`^([\d:a-fA-F./]+)\s+(.*)`)

	// Route detail fields
	routeViaRe    = regexp.MustCompile(`via\s+(\S+)\s+on\s+(\S+)`)
	routeProtoRe  = regexp.MustCompile(`\((\w+)(?:/(\w+))?\)`)
	routeSinceRe  = regexp.MustCompile(`(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})`)
	routeFromRe   = regexp.MustCompile(`from\s+([^\s\]]+)`)
	routeMetricRe = regexp.MustCompile(`metric\s+(\d+)`)

	// BGP-level detail field patterns
	bgpStateRe        = regexp.MustCompile(`BGP state:\s+(\S+)`)
	neighborAddrRe    = regexp.MustCompile(`Neighbor address:\s+(\S+)`)
	neighborASRe      = regexp.MustCompile(`Neighbor AS:\s+(\d+)`)
	localASRe         = regexp.MustCompile(`Local AS:\s+(\d+)`)
	neighborIDRe      = regexp.MustCompile(`Neighbor ID:\s+(\S+)`)
	sourceAddrRe      = regexp.MustCompile(`Source address:\s+(\S+)`)
	holdTimerRe       = regexp.MustCompile(`Hold timer:\s+(\S+)`)
	keepaliveTimerRe  = regexp.MustCompile(`Keepalive timer:\s+(\S+)`)
	connectDelayRe    = regexp.MustCompile(`Connect delay:\s+(\S+)`)
	lastErrorRe       = regexp.MustCompile(`Last error:\s+(.+)`)
	sessionRe         = regexp.MustCompile(`Session:\s+(.+)`)
	hostnameRe        = regexp.MustCompile(`Hostname:\s+(.+)`)

	// Channel-level detail field patterns
	channelTableRe       = regexp.MustCompile(`Table:\s+(\S+)`)
	channelPreferenceRe  = regexp.MustCompile(`Preference:\s+(\d+)`)
	channelInputFilterRe = regexp.MustCompile(`Input filter:\s+(.+)`)
	channelOutputFilterRe = regexp.MustCompile(`Output filter:\s+(.+)`)
	channelImportLimitRe = regexp.MustCompile(`Import limit:\s+(\d+)`)
	channelStateRe       = regexp.MustCompile(`State:\s+(\S+)`)
	bgpNextHopRe         = regexp.MustCompile(`BGP Next hop:\s+(.+)`)
	routeChangeLineRe    = regexp.MustCompile(`^\s*(Import|Export)\s+(updates|withdraws):\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)`)
)

// BGPState represents BGP-specific state information
type BGPState struct {
	State           string `json:"state,omitempty"`
	NeighborAddress string `json:"neighbor_address,omitempty"`
	NeighborAS      int64  `json:"neighbor_as,omitempty"`
	LocalAS         int64  `json:"local_as,omitempty"`
	NeighborID      string `json:"neighbor_id,omitempty"`
	SourceAddress   string `json:"source_address,omitempty"`
	HoldTimer       string `json:"hold_timer,omitempty"`
	KeepaliveTimer  string `json:"keepalive_timer,omitempty"`
	ConnectDelay    string `json:"connect_delay,omitempty"`
	LastError       string `json:"last_error,omitempty"`
	Session         string `json:"session,omitempty"`
	Hostname        string `json:"hostname,omitempty"`
}

// ProtocolSummary represents a single protocol from "show protocols"
type ProtocolSummary struct {
	Name    string `json:"name"`
	Proto   string `json:"proto"`
	Table   string `json:"table"`
	State   string `json:"state"`
	Since   string `json:"since"`
	Info    string `json:"info,omitempty"`
}

// ProtocolDetail represents detailed protocol info from "show protocols all <name>"
type ProtocolDetail struct {
	Name         string                     `json:"name"`
	Proto        string                     `json:"proto"`
	Table        string                     `json:"table"`
	State        string                     `json:"state"`
	Since        string                     `json:"since"`
	Info         string                     `json:"info,omitempty"`
	Channels     []ChannelInfo              `json:"channels"`
	BGP          *BGPState                  `json:"bgp,omitempty"`
}

// parseRCSNum parses a route change stat number, treating "---" as 0
func parseRCSNum(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "---" || s == "" {
		return 0
	}
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// RCSCounters holds the 5-column route change stat counters
type RCSCounters struct {
	Received int64 `json:"received,omitempty"`
	Rejected int64 `json:"rejected,omitempty"`
	Filtered int64 `json:"filtered,omitempty"`
	Ignored  int64 `json:"ignored,omitempty"`
	Accepted int64 `json:"accepted,omitempty"`
}

// RouteChangeStats holds per-direction route change counters for a channel
type RouteChangeStats struct {
	ImportUpdates   RCSCounters `json:"import_updates,omitempty"`
	ImportWithdraws RCSCounters `json:"import_withdraws,omitempty"`
	ExportUpdates   RCSCounters `json:"export_updates,omitempty"`
	ExportWithdraws RCSCounters `json:"export_withdraws,omitempty"`
}

// ChannelInfo represents a BIRD channel (ipv4/ipv6)
type ChannelInfo struct {
	Name             string            `json:"name"`
	State            string            `json:"state"`
	Imported         int64             `json:"imported"`
	Exported         int64             `json:"exported"`
	Filtered         int64             `json:"filtered,omitempty"`
	Preferred        int64             `json:"preferred,omitempty"`
	Table            string            `json:"table,omitempty"`
	Preference       int64             `json:"preference,omitempty"`
	InputFilter      string            `json:"input_filter,omitempty"`
	OutputFilter     string            `json:"output_filter,omitempty"`
	ImportLimit      int64             `json:"import_limit,omitempty"`
	RouteChangeStats *RouteChangeStats `json:"route_change_stats,omitempty"`
	BGPNextHop       string            `json:"bgp_next_hop,omitempty"`
}

// RouteEntry represents a single routing table entry
type RouteEntry struct {
	Prefix   string `json:"prefix"`
	Interface string `json:"interface,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Type     string `json:"type,omitempty"`
	Since    string `json:"since,omitempty"`
	From     string `json:"from,omitempty"`
	Metric   int64  `json:"metric,omitempty"`
	Primary  bool   `json:"primary"`
}

// ParseProtocolsSummary parses "show protocols" output into structured data
// BIRD output format:
//
// Name       Proto      Table      State  Since         Info
// dn42_xxx   BGP        ---        up     2025-06-12 16:11:45  Established
// static1    Static     master4    up     2025-01-01 00:00:00
func ParseProtocolsSummary(raw string) []ProtocolSummary {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	var result []ProtocolSummary

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip header line
		if strings.HasPrefix(line, "Name ") || strings.HasPrefix(line, "name ") {
			continue
		}

		// Try structured regex first
		if m := protoSummaryRe.FindStringSubmatch(line); len(m) >= 6 {
			p := ProtocolSummary{
				Name:  m[1],
				Proto: m[2],
				Table: m[3],
				State: m[4],
				Since: strings.TrimSpace(m[5]),
			}
			if len(m) > 6 {
				p.Info = strings.TrimSpace(m[6])
			}
			result = append(result, p)
			continue
		}

		// Fallback: space-separated fields
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			p := ProtocolSummary{
				Name:  fields[0],
				Proto: fields[1],
				Table: fields[2],
				State: fields[3],
			}
			if len(fields) >= 6 {
				p.Since = fields[4] + " " + fields[5]
			}
			if len(fields) >= 7 {
				p.Info = strings.Join(fields[6:], " ")
			}
			result = append(result, p)
		}
	}

	return result
}

// ParseProtocolDetail parses "show protocols all <name>" output into structured data
// This is more complex as it includes channel information with route counts
func ParseProtocolDetail(raw string) *ProtocolDetail {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) == 0 {
		return nil
	}

	detail := &ProtocolDetail{}

	// Parse first data line (line[1] after header)
	if len(lines) > 1 {
		firstLine := strings.TrimSpace(lines[1])
		if m := protoSummaryRe.FindStringSubmatch(firstLine); len(m) >= 6 {
			detail.Name = m[1]
			detail.Proto = m[2]
			detail.Table = m[3]
			detail.State = m[4]
			detail.Since = strings.TrimSpace(m[5])
			if len(m) > 6 {
				detail.Info = strings.TrimSpace(m[6])
			}
		} else {
			fields := strings.Fields(firstLine)
			if len(fields) >= 4 {
				detail.Name = fields[0]
				detail.Proto = fields[1]
				detail.Table = fields[2]
				detail.State = fields[3]
				if len(fields) >= 6 {
					detail.Since = fields[4] + " " + fields[5]
				}
				if len(fields) >= 7 {
					detail.Info = strings.Join(fields[6:], " ")
				}
			}
		}
	}

	// Parse BGP-level fields and channels
	var currentChannel *ChannelInfo
	bgp := &BGPState{}
	hasBgpData := false

	for _, line := range lines[2:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Channel header: "Channel ipv4" or "Channel ipv6"
		if m := channelRegex.FindStringSubmatch(line); len(m) > 1 {
			if currentChannel != nil {
				detail.Channels = append(detail.Channels, *currentChannel)
			}
			currentChannel = &ChannelInfo{
				Name:  m[1],
				State: "up",
			}
			continue
		}

		if currentChannel != nil {
			// Channel State: UP / DOWN
			if m := channelStateRe.FindStringSubmatch(line); len(m) > 1 {
				currentChannel.State = strings.ToLower(m[1])
				continue
			}
			// Channel Table
			if m := channelTableRe.FindStringSubmatch(line); len(m) > 1 {
				currentChannel.Table = m[1]
				continue
			}
			// Channel Preference
			if m := channelPreferenceRe.FindStringSubmatch(line); len(m) > 1 {
				currentChannel.Preference, _ = strconv.ParseInt(m[1], 10, 64)
				continue
			}
			// Channel Input filter
			if m := channelInputFilterRe.FindStringSubmatch(line); len(m) > 1 {
				currentChannel.InputFilter = m[1]
				continue
			}
			// Channel Output filter
			if m := channelOutputFilterRe.FindStringSubmatch(line); len(m) > 1 {
				currentChannel.OutputFilter = m[1]
				continue
			}
			// Channel Import limit
			if m := channelImportLimitRe.FindStringSubmatch(line); len(m) > 1 {
				currentChannel.ImportLimit, _ = strconv.ParseInt(m[1], 10, 64)
				continue
			}
			// Routes: N imported, M exported, K preferred
			if m := routeLineRegex.FindStringSubmatch(line); len(m) > 2 {
				currentChannel.Imported, _ = strconv.ParseInt(m[1], 10, 64)
				currentChannel.Exported, _ = strconv.ParseInt(m[2], 10, 64)
				if len(m) > 3 && m[3] != "" {
					currentChannel.Preferred, _ = strconv.ParseInt(m[3], 10, 64)
				}
				continue
			}
			// BGP Next hop (per channel)
			if m := bgpNextHopRe.FindStringSubmatch(line); len(m) > 1 {
				currentChannel.BGPNextHop = m[1]
				continue
			}
			// Route change stats line: "Import updates: N N N N N" etc
			if m := routeChangeLineRe.FindStringSubmatch(line); len(m) > 2 {
				if currentChannel.RouteChangeStats == nil {
					currentChannel.RouteChangeStats = &RouteChangeStats{}
				}
				rcs := currentChannel.RouteChangeStats
				counters := RCSCounters{
					Received: parseRCSNum(m[3]),
					Rejected: parseRCSNum(m[4]),
					Filtered: parseRCSNum(m[5]),
					Ignored:  parseRCSNum(m[6]),
					Accepted: parseRCSNum(m[7]),
				}
				direction := m[1]   // Import or Export
				action := m[2]      // updates or withdraws
				switch {
				case direction == "Import" && action == "updates":
					rcs.ImportUpdates = counters
				case direction == "Import" && action == "withdraws":
					rcs.ImportWithdraws = counters
				case direction == "Export" && action == "updates":
					rcs.ExportUpdates = counters
				case direction == "Export" && action == "withdraws":
					rcs.ExportWithdraws = counters
				}
				continue
			}
			// Filter count (if present): "  N filtered"
			if strings.Contains(line, "filtered") && currentChannel != nil {
				filterRe := regexp.MustCompile(`(\d+)\s+filtered`)
				if fm := filterRe.FindStringSubmatch(line); len(fm) > 1 {
					currentChannel.Filtered, _ = strconv.ParseInt(fm[1], 10, 64)
				}
			}
			continue
		}

		// BGP-level fields (before any channel section)
		if m := bgpStateRe.FindStringSubmatch(line); len(m) > 1 {
			bgp.State = m[1]; hasBgpData = true
			continue
		}
		if m := neighborAddrRe.FindStringSubmatch(line); len(m) > 1 {
			bgp.NeighborAddress = m[1]; hasBgpData = true
			continue
		}
		if m := neighborASRe.FindStringSubmatch(line); len(m) > 1 {
			bgp.NeighborAS, _ = strconv.ParseInt(m[1], 10, 64); hasBgpData = true
			continue
		}
		if m := localASRe.FindStringSubmatch(line); len(m) > 1 {
			bgp.LocalAS, _ = strconv.ParseInt(m[1], 10, 64); hasBgpData = true
			continue
		}
		if m := neighborIDRe.FindStringSubmatch(line); len(m) > 1 {
			bgp.NeighborID = m[1]; hasBgpData = true
			continue
		}
		if m := sourceAddrRe.FindStringSubmatch(line); len(m) > 1 {
			bgp.SourceAddress = m[1]; hasBgpData = true
			continue
		}
		if m := holdTimerRe.FindStringSubmatch(line); len(m) > 1 {
			bgp.HoldTimer = m[1]; hasBgpData = true
			continue
		}
		if m := keepaliveTimerRe.FindStringSubmatch(line); len(m) > 1 {
			bgp.KeepaliveTimer = m[1]; hasBgpData = true
			continue
		}
		if m := connectDelayRe.FindStringSubmatch(line); len(m) > 1 {
			bgp.ConnectDelay = m[1]; hasBgpData = true
			continue
		}
		if m := lastErrorRe.FindStringSubmatch(line); len(m) > 1 {
			bgp.LastError = m[1]; hasBgpData = true
			continue
		}
		if m := sessionRe.FindStringSubmatch(line); len(m) > 1 {
			bgp.Session = m[1]; hasBgpData = true
			continue
		}
		if m := hostnameRe.FindStringSubmatch(line); len(m) > 1 {
			bgp.Hostname = m[1]; hasBgpData = true
			continue
		}
	}

	if currentChannel != nil {
		detail.Channels = append(detail.Channels, *currentChannel)
	}

	if hasBgpData {
		detail.BGP = bgp
	}

	return detail
}

// ParseRoutes parses "show route all" output into structured entries
// BIRD output format:
// 172.20.x.x/32 unicast [xxx 14:20 from x.x.x.x] * (BGP/...) [ASxxxxx i]
//     via 10.x.x.x on dn42_xxx
func ParseRoutes(raw string) []RouteEntry {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	var result []RouteEntry

	var currentPrefix string
	var currentEntry *RouteEntry

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			// Flush current entry
			if currentEntry != nil {
				result = append(result, *currentEntry)
				currentEntry = nil
			}
			continue
		}

		// New route entry line (starts with a prefix or ::/prefix)
		if routeEntryRe.MatchString(line) && !strings.HasPrefix(line, "via ") && !strings.HasPrefix(line, "from ") {
			// Flush previous entry
			if currentEntry != nil {
				result = append(result, *currentEntry)
			}

			// Extract prefix
			parts := strings.Fields(line)
			if len(parts) == 0 {
				continue
			}

			// Find where "via" starts, or parse inline fields
			prefix := parts[0]
			currentPrefix = prefix

			entry := RouteEntry{
				Prefix:  prefix,
				Primary: strings.Contains(line, "*"),
			}

			// Extract protocol type
			if m := routeProtoRe.FindStringSubmatch(line); len(m) > 1 {
				entry.Type = m[1]
				if len(m) > 2 && m[2] != "" {
					entry.Protocol = m[2]
				}
			}

			// Extract since
			if m := routeSinceRe.FindStringSubmatch(line); len(m) > 1 {
				entry.Since = m[1]
			}

			// Extract from (origin)
			if m := routeFromRe.FindStringSubmatch(line); len(m) > 1 {
				entry.From = m[1]
			}

			// Extract metric
			if m := routeMetricRe.FindStringSubmatch(line); len(m) > 1 {
				entry.Metric, _ = strconv.ParseInt(m[1], 10, 64)
			}

			// Extract via/on inline (some routes have it on the same line)
			if m := routeViaRe.FindStringSubmatch(line); len(m) > 1 {
				entry.Interface = m[2]
			}

			currentEntry = &entry
			continue
		}

		// Continuation line: "via X on Y"
		if strings.HasPrefix(line, "via ") || strings.HasPrefix(line, "from ") {
			if currentEntry != nil {
				if m := routeViaRe.FindStringSubmatch(line); len(m) > 1 {
					currentEntry.Interface = m[2]
				}
			}
			continue
		}

		// If we're here, it's a multi-line entry detail
		// Try to extract any additional info
		if currentEntry != nil {
			if m := routeViaRe.FindStringSubmatch(line); len(m) > 1 {
				currentEntry.Interface = m[2]
			}
			if m := routeFromRe.FindStringSubmatch(line); len(m) > 1 && currentEntry.From == "" {
				currentEntry.From = m[1]
			}
		}

		_ = currentPrefix
	}

	// Flush last entry
	if currentEntry != nil {
		result = append(result, *currentEntry)
	}

	return result
}

// Compile patterns once at package initialization for optimal performance
func init() {
	// Verify all patterns compile correctly
	patterns := []*regexp.Regexp{
		routeLineRegex,
		channelRegex,
		stateDownRegex,
		protocolLineRegex,
		sinceTimeRegex,
	}

	for i, pattern := range patterns {
		if pattern == nil {
			panic(fmt.Sprintf("Failed to compile regex pattern %d", i))
		}
	}
}
