package main

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"slices"

	"github.com/matishsiao/goInfo"
	"github.com/oschwald/geoip2-golang"
)

func getOsUname() string {
	gi, _ := goInfo.GetInfo()
	platform := gi.Platform
	if strings.ToLower(platform) == "unknown" {
		platform = runtime.GOARCH
	}
	return fmt.Sprintf("%s %s %s", gi.Kernel, gi.Core, platform)
}

func countConnections(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		// It’s fine if the system doesn't support IPv6, just return 0
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0

	firstLine := true
	for scanner.Scan() {
		if firstLine {
			firstLine = false // skip header
			continue
		}
		count++
	}
	return count, scanner.Err()
}

func getTcpConnections() int {
	tcp4, _ := countConnections("/proc/net/tcp")
	tcp6, _ := countConnections("/proc/net/tcp6")
	return tcp4 + tcp6
}

func getUdpConnections() int {
	udp4, _ := countConnections("/proc/net/udp")
	udp6, _ := countConnections("/proc/net/udp6")
	return udp4 + udp6
}

func getUptimeSeconds() float64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	parts := strings.Fields(string(data))
	if len(parts) < 1 {
		return 0
	}
	uptime, _ := strconv.ParseFloat(parts[0], 64)
	return uptime
}

func getInterfaceTraffic(interfaces []string) (rxTotal, txTotal uint64, err error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	ifaceSet := make(map[string]struct{})
	for _, iface := range interfaces {
		ifaceSet[iface] = struct{}{}
	}

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		if lineCount <= 2 {
			// Skip headers
			continue
		}

		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		iface := strings.TrimSpace(parts[0])
		if _, ok := ifaceSet[iface]; !ok {
			continue
		}

		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}

		// RX bytes = fields[0], TX bytes = fields[8]
		var rx, tx uint64
		fmt.Sscanf(fields[0], "%d", &rx)
		fmt.Sscanf(fields[8], "%d", &tx)

		rxTotal += rx
		txTotal += tx
	}

	if err := scanner.Err(); err != nil {
		return 0, 0, err
	}

	return rxTotal, txTotal, nil
}

func getLoadAverage() (load1, load5, load15 float64, err error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}

	parts := strings.Fields(string(data))
	if len(parts) < 3 {
		return 0, 0, 0, fmt.Errorf("unexpected format: %s", data)
	}

	load1, err = strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return
	}
	load5, err = strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return
	}
	load15, err = strconv.ParseFloat(parts[2], 64)
	return
}

func getLoadAverageStr() string {
	load1, load5, load15, err := getLoadAverage()
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%.2f %.2f %.2f", load1, load5, load15)
}

func interfaceExists(iface string) (exist bool, err error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		if lineCount <= 2 {
			// Skip headers
			continue
		}

		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		_iface := strings.TrimSpace(parts[0])
		if _iface == iface {
			return true, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return false, err
	}

	return false, nil
}

func icmpPingAverage(address string, tries, timeoutSeconds int) (int, float64) {
	// Use system native ping command
	rtt, loss, err := systemPing(address, tries, timeoutSeconds)
	if err != nil {
		return -1, 1.0
	}
	return rtt, loss
}

