package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// JSONRPCRequest defines a generic JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse defines a generic JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string                    `json:"jsonrpc"`
	ID      int                       `json:"id"`
	Result  json.RawMessage           `json:"result,omitempty"`
	Error   *JSONRPCError             `json:"error,omitempty"`
}

// JSONRPCError defines the error format in JSON-RPC.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// JSONRPCNotification defines a notification without an ID.
type JSONRPCNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Transport defines the interface for sending and receiving JSON-RPC messages.
type Transport interface {
	Send(request JSONRPCRequest) (JSONRPCResponse, error)
	Close() error
}

// StdioTransport implements Transport using stdin/stdout of a subprocess.
type StdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	mu     sync.Mutex // Protects write operations if necessary, though request/response matching needs care
	
	// For reading responses asynchronously or handling notifications
	reader *bufio.Reader
	nextID int
}

// NewStdioTransport creates a new transport for the given command.
func NewStdioTransport(command string, args []string) (*StdioTransport, error) {
	cmd := exec.Command(command, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	// We usually want to capture stderr separately or ignore it to not block stdout reading
	// But for MCP, stderr is for logging, stdout is for transport.
	
	transport := &StdioTransport{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		reader: bufio.NewReader(stdout),
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return transport, nil
}

// Send marshals the request, writes it to stdin, and reads a response from stdout.
func (t *StdioTransport) Send(req JSONRPCRequest) (JSONRPCResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	req.JSONRPC = "2.0"
	req.ID = t.nextID
	t.nextID++

	data, err := json.Marshal(req)
	if err != nil {
		return JSONRPCResponse{}, err
	}

	// MCP stdio uses Content-Length headers like HTTP? 
	// Actually, MCP specification says: "The client and server MUST communicate over stdin and stdout using JSON-RPC 2.0... The transport MUST adhere to the HTTP-like content framing..."
	// Wait, strictly speaking, MCP stdio transport uses HTTP content framing (Content-Length: X\r\n\r\n{body}).
	
	// Let's implement the Content-Length framing correctly.
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := t.stdin.Write([]byte(header)); err != nil {
		return JSONRPCResponse{}, err
	}
	if _, err := t.stdin.Write(data); err != nil {
		return JSONRPCResponse{}, err
	}

	// Read headers
	contentLength := -1
	for {
		line, err := t.reader.ReadString('\n')
		if err != nil {
			return JSONRPCResponse{}, err
		}
		// Trim \r
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if line == "" {
			// End of headers
			break
		}
		
		if len(line) > 16 && line[:16] == "Content-Length: " {
			_, err := fmt.Sscanf(line[16:], "%d", &contentLength)
			if err != nil {
				return JSONRPCResponse{}, fmt.Errorf("invalid Content-Length header: %s", line)
			}
		}
	}

	if contentLength == -1 {
		return JSONRPCResponse{}, fmt.Errorf("missing Content-Length header")
	}

	// Read body
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(t.reader, body); err != nil {
		return JSONRPCResponse{}, err
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return JSONRPCResponse{}, err
	}

	return resp, nil
}

// Close terminates the subprocess.
func (t *StdioTransport) Close() error {
	t.stdin.Close()
	return t.cmd.Wait()
}
