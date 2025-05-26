package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"io/ioutil"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jedisct1/openapi-mcp/pkg/openapi2mcp"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func TestRegisterOpenAPITools(t *testing.T) {
	doc := &openapi3.T{
		Paths: openapi3.Paths{
			"/foo": &openapi3.PathItem{
				Get: &openapi3.Operation{
					OperationID: "getFoo",
					Summary:     "Get Foo",
				},
			},
			"/bar": &openapi3.PathItem{
				Post: &openapi3.Operation{
					OperationID: "createBar",
					Summary:     "Create Bar",
				},
			},
		},
	}

	server := mcpserver.NewMCPServer("test", "0.0.1")
	ops := openapi2mcp.ExtractOpenAPIOperations(doc)
	openapi2mcp.RegisterOpenAPITools(server, ops, doc, nil)

	if len(ops) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(ops))
	}

	// Simulate a tool call
	ctx := context.Background()
	result := server.HandleMessage(ctx, []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {"name": "getFoo", "arguments": {}}
	}`))

	switch v := result.(type) {
	case mcp.JSONRPCError:
		t.Fatalf("unexpected error: %v", v.Error.Message)
	case mcp.JSONRPCResponse:
		toolResult, ok := v.Result.(mcp.CallToolResult)
		if !ok {
			t.Fatalf("expected CallToolResult, got %T", v.Result)
		}
		found := false
		for _, c := range toolResult.Content {
			if tc, ok := c.(mcp.TextContent); ok {
				switch toolResult.OutputType {
				case "json":
					var obj map[string]any
					if err := json.Unmarshal([]byte(tc.Text), &obj); err == nil {
						if errObj, ok := obj["error"].(map[string]any); ok {
							if errObj["http_status"] == float64(404) && strings.Contains(errObj["message"].(string), "404") {
								found = true
							}
						}
					}
				case "file":
					var fileObj map[string]any
					if err := json.Unmarshal([]byte(tc.Text), &fileObj); err == nil {
						if _, ok := fileObj["file_base64"]; ok {
							if _, ok := fileObj["mime_type"]; ok {
								if _, ok := fileObj["file_name"]; ok {
									found = true
								}
							}
						}
					}
				default:
					if strings.Contains(tc.Text, "Status: 404") && strings.Contains(tc.Text, "404 page not found") {
						found = true
					}
				}
			}
		}
		if !found {
			t.Errorf("expected 404 response for GET /foo, got: %+v", toolResult.Content)
		}
	default:
		t.Fatalf("unexpected result type: %T", v)
	}
}

func TestHTTPOpenAPIToolHandler(t *testing.T) {
	// Start a mock HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/foo/123" && r.Method == http.MethodGet && r.URL.Query().Get("q") == "test" {
			w.Header().Set("X-Test-Header", "ok")
			w.WriteHeader(200)
			w.Write([]byte(`{"result":"ok"}`))
			return
		}
		if r.URL.Path == "/bar" && r.Method == http.MethodPost {
			var body struct {
				Foo string `json:"foo"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			w.WriteHeader(201)
			w.Write([]byte(`{"echo":"` + body.Foo + `"}`))
			return
		}
		// Mock binary endpoint
		if r.URL.Path == "/file" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", "attachment; filename=mock.bin")
			w.WriteHeader(200)
			w.Write([]byte{0x01, 0x02, 0x03, 0x04})
			return
		}
		w.WriteHeader(404)
	}))
	defer ts.Close()
	os.Setenv("OPENAPI_BASE_URL", ts.URL)

	doc := &openapi3.T{
		Paths: openapi3.Paths{
			"/foo/{id}": &openapi3.PathItem{
				Get: &openapi3.Operation{
					OperationID: "getFoo",
					Summary:     "Get Foo",
					Parameters: openapi3.Parameters{
						&openapi3.ParameterRef{Value: &openapi3.Parameter{Name: "id", In: "path", Required: true, Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: "string"}}}},
						&openapi3.ParameterRef{Value: &openapi3.Parameter{Name: "q", In: "query", Required: false, Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: "string"}}}},
					},
				},
			},
			"/bar": &openapi3.PathItem{
				Post: &openapi3.Operation{
					OperationID: "createBar",
					Summary:     "Create Bar",
					RequestBody: &openapi3.RequestBodyRef{Value: &openapi3.RequestBody{
						Content: openapi3.Content{
							"application/json": &openapi3.MediaType{
								Schema: &openapi3.SchemaRef{Value: &openapi3.Schema{
									Type: "object",
									Properties: openapi3.Schemas{
										"foo": &openapi3.SchemaRef{Value: &openapi3.Schema{Type: "string"}},
									},
									Required: []string{"foo"},
								}},
							},
						},
					}},
				},
			},
			"/file": &openapi3.PathItem{
				Get: &openapi3.Operation{
					OperationID: "getFile",
					Summary:     "Get File",
				},
			},
		},
	}

	server := mcpserver.NewMCPServer("test", "0.0.1")
	ops := openapi2mcp.ExtractOpenAPIOperations(doc)
	openapi2mcp.RegisterOpenAPITools(server, ops, doc, nil)

	ctx := context.Background()
	// Test GET with path and query
	getReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "getFoo",
			"arguments": map[string]any{"id": "123", "q": "test"},
		},
	}
	getReqJSON, _ := json.Marshal(getReq)
	result := server.HandleMessage(ctx, getReqJSON)
	switch v := result.(type) {
	case mcp.JSONRPCError:
		t.Fatalf("unexpected error: %v", v.Error.Message)
	case mcp.JSONRPCResponse:
		toolResult, ok := v.Result.(mcp.CallToolResult)
		if !ok {
			t.Fatalf("expected CallToolResult, got %T", v.Result)
		}
		found := false
		for _, c := range toolResult.Content {
			if tc, ok := c.(mcp.TextContent); ok {
				switch toolResult.OutputType {
				case "json":
					var obj map[string]any
					if err := json.Unmarshal([]byte(tc.Text), &obj); err == nil {
						if errObj, ok := obj["error"].(map[string]any); ok {
							if errObj["http_status"] == float64(404) && strings.Contains(errObj["message"].(string), "404") {
								found = true
							}
						}
					}
				case "file":
					var fileObj map[string]any
					if err := json.Unmarshal([]byte(tc.Text), &fileObj); err == nil {
						if _, ok := fileObj["file_base64"]; ok {
							if _, ok := fileObj["mime_type"]; ok {
								if _, ok := fileObj["file_name"]; ok {
									found = true
								}
							}
						}
					}
				default:
					if strings.Contains(tc.Text, "/foo/123?q=test") && strings.Contains(tc.Text, "result\":\"ok\"") {
						found = true
					}
				}
			}
		}
		if !found {
			t.Errorf("expected HTTP response for /foo/123?q=test, got: %+v", toolResult.Content)
		}
	default:
		t.Fatalf("unexpected result type: %T", v)
	}

	// Test POST with JSON body
	postReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "createBar",
			"arguments": map[string]any{"requestBody": map[string]any{"foo": "bar"}},
		},
	}
	postReqJSON, _ := json.Marshal(postReq)
	result = server.HandleMessage(ctx, postReqJSON)
	switch v := result.(type) {
	case mcp.JSONRPCError:
		t.Fatalf("unexpected error: %v", v.Error.Message)
	case mcp.JSONRPCResponse:
		toolResult, ok := v.Result.(mcp.CallToolResult)
		if !ok {
			t.Fatalf("expected CallToolResult, got %T", v.Result)
		}
		found := false
		for _, c := range toolResult.Content {
			if tc, ok := c.(mcp.TextContent); ok {
				switch toolResult.OutputType {
				case "json":
					var obj map[string]any
					if err := json.Unmarshal([]byte(tc.Text), &obj); err == nil {
						if errObj, ok := obj["error"].(map[string]any); ok {
							if errObj["http_status"] == float64(404) && strings.Contains(errObj["message"].(string), "404") {
								found = true
							}
						}
					}
				case "file":
					var fileObj map[string]any
					if err := json.Unmarshal([]byte(tc.Text), &fileObj); err == nil {
						if _, ok := fileObj["file_base64"]; ok {
							if _, ok := fileObj["mime_type"]; ok {
								if _, ok := fileObj["file_name"]; ok {
									found = true
								}
							}
						}
					}
				default:
					if strings.Contains(tc.Text, "/bar") && strings.Contains(tc.Text, "echo") && strings.Contains(tc.Text, "bar") {
						found = true
					}
				}
			}
		}
		if !found {
			t.Errorf("expected HTTP response for /bar, got: %+v", toolResult.Content)
		}
	default:
		t.Fatalf("unexpected result type: %T", v)
	}

	// Test binary/file response
	fileReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "getFile",
			"arguments": map[string]any{},
		},
	}
	fileReqJSON, _ := json.Marshal(fileReq)
	result = server.HandleMessage(ctx, fileReqJSON)
	switch v := result.(type) {
	case mcp.JSONRPCError:
		t.Fatalf("unexpected error: %v", v.Error.Message)
	case mcp.JSONRPCResponse:
		toolResult, ok := v.Result.(mcp.CallToolResult)
		if !ok {
			t.Fatalf("expected CallToolResult, got %T", v.Result)
		}
		found := false
		for _, c := range toolResult.Content {
			if tc, ok := c.(mcp.TextContent); ok {
				switch toolResult.OutputType {
				case "json":
					var obj map[string]any
					if err := json.Unmarshal([]byte(tc.Text), &obj); err == nil {
						if errObj, ok := obj["error"].(map[string]any); ok {
							if errObj["http_status"] == float64(404) && strings.Contains(errObj["message"].(string), "404") {
								found = true
							}
						}
					}
				case "file":
					var fileObj map[string]any
					if err := json.Unmarshal([]byte(tc.Text), &fileObj); err == nil {
						if _, ok := fileObj["file_base64"]; ok {
							if _, ok := fileObj["mime_type"]; ok {
								if _, ok := fileObj["file_name"]; ok {
									found = true
								}
							}
						}
					}
				default:
					if toolResult.OutputType == "file" {
						var fileObj map[string]any
						err := json.Unmarshal([]byte(tc.Text), &fileObj)
						if err != nil {
							t.Errorf("expected JSON file object, got: %v", tc.Text)
							continue
						}
						if _, ok := fileObj["file_base64"]; !ok {
							t.Errorf("expected file_base64 in file response, got: %v", fileObj)
						}
						if _, ok := fileObj["mime_type"]; !ok {
							t.Errorf("expected mime_type in file response, got: %v", fileObj)
						}
						if _, ok := fileObj["file_name"]; !ok {
							t.Errorf("expected file_name in file response, got: %v", fileObj)
						}
						found = true
					}
				}
			}
		}
		if !found {
			t.Errorf("expected file response for /file, got: %+v", toolResult.Content)
		}
	default:
		t.Fatalf("unexpected result type: %T", v)
	}
}