// systemPing executes the system(Linux iputils-ping) native ping command and parses the result
func systemPing(address string, count, timeoutSeconds int) (int, float64, error) {
	// Determine if we're dealing with IPv6
	isIPv6 := strings.Contains(address, ":") || strings.Contains(address, "%")

	// Build ping command arguments
	var args []string
	if isIPv6 {
		args = append(args, "-6") // Force IPv6
	} else {
		args = append(args, "-4") // Force IPv4
	}

	// Add common arguments
	const interval float64 = 0.2                                          // interval seconds between packets (200ms)
	args = append(args, "-c", strconv.Itoa(count))                        // packet count
	args = append(args, "-W", strconv.Itoa(timeoutSeconds))               // timeout per packet
	args = append(args, "-i", strconv.FormatFloat(interval, 'f', -1, 64)) // interval between packets (200ms)
	args = append(args, "-q")                                             // quiet mode (only summary)
	args = append(args, address)                                          // target address

	// Create context with timeout (add another 5 seconds buffer to deadline)
	deadline := timeoutSeconds*count + int(float64(count)*interval) + 5 // deadline in seconds
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(deadline)*time.Second)
	defer cancel()

	// Execute ping command
	cmd := exec.CommandContext(ctx, cfg.Metric.PingCommandPath, args...)

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it's a timeout or context cancellation
		if ctx.Err() == context.DeadlineExceeded {
			return -1, 1.0, fmt.Errorf("ping timeout")
		}
		// For other errors (host unreachable, etc.), try to parse what we can
		if len(output) > 0 {
			return parsePingOutput(string(output))
		}
		return -1, 1.0, fmt.Errorf("ping failed: %w", err)
	}

	return parsePingOutput(string(output))
}

// parsePingOutput parses the output from the ping command
func parsePingOutput(output string) (int, float64, error) {
	// Example output:
	// PING 172.23.91.1 (172.23.91.1) 56(84) bytes of data.
	//
	// --- 172.23.91.1 ping statistics ---
	// 4 packets transmitted, 4 received, 0% packet loss, time 3081ms
	// rtt min/avg/max/mdev = 0.392/0.426/0.503/0.044 ms

	lines := strings.Split(output, "\n")

	var packetLoss float64 = 1.0 // Default to 100% loss
	var avgRTT int = -1          // Default to -1 (no RTT)

	// Parse packet loss from statistics line
	packetLossRegex := regexp.MustCompile(`(\d+)% packet loss`)
	rttRegex := regexp.MustCompile(`rtt min/avg/max/mdev = ([\d.]+)/([\d.]+)/([\d.]+)/([\d.]+) ms`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Parse packet loss
		if matches := packetLossRegex.FindStringSubmatch(line); len(matches) > 1 {
			if lossPercent, err := strconv.Atoi(matches[1]); err == nil {
				packetLoss = float64(lossPercent) / 100.0
			}
		}

		// Parse RTT statistics
		if matches := rttRegex.FindStringSubmatch(line); len(matches) > 2 {
			if avgFloat, err := strconv.ParseFloat(matches[2], 64); err == nil {
				avgRTT = int(avgFloat) // Convert to milliseconds
			}
		}
	}

	// If we got 100% packet loss, return -1 for RTT
	if packetLoss >= 1.0 {
		return -1, 1.0, nil
	}

	// If we couldn't parse RTT but had some successful packets, estimate from packet loss
	if avgRTT == -1 && packetLoss < 1.0 {
		return -1, packetLoss, fmt.Errorf("could not parse RTT from ping output")
	}

	return avgRTT, packetLoss, nil
}

// Parse input string (with or without port) and extract IP/hostname
func extractHost(addr string) string {
	// Handle IPv6 [::1]:443
	if strings.HasPrefix(addr, "[") {
		if i := strings.Index(addr, "]"); i != -1 {
			return addr[1:i]
		}
	}
	// For host:port
	if h, _, err := net.SplitHostPort(addr); err == nil {
		return h
	}
	return addr // No port
}

// Resolve hostname to IP
func resolveToIP(host string) (net.IP, error) {
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return nil, err
	}
	return ips[0], nil // fallback to first (IPv6)
}

// geoIPCountryCode returns the 2-letter country code from IP/hostname
func geoIPCountryCode(db *geoip2.Reader, input string) (string, error) {
	host := extractHost(input)
	ip, err := resolveToIP(host)
	if err != nil {
		return "", err
	}
	record, err := db.Country(ip)
	if err != nil {
		return "", err
	}
	return strings.ToUpper(record.Country.IsoCode), nil
}

