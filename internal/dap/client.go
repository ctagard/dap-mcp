package dap

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/go-dap"
)

// StoppedInfo contains information about why the debugger stopped
type StoppedInfo struct {
	Reason      string
	ThreadID    int
	Description string
	AllStopped  bool
}

// Client provides a high-level API for DAP operations
type Client struct {
	transport *Transport

	// Response handling
	pendingRequests map[int]chan dap.Message
	mu              sync.Mutex

	// Event handling
	eventHandler func(dap.Message)

	// Capabilities from initialize response
	capabilities dap.Capabilities

	// Initialization synchronization
	initialized     chan struct{}
	initializedOnce sync.Once

	// Stopped event handling
	stoppedChan chan *StoppedInfo
	stoppedMu   sync.Mutex

	// Context for shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewClient creates a new DAP client with the given transport
func NewClient(transport *Transport) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{
		transport:       transport,
		pendingRequests: make(map[int]chan dap.Message),
		initialized:     make(chan struct{}),
		ctx:             ctx,
		cancel:          cancel,
	}

	// Start the message reader goroutine
	c.wg.Add(1)
	go c.readLoop()

	return c
}

// SetEventHandler sets the handler for DAP events
func (c *Client) SetEventHandler(handler func(dap.Message)) {
	c.eventHandler = handler
}

// readLoop continuously reads messages from the transport
func (c *Client) readLoop() {
	defer c.wg.Done()

	consecutiveErrors := 0
	const maxConsecutiveErrors = 5

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		msg, err := c.transport.Receive()
		if err != nil {
			// Check if we're shutting down
			select {
			case <-c.ctx.Done():
				return
			default:
				consecutiveErrors++
				// Log the error for debugging, but continue to handle transient issues
				log.Printf("DAP transport error (attempt %d/%d): %v", consecutiveErrors, maxConsecutiveErrors, err)

				// If we get too many consecutive errors, stop the read loop
				// This prevents infinite loops on persistent transport failures
				if consecutiveErrors >= maxConsecutiveErrors {
					log.Printf("DAP transport: too many consecutive errors, stopping read loop")
					return
				}
				continue
			}
		}

		// Reset error counter on successful read
		consecutiveErrors = 0
		c.handleMessage(msg)
	}
}

// handleMessage routes incoming messages to the appropriate handler
func (c *Client) handleMessage(msg dap.Message) {
	// Try to extract RequestSeq from response messages
	var requestSeq int
	var isResponse bool

	switch m := msg.(type) {
	case *dap.InitializeResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.LaunchResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.AttachResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.DisconnectResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.ConfigurationDoneResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.ThreadsResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.StackTraceResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.ScopesResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.VariablesResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.EvaluateResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.SetBreakpointsResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.SetFunctionBreakpointsResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.ContinueResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.NextResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.StepInResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.StepOutResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.PauseResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.SetVariableResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.SourceResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.ModulesResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.ErrorResponse:
		requestSeq, isResponse = m.RequestSeq, true
	case *dap.InitializedEvent:
		// Signal that we received the initialized event
		c.initializedOnce.Do(func() {
			close(c.initialized)
		})
		if c.eventHandler != nil {
			c.eventHandler(msg)
		}
		return
	case *dap.StoppedEvent:
		// Notify any waiters that we've stopped
		info := &StoppedInfo{
			Reason:      m.Body.Reason,
			ThreadID:    m.Body.ThreadId,
			Description: m.Body.Description,
			AllStopped:  m.Body.AllThreadsStopped,
		}
		c.stoppedMu.Lock()
		if c.stoppedChan != nil {
			select {
			case c.stoppedChan <- info:
			default:
				// Channel full, skip
			}
		}
		c.stoppedMu.Unlock()
		if c.eventHandler != nil {
			c.eventHandler(msg)
		}
		return
	}

	if isResponse {
		c.mu.Lock()
		if ch, ok := c.pendingRequests[requestSeq]; ok {
			ch <- msg
			delete(c.pendingRequests, requestSeq)
		}
		c.mu.Unlock()
		return
	}

	// Handle other events
	if c.eventHandler != nil {
		c.eventHandler(msg)
	}
}

