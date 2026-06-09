package main

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// Middleware for body size limiting
func bodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if maxBytes > 0 {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Middleware for IP validation and trusted proxy handling
func trustedProxyMiddleware(trustedProxies []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get the real IP address considering trusted proxies
			clientIP := getRealIP(r, trustedProxies)

			// Enhanced IP validation (EnableIPValidation equivalent)
			if clientIP != "" {
				parsedIP := net.ParseIP(clientIP)
				if parsedIP == nil {
					sendJSONResponse(w, http.StatusBadRequest, "Invalid IP address format", nil)
					return
				}

				// Additional IP validation - reject unspecified IPs
				if parsedIP.IsUnspecified() {
					sendJSONResponse(w, http.StatusBadRequest, "Unspecified IP address not allowed", nil)
					return
				}
			}

			// Store the real IP in request context for later use
			r.Header.Set("X-Real-IP", clientIP)
			next.ServeHTTP(w, r)
		})
	}
}

// Get real IP address considering trusted proxies
func getRealIP(r *http.Request, trustedProxies []string) string {
	remoteAddr := r.RemoteAddr
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		remoteAddr = host
	}

	// Check if the request comes from a trusted proxy
	if isTrustedProxy(remoteAddr, trustedProxies) {
		// Check X-Forwarded-For header
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ips := strings.Split(forwarded, ",")
			if len(ips) > 0 {
				return strings.TrimSpace(ips[0])
			}
		}

		// Check X-Real-IP header
		if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			return strings.TrimSpace(realIP)
		}
	}

	return remoteAddr
}

// Check if IP is in trusted proxy list
func isTrustedProxy(ip string, trustedProxies []string) bool {
	clientIP := net.ParseIP(ip)
	if clientIP == nil {
		return false
	}

	for _, trusted := range trustedProxies {
		if strings.Contains(trusted, "/") {
			// CIDR notation
			_, network, err := net.ParseCIDR(trusted)
			if err == nil && network.Contains(clientIP) {
				return true
			}
		} else {
			// Direct IP match
			if ip == trusted {
				return true
			}
		}
	}

	return false
}

// Middleware for adding server header
func serverHeaderMiddleware(serverHeader string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Server", serverHeader)
			next.ServeHTTP(w, r)
		})
	}
}

// Debug logging middleware
func debugLoggingMiddleware(debug bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if debug {
				clientIP := r.Header.Get("X-Real-IP")
				if clientIP == "" {
					clientIP = r.RemoteAddr
				}
				fmt.Printf("[DEBUG] %s %s %s from %s\n", r.Method, r.URL.Path, r.Proto, clientIP)
			}
			next.ServeHTTP(w, r)
		})
	}
}