// getInterfaceMTU returns the MTU of the given interface from /sys/class/net/<iface>/mtu
func getInterfaceMTU(name string) (int, error) {
	data, err := os.ReadFile("/sys/class/net/" + name + "/mtu")
	if err != nil {
		return 0, fmt.Errorf("failed to read MTU: %w", err)
	}
	mtuStr := strings.TrimSpace(string(data))
	mtu, err := strconv.Atoi(mtuStr)
	if err != nil {
		return 0, fmt.Errorf("invalid MTU format: %w", err)
	}
	return mtu, nil
}

func getInterfaceMAC(ifaceName string) (string, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return "", err
	}
	mac := iface.HardwareAddr.String()
	if mac == "" {
		return "", fmt.Errorf("no MAC address found for interface %s", ifaceName)
	}
	return mac, nil
}

// getInterfaceFlags returns the list of string flags (e.g. UP, BROADCAST) for the given interface
func getInterfaceFlags(name string) (string, error) {
	data, err := os.ReadFile("/sys/class/net/" + name + "/flags")
	if err != nil {
		return "", fmt.Errorf("failed to read flags: %w", err)
	}
	flagStr := strings.TrimSpace(string(data))
	flags, err := strconv.ParseUint(flagStr, 0, 64)
	if err != nil {
		return "", fmt.Errorf("invalid flags format: %w", err)
	}

	var result []string
	for bit, name := range interfaceFlagMap {
		if flags&bit != 0 {
			result = append(result, name)
		}
	}
	slices.Sort(result)
	return strings.Join(result, ",\x20"), nil
}

var interfaceFlagMap = map[uint64]string{
	0x1:     "UP",
	0x2:     "Broadcast",
	0x4:     "Debug",
	0x8:     "Loopback",
	0x10:    "PointToPoint",
	0x20:    "NoTrailers",
	0x40:    "Running",
	0x80:    "NoARP",
	0x100:   "Promisc",
	0x200:   "AllMulti",
	0x400:   "Master",
	0x800:   "Slave",
	0x1000:  "Multicast",
	0x2000:  "PortSel",
	0x4000:  "AutoMedia",
	0x8000:  "Dynamic",
	0x10000: "LowerUp",
	0x20000: "Dormant",
	0x40000: "Echo",
}

func _getRandomUnusedPort(proto string) (int, error) {
	var addr *net.UDPAddr
	var err error

	switch proto {
	case "tcp":
		// Use port :0 to let the OS choose a free port
		l, err := net.Listen("tcp", ":0")
		if err != nil {
			return 0, err
		}
		defer l.Close()
		return l.Addr().(*net.TCPAddr).Port, nil
	case "udp":
		addr, err = net.ResolveUDPAddr("udp", ":0")
		if err != nil {
			return 0, err
		}
		c, err := net.ListenUDP("udp", addr)
		if err != nil {
			return 0, err
		}
		defer c.Close()
		return c.LocalAddr().(*net.UDPAddr).Port, nil
	default:
		return 0, fmt.Errorf("unsupported protocol: %s", proto)
	}
}

var (
	reservedPorts      = make(map[int]int64)
	reservedPortsMutex sync.Mutex
)

// GetRandomUnusedPort returns a random unused port based on protocol ("tcp" or "udp")
func getRandomUnusedPort(proto string) (int, error) {
	const maxRetries = 10

	for range maxRetries {
		port, err := _getRandomUnusedPort(proto)
		if err != nil {
			return 0, fmt.Errorf("failed to get random unused port: %w", err)
		}
		reservedPortsMutex.Lock()
		if expireTimestamp, exists := reservedPorts[port]; exists {
			if time.Now().Unix() < expireTimestamp {
				// Port is reserved, skip it
				reservedPortsMutex.Unlock()
				continue // try again
			}
		}
		reservedPorts[port] = time.Now().Unix() + 300
		reservedPortsMutex.Unlock()
		return port, nil
	}

	return 0, fmt.Errorf("unable to find unused port after %d attempts", maxRetries)
}