// sendRequest sends a request and waits for the response
func (c *Client) sendRequest(req dap.RequestMessage, timeout time.Duration) (dap.Message, error) {
	seq := c.transport.NextSeq()

	// Set the sequence number on the request
	switch r := req.(type) {
	case *dap.InitializeRequest:
		r.Seq = seq
	case *dap.LaunchRequest:
		r.Seq = seq
	case *dap.AttachRequest:
		r.Seq = seq
	case *dap.DisconnectRequest:
		r.Seq = seq
	case *dap.ConfigurationDoneRequest:
		r.Seq = seq
	case *dap.ThreadsRequest:
		r.Seq = seq
	case *dap.StackTraceRequest:
		r.Seq = seq
	case *dap.ScopesRequest:
		r.Seq = seq
	case *dap.VariablesRequest:
		r.Seq = seq
	case *dap.EvaluateRequest:
		r.Seq = seq
	case *dap.SetBreakpointsRequest:
		r.Seq = seq
	case *dap.SetFunctionBreakpointsRequest:
		r.Seq = seq
	case *dap.ContinueRequest:
		r.Seq = seq
	case *dap.NextRequest:
		r.Seq = seq
	case *dap.StepInRequest:
		r.Seq = seq
	case *dap.StepOutRequest:
		r.Seq = seq
	case *dap.PauseRequest:
		r.Seq = seq
	case *dap.SetVariableRequest:
		r.Seq = seq
	case *dap.SourceRequest:
		r.Seq = seq
	case *dap.ModulesRequest:
		r.Seq = seq
	}

	// Create response channel
	respCh := make(chan dap.Message, 1)
	c.mu.Lock()
	c.pendingRequests[seq] = respCh
	c.mu.Unlock()

	// Send the request
	if err := c.transport.Send(req); err != nil {
		c.mu.Lock()
		delete(c.pendingRequests, seq)
		c.mu.Unlock()
		return nil, err
	}

	// Wait for response
	select {
	case resp := <-respCh:
		return resp, nil
	case <-time.After(timeout):
		c.mu.Lock()
		delete(c.pendingRequests, seq)
		c.mu.Unlock()
		return nil, fmt.Errorf("request timeout")
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	}
}

// Initialize sends the initialize request
func (c *Client) Initialize(clientID, clientName string) (*dap.InitializeResponse, error) {
	req := &dap.InitializeRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "initialize",
		},
		Arguments: dap.InitializeRequestArguments{
			ClientID:                     clientID,
			ClientName:                   clientName,
			AdapterID:                    "dap-mcp",
			Locale:                       "en-US",
			LinesStartAt1:                true,
			ColumnsStartAt1:              true,
			PathFormat:                   "path",
			SupportsVariableType:         true,
			SupportsVariablePaging:       true,
			SupportsRunInTerminalRequest: false,
		},
	}

	resp, err := c.sendRequest(req, 10*time.Second)
	if err != nil {
		return nil, err
	}

	initResp, ok := resp.(*dap.InitializeResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", resp)
	}

	if !initResp.Success {
		return nil, fmt.Errorf("initialize failed: %s", initResp.Message)
	}

	c.capabilities = initResp.Body

	return initResp, nil
}

// WaitInitialized waits for the initialized event with a timeout
func (c *Client) WaitInitialized(timeout time.Duration) error {
	select {
	case <-c.initialized:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for initialized event")
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

// Launch sends a launch request
// Note: After calling Launch, caller should wait for InitializedEvent, then call ConfigurationDone
// The launch response may not arrive until after ConfigurationDone is sent
func (c *Client) Launch(args map[string]interface{}) (*dap.LaunchResponse, error) {
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal launch args: %w", err)
	}

	req := &dap.LaunchRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "launch",
		},
		Arguments: argsJSON,
	}

	// Send the request but use a longer timeout since debugpy may not respond until after configurationDone
	resp, err := c.sendRequest(req, 30*time.Second)
	if err != nil {
		return nil, err
	}

	launchResp, ok := resp.(*dap.LaunchResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", resp)
	}

	if !launchResp.Success {
		return nil, fmt.Errorf("launch failed: %s", launchResp.Message)
	}

	return launchResp, nil
}

