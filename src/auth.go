package main

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	bearerScheme = "Bearer\x20"
)

// verifyBearerToken verifies if the request has a valid bearer token
// The token should be a bcrypt hash of agentSecret+routerUuid
func verifyBearerToken(r *http.Request, token string) bool {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, bearerScheme) {
		return false
	}

	tokenStr := authHeader[len(bearerScheme):]

	// Check if it's our own token going outbound
	if subtle.ConstantTimeCompare([]byte(tokenStr), []byte(token)) == 1 {
		return true
	}

	// Otherwise verify the inbound bcrypt hash
	err := bcrypt.CompareHashAndPassword([]byte(tokenStr), []byte(cfg.PeerAPI.AgentSecret+cfg.PeerAPI.RouterUUID))
	return err == nil
}

// generateToken generates a bcrypt token for outbound requests
func generateToken() (string, error) {
	if cfg.PeerAPI.Secret == "" || cfg.PeerAPI.RouterUUID == "" {
		return "", errors.New("missing PeerAPI secret or router UUID in config")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.PeerAPI.Secret+cfg.PeerAPI.RouterUUID), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

// withAuth wraps handlers with bearer token authentication
func withAuth(handler func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !verifyBearerToken(r, cfg.PeerAPI.Secret) {
			sendJSONResponse(w, http.StatusUnauthorized, "Unauthorized", nil)
			return
		}
		handler(w, r)
	}
}
