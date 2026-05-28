package mcp

import (
	"encoding/json"
	"testing"
)

func TestServerDefStructure(t *testing.T) {
	def := ServerDef{
		Name:    "test-server",
		Command: "/usr/bin/test",
		Args:    []string{"--verbose"},
	}

	if def.Name != "test-server" {
		t.Errorf("expected name 'test-server', got %s", def.Name)
	}
	if def.Command != "/usr/bin/test" {
		t.Errorf("expected command '/usr/bin/test', got %s", def.Command)
	}
	if len(def.Args) != 1 || def.Args[0] != "--verbose" {
		t.Error("args not set correctly")
	}
}

func TestServerDefJSONMarshal(t *testing.T) {
	def := ServerDef{
		Name:    "my-server",
		Command: "/bin/server",
		Args:    []string{"--port", "8080"},
	}

	data, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed ServerDef
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed.Name != def.Name {
		t.Error("name mismatch after roundtrip")
	}
	if parsed.Command != def.Command {
		t.Error("command mismatch after roundtrip")
	}
}

func TestServerDefJSONOmitEmpty(t *testing.T) {
	// Note: omitempty is only on yaml tag, not json
	// So in JSON, args will be present even if empty
	
	def := ServerDef{
		Name:    "no-args",
		Command: "/bin/cmd",
	}

	data, err := json.Marshal(def)
	if err != nil {
		t.Fatal(err)
	}

	var parsed ServerDef
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed.Name != def.Name {
		t.Error("name mismatch")
	}
	// args will be null in JSON
	if parsed.Args != nil {
		t.Error("expected empty args")
	}
}

func TestMustMarshal(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "simple map",
			input:    map[string]string{"key": "value"},
			expected: `{"key":"value"}`,
		},
		{
			name:     "struct",
			input:    struct{ Name string }{Name: "test"},
			expected: `{"Name":"test"}`,
		},
		{
			name:     "number",
			input:    42,
			expected: `42`,
		},
		{
			name:     "string",
			input:    "hello",
			expected: `"hello"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mustMarshal(tt.input)
			if string(result) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(result))
			}
		})
	}
}

func TestJSONRPCRequestDefaults(t *testing.T) {
	req := JSONRPCRequest{
		Method: "test",
	}

	if req.JSONRPC != "" {
		t.Error("JSONRPC should default empty")
	}
	if req.ID != 0 {
		t.Error("ID should default to 0")
	}
	if len(req.Params) != 0 {
		t.Error("Params should default to empty")
	}
}

func TestJSONRPCRequestMarshal(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  mustMarshal(map[string]any{"version": "1.0"}),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	var parsed JSONRPCRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed.Method != "initialize" {
		t.Error("method mismatch")
	}
	if parsed.JSONRPC != "2.0" {
		t.Error("jsonrpc version mismatch")
	}
}

func TestJSONRPCResponseFields(t *testing.T) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  json.RawMessage(`{"tools": []}`),
		Error:   nil,
	}

	if resp.JSONRPC != "2.0" {
		t.Error("jsonrpc version mismatch")
	}
	if resp.ID != 1 {
		t.Error("ID mismatch")
	}
	if string(resp.Result) != `{"tools": []}` {
		t.Errorf("Result mismatch: %s", resp.Result)
	}
}

func TestJSONRPCResponseMarshal(t *testing.T) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  json.RawMessage(`{"success":true}`),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}

	var parsed JSONRPCResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed.JSONRPC != "2.0" {
		t.Error("jsonrpc mismatch")
	}
	if parsed.ID != 1 {
		t.Error("ID mismatch")
	}
	if string(parsed.Result) != `{"success":true}` {
		t.Errorf("Result mismatch: %s", parsed.Result)
	}
}

func TestJSONRPCErrorStructure(t *testing.T) {
	err := JSONRPCError{
		Code:    -32600,
		Message: "Invalid Request",
		Data:    map[string]string{"details": "invalid json"},
	}

	if err.Code != -32600 {
		t.Error("error code mismatch")
	}
	if err.Message != "Invalid Request" {
		t.Error("error message mismatch")
	}
}

func TestJSONRPCErrorMarshal(t *testing.T) {
	origErr := &JSONRPCError{
		Code:    -32603,
		Message: "Internal error",
	}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Error:   origErr,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}

	var parsed JSONRPCResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed.Error == nil {
		t.Fatal("expected error to be preserved")
	}
	if parsed.Error.Code != -32603 {
		t.Error("error code mismatch after roundtrip")
	}
	if parsed.Error.Message != "Internal error" {
		t.Error("error message mismatch after roundtrip")
	}
}

func TestJSONRPCNotificationStructure(t *testing.T) {
	notif := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  mustMarshal(map[string]any{}),
	}

	if notif.JSONRPC != "2.0" {
		t.Error("jsonrpc mismatch")
	}
	if notif.Method != "notifications/initialized" {
		t.Error("method mismatch")
	}
}

func TestJSONRPCNotificationMarshal(t *testing.T) {
	notif := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  "test/notification",
		Params:  mustMarshal(map[string]string{"key": "value"}),
	}

	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatal(err)
	}

	var parsed JSONRPCNotification
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed.Method != "test/notification" {
		t.Error("method mismatch after roundtrip")
	}
}

func TestJSONRPCNotificationNoID(t *testing.T) {
	// Notifications don't have ID field
	notif := JSONRPCNotification{Method: "test"}
	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}

	if _, exists := m["id"]; exists {
		t.Error("notifications should not have 'id' field")
	}
}

func TestClientStructure(t *testing.T) {
	client := &Client{
		ServerTools: nil,
	}

	if client.initialized != false {
		t.Error("expected initialized to be false")
	}
	if client.ServerTools != nil {
		t.Error("expected ServerTools to be nil")
	}
}
