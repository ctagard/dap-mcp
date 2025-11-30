// Package dap implements a client for the Debug Adapter Protocol (DAP).
//
// DAP is a protocol used to communicate between a development tool (like an IDE)
// and a debugger. This package provides:
//   - Transport: Low-level message sending/receiving over TCP or stdio
//   - Client: High-level DAP operations (Initialize, Launch, Attach, SetBreakpoints, etc.)
//   - SessionManager: Manages multiple concurrent debug sessions with lifecycle management
//
// The protocol is described at: https://microsoft.github.io/debug-adapter-protocol/
package dap

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/google/go-dap"
)

// Transport handles communication with a DAP server
type Transport struct {
	conn   io.ReadWriteCloser
	reader *bufio.Reader
	writer *bufio.Writer
	mu     sync.Mutex
	seq    int
}

// NewTCPTransport creates a transport connected to a TCP address
func NewTCPTransport(address string) (*Transport, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to DAP server at %s: %w", address, err)
	}

	return &Transport{
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
		seq:    1,
	}, nil
}

// NewStdioTransport creates a transport using stdio streams
func NewStdioTransport(stdin io.WriteCloser, stdout io.ReadCloser) *Transport {
	// Create a combined ReadWriteCloser
	rwc := &stdioRWC{
		reader: stdout,
		writer: stdin,
	}

	return &Transport{
		conn:   rwc,
		reader: bufio.NewReader(stdout),
		writer: bufio.NewWriter(stdin),
		seq:    1,
	}
}

type stdioRWC struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

func (s *stdioRWC) Read(p []byte) (n int, err error) {
	return s.reader.Read(p)
}

func (s *stdioRWC) Write(p []byte) (n int, err error) {
	return s.writer.Write(p)
}

func (s *stdioRWC) Close() error {
	err1 := s.reader.Close()
	err2 := s.writer.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

// NextSeq returns the next sequence number
func (t *Transport) NextSeq() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	seq := t.seq
	t.seq++
	return seq
}

// Send sends a DAP message
func (t *Transport) Send(msg dap.Message) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := dap.WriteProtocolMessage(t.writer, msg); err != nil {
		return fmt.Errorf("failed to write DAP message: %w", err)
	}

	if err := t.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush DAP message: %w", err)
	}

	return nil
}

// Receive receives a DAP message
func (t *Transport) Receive() (dap.Message, error) {
	msg, err := dap.ReadProtocolMessage(t.reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read DAP message: %w", err)
	}
	return msg, nil
}

// Close closes the transport
func (t *Transport) Close() error {
	return t.conn.Close()
}
