package bird

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
)

// BirdConn represents a connection to the BIRD routing daemon
type BirdConn struct {
	conn net.Conn
}

// Read reads output from the BIRD socket, removes preceding status numbers,
// and writes the result to the output buffer.
func (b *BirdConn) Read(outputBuffer io.Writer) {
	if b == nil || b.conn == nil {
		return
	}

	const (
		StatusSize  = 4
		NewlineChar = '\n'
	)
	reader := bufio.NewReader(b.conn)

	for {
		line, err := reader.ReadBytes(NewlineChar)
		if err != nil {
			// EOF or error
			break
		}

		lineLen := len(line)

		// Check if the line starts with a status number
		if lineLen > StatusSize && isNumeric(line[0]) && isNumeric(line[1]) && isNumeric(line[2]) && isNumeric(line[3]) {
			// Ensure there's content after the status number
			if outputBuffer != nil && lineLen > StatusSize+1 {
				outputBuffer.Write(line[StatusSize+1:])
			}
			// Status indicates no more lines could be read
			if line[0] == byte('0') || line[0] == byte('8') || line[0] == byte('9') {
				break
			}
		} else if outputBuffer != nil {
			// Removes starting space and output
			outputBuffer.Write(line[1:])
		}
	}
}

// Write writes a command string to the BIRD socket with a newline appended.
func (b *BirdConn) Write(s string) error {
	if b == nil {
		return fmt.Errorf("BirdConn is nil")
	}
	if b.conn == nil {
		return fmt.Errorf("connection is nil")
	}
	if _, err := b.conn.Write([]byte(s + "\n")); err != nil {
		return fmt.Errorf("failed to write command: %w", err)
	}
	return nil
}

// Close closes the BIRD socket connection.
func (b *BirdConn) Close() error {
	if b.conn == nil {
		return nil
	}
	if err := b.conn.Close(); err != nil {
		return fmt.Errorf("failed to close bird socket: %w", err)
	}
	b.conn = nil
	return nil
}

// Restrict sends the "restrict" command to the BIRD socket and checks for confirmation.
func (b *BirdConn) Restrict() (bool, error) {
	if err := b.Write("restrict"); err != nil {
		return false, fmt.Errorf("failed to send restrict command: %w", err)
	}

	var reply bytes.Buffer
	b.Read(&reply)

	return strings.Contains(reply.String(), "Access restricted"), nil
}

// isNumeric checks if a byte represents a numeric character.
func isNumeric(b byte) bool {
	return b >= byte('0') && b <= byte('9')
}

// Open establishes a connection to the BIRD socket and removes banner/startup messages.
func NewBirdConnection(unixPath string) (*BirdConn, error) {
	conn, err := net.Dial("unix", unixPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to bird socket: %w", err)
	}

	birdConn := &BirdConn{conn: conn}

	// Remove banner/startup messages by reading the first line
	birdConn.Read(nil)

	return birdConn, nil
}