// LaunchAsync sends a launch request without waiting for response
// Returns a channel that will receive the response
func (c *Client) LaunchAsync(args map[string]interface{}) (chan dap.Message, error) {
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal launch args: %w", err)
	}

	seq := c.transport.NextSeq()

	req := &dap.LaunchRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request", Seq: seq},
			Command:         "launch",
		},
		Arguments: argsJSON,
	}

	// Create response channel
	respCh := make(chan dap.Message, 1)
	c.mu.Lock()
	c.pendingRequests[seq] = respCh
	c.mu.Unlock()

	// Send the request
	if err := c.transport.Send(req); err != nil {
		c.mu.Lock()
		delete(c.pendingRequests, seq)
		c.mu.Unlock()
		return nil, err
	}

	return respCh, nil
}

// WaitForLaunchResponse waits for the launch response on the channel
func (c *Client) WaitForLaunchResponse(respCh chan dap.Message, timeout time.Duration) (*dap.LaunchResponse, error) {
	select {
	case resp := <-respCh:
		launchResp, ok := resp.(*dap.LaunchResponse)
		if !ok {
			return nil, fmt.Errorf("unexpected response type: %T", resp)
		}
		if !launchResp.Success {
			return nil, fmt.Errorf("launch failed: %s", launchResp.Message)
		}
		return launchResp, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("launch response timeout")
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	}
}

// Attach sends an attach request
func (c *Client) Attach(args map[string]interface{}) (*dap.AttachResponse, error) {
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal attach args: %w", err)
	}

	req := &dap.AttachRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "attach",
		},
		Arguments: argsJSON,
	}

	resp, err := c.sendRequest(req, 30*time.Second)
	if err != nil {
		return nil, err
	}

	attachResp, ok := resp.(*dap.AttachResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", resp)
	}

	if !attachResp.Success {
		return nil, fmt.Errorf("attach failed: %s", attachResp.Message)
	}

	return attachResp, nil
}

// AttachAsync sends an attach request without waiting for response
// Returns a channel that will receive the response
func (c *Client) AttachAsync(args map[string]interface{}) (chan dap.Message, error) {
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal attach args: %w", err)
	}

	seq := c.transport.NextSeq()

	req := &dap.AttachRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request", Seq: seq},
			Command:         "attach",
		},
		Arguments: argsJSON,
	}

	// Create response channel
	respCh := make(chan dap.Message, 1)
	c.mu.Lock()
	c.pendingRequests[seq] = respCh
	c.mu.Unlock()

	// Send the request
	if err := c.transport.Send(req); err != nil {
		c.mu.Lock()
		delete(c.pendingRequests, seq)
		c.mu.Unlock()
		return nil, err
	}

	return respCh, nil
}

// WaitForAttachResponse waits for the attach response on the channel
func (c *Client) WaitForAttachResponse(respCh chan dap.Message, timeout time.Duration) (*dap.AttachResponse, error) {
	select {
	case resp := <-respCh:
		attachResp, ok := resp.(*dap.AttachResponse)
		if !ok {
			return nil, fmt.Errorf("unexpected response type: %T", resp)
		}
		if !attachResp.Success {
			return nil, fmt.Errorf("attach failed: %s", attachResp.Message)
		}
		return attachResp, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("attach response timeout")
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	}
}

// ConfigurationDone signals that configuration is complete
func (c *Client) ConfigurationDone() error {
	req := &dap.ConfigurationDoneRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "configurationDone",
		},
	}

	resp, err := c.sendRequest(req, 10*time.Second)
	if err != nil {
		return err
	}

	configResp, ok := resp.(*dap.ConfigurationDoneResponse)
	if !ok {
		return fmt.Errorf("unexpected response type: %T", resp)
	}

	if !configResp.Success {
		return fmt.Errorf("configurationDone failed: %s", configResp.Message)
	}

	return nil
}