// getRandomPortInRange returns a random unused port within the specified range
func getRandomPortInRange(proto string, minPort, maxPort int) (int, error) {
	const maxRetries = 20

	for range maxRetries {
		// Generate random port in range
		port := minPort + rand.Intn(maxPort-minPort+1)

		// Check if port is already reserved
		reservedPortsMutex.Lock()
		if expireTimestamp, exists := reservedPorts[port]; exists {
			if time.Now().Unix() < expireTimestamp {
				reservedPortsMutex.Unlock()
				continue
			}
		}

		// Try to bind to verify port is available
		var addr net.Addr
		var err error

		switch proto {
		case "tcp":
			addr, err = net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", port))
			if err != nil {
				reservedPortsMutex.Unlock()
				continue
			}
			l, err := net.ListenTCP("tcp", addr.(*net.TCPAddr))
			if err != nil {
				reservedPortsMutex.Unlock()
				continue
			}
			l.Close()
		case "udp":
			addr, err = net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
			if err != nil {
				reservedPortsMutex.Unlock()
				continue
			}
			c, err := net.ListenUDP("udp", addr.(*net.UDPAddr))
			if err != nil {
				reservedPortsMutex.Unlock()
				continue
			}
			c.Close()
		default:
			reservedPortsMutex.Unlock()
			return 0, fmt.Errorf("unsupported protocol: %s", proto)
		}

		// Port is available, reserve it
		reservedPorts[port] = time.Now().Unix() + 300
		reservedPortsMutex.Unlock()
		return port, nil
	}

	return 0, fmt.Errorf("unable to find unused port in range %d-%d after %d attempts", minPort, maxPort, maxRetries)
}

// validateInterfaceIP checks if an IP address is allowed for interface assignment
// Returns true if the IP is allowed, false if it should be blocked
func validateInterfaceIP(ipAddr string) (bool, error) {
	if ipAddr == "" {
		return true, nil // Empty IP is allowed
	}

	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return false, fmt.Errorf("invalid IP address: %s", ipAddr)
	}

	// Check if public IPs are allowed
	if !cfg.PeerAPI.InterfaceIpAllowPublic {
		// Check if it's a public IP
		if isPublicIP(ip) {
			return false, fmt.Errorf("public IP addresses are not allowed: %s", ipAddr)
		}
	}

	// Check blacklist
	for _, blacklistEntry := range cfg.PeerAPI.InterfaceIpBlacklist {
		if isIPInCIDR(ipAddr, blacklistEntry) {
			return false, fmt.Errorf("IP %s is in blacklist range: %s", ipAddr, blacklistEntry)
		}
	}

	return true, nil
}

// isPublicIP checks if an IP address is in public IP ranges
func isPublicIP(ip net.IP) bool {
	// Define private IP ranges
	privateRanges := []string{
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"127.0.0.0/8",    // Loopback
		"169.254.0.0/16", // Link-local
		"224.0.0.0/4",    // Multicast
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
		"fc00::/7",       // IPv6 unique local
		"ff00::/8",       // IPv6 multicast
	}

	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return false // It's private, not public
		}
	}

	return true // It's public
}

// isIPInCIDR checks if an IP address is within a CIDR range
func isIPInCIDR(ipAddr, cidr string) bool {
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return false
	}

	// Handle single IP case (no CIDR notation)
	if !strings.Contains(cidr, "/") {
		blacklistIP := net.ParseIP(cidr)
		if blacklistIP == nil {
			return false
		}
		return ip.Equal(blacklistIP)
	}

	// Handle CIDR case
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}

	return network.Contains(ip)
}

// extractHostFromEndpoint extracts the IP/hostname portion from an endpoint (removing port if present)
func extractHostFromEndpoint(endpoint string) string {
	if endpoint == "" {
		return ""
	}

	// Try to split host and port
	if host, _, err := net.SplitHostPort(endpoint); err == nil {
		return host
	}

	// If no port present, return the endpoint as is
	return endpoint
}