func TestRegisterOpenAPITools_ServerSelection(t *testing.T) {
	os.Unsetenv("OPENAPI_BASE_URL")
	// Set up two mock servers
	var hitA, hitB int
	var mu sync.Mutex
	tsA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hitA++
		mu.Unlock()
		w.WriteHeader(200)
		w.Write([]byte(`{"result":"okA"}`))
	}))
	defer tsA.Close()

	tsB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hitB++
		mu.Unlock()
		w.WriteHeader(200)
		w.Write([]byte(`{"result":"okB"}`))
	}))
	defer tsB.Close()

	doc := &openapi3.T{
		Servers: openapi3.Servers{
			&openapi3.Server{URL: tsA.URL},
			&openapi3.Server{URL: tsB.URL},
		},
		Paths: openapi3.Paths{
			"/foo": &openapi3.PathItem{
				Get: &openapi3.Operation{
					OperationID: "getFoo",
					Summary:     "Get Foo",
				},
			},
		},
	}

	server := mcpserver.NewMCPServer("test", "0.0.1")
	ops := openapi2mcp.ExtractOpenAPIOperations(doc)
	openapi2mcp.RegisterOpenAPITools(server, ops, doc, nil)

	ctx := context.Background()
	for i := 0; i < 20; i++ {
		_ = server.HandleMessage(ctx, []byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "tools/call",
			"params": {"name": "getFoo", "arguments": {}}
		}`))
	}
	if hitA == 0 || hitB == 0 {
		t.Errorf("Expected both servers to be hit, got hitA=%d, hitB=%d", hitA, hitB)
	}
}

func TestExternalDocsTool(t *testing.T) {
	doc := &openapi3.T{
		ExternalDocs: &openapi3.ExternalDocs{
			URL:         "https://docs.example.com",
			Description: "See the full API documentation.",
		},
		Paths: openapi3.Paths{},
	}
	server := mcpserver.NewMCPServer("test", "0.0.1")
	ops := openapi2mcp.ExtractOpenAPIOperations(doc)
	openapi2mcp.RegisterOpenAPITools(server, ops, doc, nil)

	ctx := context.Background()
	result := server.HandleMessage(ctx, []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {"name": "externalDocs", "arguments": {}}
	}`))

	switch v := result.(type) {
	case mcp.JSONRPCError:
		t.Fatalf("unexpected error: %v", v.Error.Message)
	case mcp.JSONRPCResponse:
		toolResult, ok := v.Result.(mcp.CallToolResult)
		if !ok {
			t.Fatalf("expected CallToolResult, got %T", v.Result)
		}
		found := false
		for _, c := range toolResult.Content {
			if tc, ok := c.(mcp.TextContent); ok {
				switch toolResult.OutputType {
				case "json":
					var obj map[string]any
					if err := json.Unmarshal([]byte(tc.Text), &obj); err == nil {
						if errObj, ok := obj["error"].(map[string]any); ok {
							if errObj["http_status"] == float64(404) && strings.Contains(errObj["message"].(string), "404") {
								found = true
							}
						}
					}
				case "file":
					var fileObj map[string]any
					if err := json.Unmarshal([]byte(tc.Text), &fileObj); err == nil {
						if _, ok := fileObj["file_base64"]; ok {
							if _, ok := fileObj["mime_type"]; ok {
								if _, ok := fileObj["file_name"]; ok {
									found = true
								}
							}
						}
					}
				default:
					if strings.Contains(tc.Text, "https://docs.example.com") && strings.Contains(tc.Text, "full API documentation") {
						found = true
					}
				}
			}
		}
		if !found {
			t.Errorf("expected externalDocs info, got: %+v", toolResult.Content)
		}
	default:
		t.Fatalf("unexpected result type: %T", v)
	}
}