// Disconnect ends the debug session
func (c *Client) Disconnect(terminateDebuggee bool) error {
	req := &dap.DisconnectRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "disconnect",
		},
		Arguments: &dap.DisconnectArguments{
			TerminateDebuggee: terminateDebuggee,
		},
	}

	resp, err := c.sendRequest(req, 10*time.Second)
	if err != nil {
		return err
	}

	disconnectResp, ok := resp.(*dap.DisconnectResponse)
	if !ok {
		return fmt.Errorf("unexpected response type: %T", resp)
	}

	if !disconnectResp.Success {
		return fmt.Errorf("disconnect failed: %s", disconnectResp.Message)
	}

	return nil
}

// Threads gets all threads
func (c *Client) Threads() ([]dap.Thread, error) {
	req := &dap.ThreadsRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "threads",
		},
	}

	resp, err := c.sendRequest(req, 10*time.Second)
	if err != nil {
		return nil, err
	}

	threadsResp, ok := resp.(*dap.ThreadsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", resp)
	}

	if !threadsResp.Success {
		return nil, fmt.Errorf("threads request failed: %s", threadsResp.Message)
	}

	return threadsResp.Body.Threads, nil
}

// StackTrace gets the stack trace for a thread
func (c *Client) StackTrace(threadID, startFrame, levels int) ([]dap.StackFrame, int, error) {
	req := &dap.StackTraceRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "stackTrace",
		},
		Arguments: dap.StackTraceArguments{
			ThreadId:   threadID,
			StartFrame: startFrame,
			Levels:     levels,
		},
	}

	resp, err := c.sendRequest(req, 10*time.Second)
	if err != nil {
		return nil, 0, err
	}

	stackResp, ok := resp.(*dap.StackTraceResponse)
	if !ok {
		return nil, 0, fmt.Errorf("unexpected response type: %T", resp)
	}

	if !stackResp.Success {
		return nil, 0, fmt.Errorf("stackTrace request failed: %s", stackResp.Message)
	}

	return stackResp.Body.StackFrames, stackResp.Body.TotalFrames, nil
}

// Scopes gets the scopes for a stack frame
func (c *Client) Scopes(frameID int) ([]dap.Scope, error) {
	req := &dap.ScopesRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "scopes",
		},
		Arguments: dap.ScopesArguments{
			FrameId: frameID,
		},
	}

	resp, err := c.sendRequest(req, 10*time.Second)
	if err != nil {
		return nil, err
	}

	scopesResp, ok := resp.(*dap.ScopesResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", resp)
	}

	if !scopesResp.Success {
		return nil, fmt.Errorf("scopes request failed: %s", scopesResp.Message)
	}

	return scopesResp.Body.Scopes, nil
}

// Variables gets variables for a reference
func (c *Client) Variables(variablesRef int, filter string, start, count int) ([]dap.Variable, error) {
	args := dap.VariablesArguments{
		VariablesReference: variablesRef,
	}
	if filter != "" {
		args.Filter = filter
	}
	if start > 0 {
		args.Start = start
	}
	if count > 0 {
		args.Count = count
	}

	req := &dap.VariablesRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "variables",
		},
		Arguments: args,
	}

	resp, err := c.sendRequest(req, 10*time.Second)
	if err != nil {
		return nil, err
	}

	varsResp, ok := resp.(*dap.VariablesResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", resp)
	}

	if !varsResp.Success {
		return nil, fmt.Errorf("variables request failed: %s", varsResp.Message)
	}

	return varsResp.Body.Variables, nil
}

// Evaluate evaluates an expression
func (c *Client) Evaluate(expression string, frameID int, context string) (*dap.EvaluateResponseBody, error) {
	req := &dap.EvaluateRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "evaluate",
		},
		Arguments: dap.EvaluateArguments{
			Expression: expression,
			FrameId:    frameID,
			Context:    context,
		},
	}

	resp, err := c.sendRequest(req, 10*time.Second)
	if err != nil {
		return nil, err
	}

	evalResp, ok := resp.(*dap.EvaluateResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", resp)
	}

	if !evalResp.Success {
		return nil, fmt.Errorf("evaluate failed: %s", evalResp.Message)
	}

	return &evalResp.Body, nil
}

