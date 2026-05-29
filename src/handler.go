package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// Helper functions for HTTP response handling
func sendJSONResponse(w http.ResponseWriter, statusCode int, message string, data any) {
	w.Header().Set("Content-Type", "application/json")

	httpStatusCode := statusCode
	if statusCode == 0 {
		httpStatusCode = http.StatusOK
	}

	response := AgentApiResponse{
		Code:    statusCode,
		Message: message,
		Data:    nil,
	}

	if data != nil {
		response.Data = data
	}

	w.WriteHeader(httpStatusCode)
	json.NewEncoder(w).Encode(response)
}

// setHTTPClientHeader sets the necessary headers for an outbound HTTP request
func setHTTPClientHeader(r *http.Request, token string, setJsonContentType bool) {
	if token != "" {
		r.Header.Set("Authorization", "Bearer\x20"+token)
	}
	r.Header.Set("User-Agent", SERVER_SIGNATURE)
	if setJsonContentType {
		r.Header.Set("Content-Type", "application/json")
	}
}

func initRouter(mux *http.ServeMux) http.Handler {
	// Register routes
	mux.HandleFunc("/status", withAuth(status))
	mux.HandleFunc("/sync", withAuth(manualSync))
	mux.HandleFunc("/info", withAuth(nodePassthroughInfo))
	mux.HandleFunc("/lg/protocols", withAuth(lgProtocols))

	// Apply middleware in reverse order (last applied = first executed)
	var handler http.Handler = mux

	// Apply body limit middleware
	if cfg.Server.BodyLimit > 0 {
		handler = bodyLimitMiddleware(int64(cfg.Server.BodyLimit))(handler)
	}

	// Apply trusted proxy middleware
	if len(cfg.Server.TrustedProxies) > 0 {
		handler = trustedProxyMiddleware(cfg.Server.TrustedProxies)(handler)
	}

	// Apply server header middleware
	handler = serverHeaderMiddleware(SERVER_SIGNATURE)(handler)

	// Apply debug logging middleware
	handler = debugLoggingMiddleware(cfg.Server.Debug)(handler)

	return handler
}

func status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		sendJSONResponse(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	// Acquire read locks for both session and metric data
	sessionMutex.RLock()
	metricMutex.RLock()

	// Release locks in reverse order of acquisition
	defer metricMutex.RUnlock()
	defer sessionMutex.RUnlock()

	sendJSONResponse(w, 0, "OK", map[string]any{
		"sessions": localSessions,
		"metrics":  localMetrics,
	})
}

func manualSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		sendJSONResponse(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	go syncSessions()
	sendJSONResponse(w, 0, "Sync initiated", nil)
}

func nodePassthroughInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		sendJSONResponse(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	// Parse the request body
	var req NodePassthroughRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONResponse(w, http.StatusBadRequest, "Invalid request format", nil)
		return
	}

	switch req.Data.LinkType {
	case "wireguard":
		getWireGuardPassthroughInfo(w, &req)
	case "gre":
		getGREPassthroughInfo(w, &req, false)
	case "ip6gre":
		getGREPassthroughInfo(w, &req, true)
	default:
		sendJSONResponse(w, http.StatusBadRequest, "Link(Interface) type not supported", nil)
	}
}

func getWireGuardPassthroughInfo(w http.ResponseWriter, req *NodePassthroughRequest) {
	// Use existing session count from API to determine port allocation
	var port int

	if req.ExistingSessionCount == 0 {
		// First session for this ASN: use 2 + last 4 digits
		port = 20000 + int(req.ASN%10000)
	} else {
		// Subsequent sessions: use random port in 40000-49999 range
		var err error
		port, err = getRandomPortInRange("udp", 40000, 49999)
		if err != nil {
			sendJSONResponse(w, http.StatusInternalServerError, "Failed to get random port", nil)
			return
		}
	}

	// Create the passthrough data
	data := map[string]any{
		"asn":  req.ASN,
		"port": port,
	}

	// Create JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(data))
	tokenString, err := token.SignedString([]byte(cfg.PeerAPI.SessionPassthroughJwtSecert))
	if err != nil {
		sendJSONResponse(w, http.StatusInternalServerError, "Failed to create token", nil)
		return
	}

	// Create response with endpoint and WireGuard public key
	endpoint := cfg.WireGuard.LocalEndpointHost
	if strings.Contains(endpoint, ":") {
		endpoint = fmt.Sprintf("[%s]", endpoint)
	}

	sendJSONResponse(w, 0, "OK", map[string]string{
		"passthrough": tokenString,
		"info": fmt.Sprintf(
			"**Endpoint**: ```%s:%d```\n\n**WireGuard** Public Key: ```%s```",
			endpoint,
			port,
			strings.TrimSpace(cfg.WireGuard.PublicKey),
		),
	})
}

// getGREPassthroughInfo generates passthrough information for GRE tunnel sessions
// The isIPv6 parameter determines whether to use IPv6GRE (true) or IPv4GRE (false)
func getGREPassthroughInfo(w http.ResponseWriter, req *NodePassthroughRequest, isIPv6 bool) {
	// Create the passthrough data (no port required for GRE tunnels)
	data := map[string]any{
		"asn": req.ASN,
	}

	// Create JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(data))
	tokenString, err := token.SignedString([]byte(cfg.PeerAPI.SessionPassthroughJwtSecert))
	if err != nil {
		sendJSONResponse(w, http.StatusInternalServerError, "Failed to create token", nil)
		return
	}

	// Select the appropriate endpoint based on GRE type (IPv4 or IPv6)
	var endpoint string
	var tunnelType string
	if isIPv6 {
		endpoint = cfg.GRE.LocalEndpointDesc6
		tunnelType = "GRE over IPv6(ip6gre)"
	} else {
		endpoint = cfg.GRE.LocalEndpointDesc4
		tunnelType = "GRE over IPv4(gre)"
	}

	endpoint = strings.TrimSpace(endpoint)

	sendJSONResponse(w, 0, "OK", map[string]string{
		"passthrough": tokenString,
		"info": fmt.Sprintf(
			"- Keep in mind that GRE Tunnels are not safe, as traffic is not going to be encrypted\n- You can create only 1 session with the same endpoint\n- You must use IP instead of hostname for endpoint\n\n**Endpoint**: ```%s```\n\n**Tunnel Type**: ```%s``` , **TTL**: ```255```",
			endpoint,
			tunnelType,
		),
	})
}

// lgProtocols returns BIRD protocol status for Looking Glass
func lgProtocols(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		sendJSONResponse(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	output, err := birdPool.ShowProtocols()
	if err != nil {
		sendJSONResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to query BIRD: %v", err), nil)
		return
	}

	sendJSONResponse(w, 0, "OK", map[string]string{
		"protocols": output,
	})
}