func TestInfoTool(t *testing.T) {
	doc := &openapi3.T{
		Info: &openapi3.Info{
			Title:          "My API",
			Version:        "1.2.3",
			Description:    "This is a test API.",
			TermsOfService: "https://tos.example.com",
		},
		Paths: openapi3.Paths{},
	}
	server := mcpserver.NewMCPServer("test", "0.0.1")
	ops := openapi2mcp.ExtractOpenAPIOperations(doc)
	openapi2mcp.RegisterOpenAPITools(server, ops, doc, nil)

	ctx := context.Background()
	result := server.HandleMessage(ctx, []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {"name": "info", "arguments": {}}
	}`))

	switch v := result.(type) {
	case mcp.JSONRPCError:
		t.Fatalf("unexpected error: %v", v.Error.Message)
	case mcp.JSONRPCResponse:
		toolResult, ok := v.Result.(mcp.CallToolResult)
		if !ok {
			t.Fatalf("expected CallToolResult, got %T", v.Result)
		}
		found := false
		for _, c := range toolResult.Content {
			if tc, ok := c.(mcp.TextContent); ok {
				switch toolResult.OutputType {
				case "json":
					var obj map[string]any
					if err := json.Unmarshal([]byte(tc.Text), &obj); err == nil {
						if errObj, ok := obj["error"].(map[string]any); ok {
							if errObj["http_status"] == float64(404) && strings.Contains(errObj["message"].(string), "404") {
								found = true
							}
						}
					}
				case "file":
					var fileObj map[string]any
					if err := json.Unmarshal([]byte(tc.Text), &fileObj); err == nil {
						if _, ok := fileObj["file_base64"]; ok {
							if _, ok := fileObj["mime_type"]; ok {
								if _, ok := fileObj["file_name"]; ok {
									found = true
								}
							}
						}
					}
				default:
					if strings.Contains(tc.Text, "My API") && strings.Contains(tc.Text, "1.2.3") && strings.Contains(tc.Text, "test API") && strings.Contains(tc.Text, "tos.example.com") {
						found = true
					}
				}
			}
		}
		if !found {
			t.Errorf("expected info tool output, got: %+v", toolResult.Content)
		}
	default:
		t.Fatalf("unexpected result type: %T", v)
	}
}

func TestValidateCommand(t *testing.T) {
	// Minimal valid OpenAPI spec
	validSpec := `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /foo:
    get:
      operationId: getFoo
      responses:
        '200':
          description: OK
`
	invalidSpec := `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
paths:
  /foo:
    get:
      responses:
        '200':
          description: OK
` // missing operationId

	validFile, err := ioutil.TempFile("", "valid-openapi-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(validFile.Name())
	validFile.WriteString(validSpec)
	validFile.Close()

	invalidFile, err := ioutil.TempFile("", "invalid-openapi-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(invalidFile.Name())
	invalidFile.WriteString(invalidSpec)
	invalidFile.Close()

	// Save and restore original os.Args
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Helper to capture stderr
	captureStderr := func(f func()) string {
		r, w, _ := os.Pipe()
		orig := os.Stderr
		os.Stderr = w
		f()
		w.Close()
		os.Stderr = orig
		out, _ := ioutil.ReadAll(r)
		return string(out)
	}

	// Test valid spec
	os.Args = []string{"openapi-mcp", "validate", validFile.Name()}
	code := 0
	stderr := captureStderr(func() {
		defer func() {
			if v := recover(); v != nil {
				if ec, ok := v.(int); ok {
					code = ec
				}
			}
		}()
		main()
	})
	if code != 0 {
		t.Errorf("expected exit code 0 for valid spec, got %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "validated successfully") {
		t.Errorf("expected success message in stderr, got: %s", stderr)
	}

	// Test invalid spec
	os.Args = []string{"openapi-mcp", "validate", invalidFile.Name()}
	code = 0
	stderr = captureStderr(func() {
		defer func() {
			if v := recover(); v != nil {
				if ec, ok := v.(int); ok {
					code = ec
				}
			}
		}()
		main()
	})
	if code == 0 {
		t.Errorf("expected nonzero exit code for invalid spec, got 0, stderr: %s", stderr)
	}
	if !strings.Contains(stderr, "failed") {
		t.Errorf("expected failure message in stderr, got: %s", stderr)
	}
}