// SetBreakpoints sets breakpoints in a source file
func (c *Client) SetBreakpoints(source dap.Source, breakpoints []dap.SourceBreakpoint) ([]dap.Breakpoint, error) {
	req := &dap.SetBreakpointsRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "setBreakpoints",
		},
		Arguments: dap.SetBreakpointsArguments{
			Source:      source,
			Breakpoints: breakpoints,
		},
	}

	resp, err := c.sendRequest(req, 10*time.Second)
	if err != nil {
		return nil, err
	}

	bpResp, ok := resp.(*dap.SetBreakpointsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", resp)
	}

	if !bpResp.Success {
		return nil, fmt.Errorf("setBreakpoints failed: %s", bpResp.Message)
	}

	return bpResp.Body.Breakpoints, nil
}

// SetFunctionBreakpoints sets function breakpoints
func (c *Client) SetFunctionBreakpoints(breakpoints []dap.FunctionBreakpoint) ([]dap.Breakpoint, error) {
	req := &dap.SetFunctionBreakpointsRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "setFunctionBreakpoints",
		},
		Arguments: dap.SetFunctionBreakpointsArguments{
			Breakpoints: breakpoints,
		},
	}

	resp, err := c.sendRequest(req, 10*time.Second)
	if err != nil {
		return nil, err
	}

	bpResp, ok := resp.(*dap.SetFunctionBreakpointsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", resp)
	}

	if !bpResp.Success {
		return nil, fmt.Errorf("setFunctionBreakpoints failed: %s", bpResp.Message)
	}

	return bpResp.Body.Breakpoints, nil
}

// Continue continues execution
func (c *Client) Continue(threadID int) (bool, error) {
	req := &dap.ContinueRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "continue",
		},
		Arguments: dap.ContinueArguments{
			ThreadId: threadID,
		},
	}

	resp, err := c.sendRequest(req, 10*time.Second)
	if err != nil {
		return false, err
	}

	contResp, ok := resp.(*dap.ContinueResponse)
	if !ok {
		return false, fmt.Errorf("unexpected response type: %T", resp)
	}

	if !contResp.Success {
		return false, fmt.Errorf("continue failed: %s", contResp.Message)
	}

	return contResp.Body.AllThreadsContinued, nil
}

// Next steps over
func (c *Client) Next(threadID int) error {
	req := &dap.NextRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "next",
		},
		Arguments: dap.NextArguments{
			ThreadId: threadID,
		},
	}

	resp, err := c.sendRequest(req, 10*time.Second)
	if err != nil {
		return err
	}

	nextResp, ok := resp.(*dap.NextResponse)
	if !ok {
		return fmt.Errorf("unexpected response type: %T", resp)
	}

	if !nextResp.Success {
		return fmt.Errorf("next failed: %s", nextResp.Message)
	}

	return nil
}

// StepIn steps into
func (c *Client) StepIn(threadID int) error {
	req := &dap.StepInRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "stepIn",
		},
		Arguments: dap.StepInArguments{
			ThreadId: threadID,
		},
	}

	resp, err := c.sendRequest(req, 10*time.Second)
	if err != nil {
		return err
	}

	stepResp, ok := resp.(*dap.StepInResponse)
	if !ok {
		return fmt.Errorf("unexpected response type: %T", resp)
	}

	if !stepResp.Success {
		return fmt.Errorf("stepIn failed: %s", stepResp.Message)
	}

	return nil
}

// StepOut steps out
func (c *Client) StepOut(threadID int) error {
	req := &dap.StepOutRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "stepOut",
		},
		Arguments: dap.StepOutArguments{
			ThreadId: threadID,
		},
	}

	resp, err := c.sendRequest(req, 10*time.Second)
	if err != nil {
		return err
	}

	stepResp, ok := resp.(*dap.StepOutResponse)
	if !ok {
		return fmt.Errorf("unexpected response type: %T", resp)
	}

	if !stepResp.Success {
		return fmt.Errorf("stepOut failed: %s", stepResp.Message)
	}

	return nil
}

// Pause pauses execution
func (c *Client) Pause(threadID int) error {
	req := &dap.PauseRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "pause",
		},
		Arguments: dap.PauseArguments{
			ThreadId: threadID,
		},
	}

	resp, err := c.sendRequest(req, 10*time.Second)
	if err != nil {
		return err
	}

	pauseResp, ok := resp.(*dap.PauseResponse)
	if !ok {
		return fmt.Errorf("unexpected response type: %T", resp)
	}

	if !pauseResp.Success {
		return fmt.Errorf("pause failed: %s", pauseResp.Message)
	}

	return nil
}

