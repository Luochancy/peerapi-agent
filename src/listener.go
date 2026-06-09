package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// createHTTPListener creates a listener with custom buffer sizes
// Supports both TCP and Unix socket listeners based on configuration
func createHTTPListener(listenerType, addr string) (net.Listener, error) {
	var listener net.Listener
	var err error

	// Normalize listener type to lowercase
	listenerType = strings.ToLower(listenerType)

	switch listenerType {
	case "tcp", "":
		// Default to TCP if not specified or explicitly set to tcp
		listener, err = net.Listen("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("failed to create TCP listener on %s: %w", addr, err)
		}
	case "unix":
		// For Unix socket, remove the socket file if it exists
		if err := os.RemoveAll(addr); err != nil {
			return nil, fmt.Errorf("failed to remove existing socket file %s: %w", addr, err)
		}

		listener, err = net.Listen("unix", addr)
		if err != nil {
			return nil, fmt.Errorf("failed to create Unix socket listener on %s: %w", addr, err)
		}

		// Set appropriate permissions for the socket file
		if err := os.Chmod(addr, 0666); err != nil {
			listener.Close()
			return nil, fmt.Errorf("failed to set permissions on socket file %s: %w", addr, err)
		}
	default:
		return nil, fmt.Errorf("unsupported listener type: %s (supported: tcp, unix)", listenerType)
	}

	// Wrap listener to apply buffer sizes to connections (only applies to TCP)
	return &customListener{
		Listener:        listener,
		listenerType:    listenerType,
		readBufferSize:  cfg.Server.ReadBufferSize,
		writeBufferSize: cfg.Server.WriteBufferSize,
	}, nil
}

// customListener wraps net.Listener to apply buffer configurations
type customListener struct {
	net.Listener
	listenerType    string
	readBufferSize  int
	writeBufferSize int
}

func (l *customListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	// Apply buffer sizes only for TCP connections
	if strings.ToLower(l.listenerType) == "tcp" {
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			if l.readBufferSize > 0 {
				tcpConn.SetReadBuffer(l.readBufferSize)
			}
			if l.writeBufferSize > 0 {
				tcpConn.SetWriteBuffer(l.writeBufferSize)
			}
		}
	}

	return conn, nil
}