// SetVariable sets a variable value
func (c *Client) SetVariable(variablesRef int, name, value string) (*dap.SetVariableResponseBody, error) {
	req := &dap.SetVariableRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "setVariable",
		},
		Arguments: dap.SetVariableArguments{
			VariablesReference: variablesRef,
			Name:               name,
			Value:              value,
		},
	}

	resp, err := c.sendRequest(req, 10*time.Second)
	if err != nil {
		return nil, err
	}

	setResp, ok := resp.(*dap.SetVariableResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", resp)
	}

	if !setResp.Success {
		return nil, fmt.Errorf("setVariable failed: %s", setResp.Message)
	}

	return &setResp.Body, nil
}

// Source gets source code
func (c *Client) Source(sourceRef int, path string) (string, string, error) {
	req := &dap.SourceRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "source",
		},
		Arguments: dap.SourceArguments{
			Source: &dap.Source{
				Path:            path,
				SourceReference: sourceRef,
			},
			SourceReference: sourceRef,
		},
	}

	resp, err := c.sendRequest(req, 10*time.Second)
	if err != nil {
		return "", "", err
	}

	sourceResp, ok := resp.(*dap.SourceResponse)
	if !ok {
		return "", "", fmt.Errorf("unexpected response type: %T", resp)
	}

	if !sourceResp.Success {
		return "", "", fmt.Errorf("source request failed: %s", sourceResp.Message)
	}

	return sourceResp.Body.Content, sourceResp.Body.MimeType, nil
}

// Modules gets loaded modules
func (c *Client) Modules(startModule, moduleCount int) ([]dap.Module, int, error) {
	req := &dap.ModulesRequest{
		Request: dap.Request{
			ProtocolMessage: dap.ProtocolMessage{Type: "request"},
			Command:         "modules",
		},
		Arguments: dap.ModulesArguments{
			StartModule: startModule,
			ModuleCount: moduleCount,
		},
	}

	resp, err := c.sendRequest(req, 10*time.Second)
	if err != nil {
		return nil, 0, err
	}

	modulesResp, ok := resp.(*dap.ModulesResponse)
	if !ok {
		return nil, 0, fmt.Errorf("unexpected response type: %T", resp)
	}

	if !modulesResp.Success {
		return nil, 0, fmt.Errorf("modules request failed: %s", modulesResp.Message)
	}

	return modulesResp.Body.Modules, modulesResp.Body.TotalModules, nil
}

// Capabilities returns the capabilities from the initialize response
func (c *Client) Capabilities() dap.Capabilities {
	return c.capabilities
}

// WaitForStopped waits for the debugger to stop (hit breakpoint, step complete, etc.)
func (c *Client) WaitForStopped(timeout time.Duration) (*StoppedInfo, error) {
	// Create channel to receive stopped event
	stoppedCh := make(chan *StoppedInfo, 1)

	c.stoppedMu.Lock()
	c.stoppedChan = stoppedCh
	c.stoppedMu.Unlock()

	defer func() {
		c.stoppedMu.Lock()
		c.stoppedChan = nil
		c.stoppedMu.Unlock()
	}()

	select {
	case info := <-stoppedCh:
		return info, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for stopped event")
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	}
}

// ContinueAndWait continues execution and waits for the program to stop
func (c *Client) ContinueAndWait(threadID int, timeout time.Duration) (*StoppedInfo, error) {
	// Set up to receive stopped event before continuing
	stoppedCh := make(chan *StoppedInfo, 1)

	c.stoppedMu.Lock()
	c.stoppedChan = stoppedCh
	c.stoppedMu.Unlock()

	defer func() {
		c.stoppedMu.Lock()
		c.stoppedChan = nil
		c.stoppedMu.Unlock()
	}()

	// Send continue request
	_, err := c.Continue(threadID)
	if err != nil {
		return nil, err
	}

	// Wait for stopped event
	select {
	case info := <-stoppedCh:
		return info, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for stopped event after continue")
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	}
}

// Close shuts down the client
func (c *Client) Close() error {
	c.cancel()
	c.wg.Wait()
	return c.transport.Close()
}
