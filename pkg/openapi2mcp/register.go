// register.go
package openapi2mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/xeipuuv/gojsonschema"
)

// RegisterOpenAPITools registers each OpenAPI operation as an MCP tool with a real HTTP handler.
// Also adds tools for externalDocs, info, and describe if present in the OpenAPI spec.
// The handler validates arguments, builds the HTTP request, and returns the HTTP response as the tool result.
// Returns the list of tool names registered.
func RegisterOpenAPITools(server *mcpserver.MCPServer, ops []OpenAPIOperation, doc *openapi3.T, opts *ToolGenOptions) []string {
	baseURLs := []string{}
	if os.Getenv("OPENAPI_BASE_URL") != "" {
		baseURLs = append(baseURLs, os.Getenv("OPENAPI_BASE_URL"))
	} else if doc.Servers != nil && len(doc.Servers) > 0 {
		for _, s := range doc.Servers {
			if s != nil && s.URL != "" {
				baseURLs = append(baseURLs, s.URL)
			}
		}
	} else {
		baseURLs = append(baseURLs, "http://localhost:8080")
	}

	// Extract API key header name from securitySchemes
	apiKeyHeader := "Fastly-Key" // default fallback
	if doc.Components != nil && doc.Components.SecuritySchemes != nil {
		if sec, ok := doc.Components.SecuritySchemes["ApiKeyAuth"]; ok && sec.Value != nil {
			if sec.Value.Type == "apiKey" && sec.Value.In == "header" && sec.Value.Name != "" {
				apiKeyHeader = sec.Value.Name
			}
		}
	}

	// Map from operationID to inputSchema JSON for validation
	toolSchemas := make(map[string][]byte)
	var toolNames []string
	var toolSummaries []map[string]any

	// Tag filtering
	filterByTag := func(op OpenAPIOperation) bool {
		if opts == nil || len(opts.TagFilter) == 0 {
			return true
		}
		for _, tag := range op.Tags {
			for _, want := range opts.TagFilter {
				if tag == want {
					return true
				}
			}
		}
		return false
	}

	for _, op := range ops {
		if !filterByTag(op) {
			continue
		}
		inputSchema := BuildInputSchema(op.Parameters, op.RequestBody)
		if opts != nil && opts.PostProcessSchema != nil {
			inputSchema = opts.PostProcessSchema(op.OperationID, inputSchema)
		}
		inputSchemaJSON, _ := json.MarshalIndent(inputSchema, "", "  ")
		desc := op.Description
		if desc == "" {
			desc = op.Summary
		}
		name := op.OperationID
		if opts != nil && opts.NameFormat != nil {
			name = opts.NameFormat(name)
		}
		annotations := mcp.ToolAnnotation{}
		var titleParts []string
		if opts != nil && opts.Version != "" {
			titleParts = append(titleParts, "OpenAPI "+opts.Version)
		}
		if len(op.Tags) > 0 {
			titleParts = append(titleParts, "Tags: "+strings.Join(op.Tags, ", "))
		}
		if len(titleParts) > 0 {
			annotations.Title = strings.Join(titleParts, " | ")
		}
		tool := mcp.NewToolWithRawSchema(name, desc, inputSchemaJSON)
		tool.Annotations = annotations
		toolSchemas[name] = inputSchemaJSON
		opCopy := op
		if opts != nil && opts.DryRun {
			// For dry run, collect summary info
			toolSummaries = append(toolSummaries, map[string]any{
				"name":        name,
				"description": desc,
				"tags":        op.Tags,
				"inputSchema": inputSchema,
			})
			toolNames = append(toolNames, name)
			continue
		}
		server.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			if args == nil {
				args = map[string]any{}
			}

			// Validate arguments against inputSchema
			inputSchemaJSON := toolSchemas[name]
			argsJSON, _ := json.Marshal(args)
			schemaLoader := gojsonschema.NewBytesLoader(inputSchemaJSON)
			argsLoader := gojsonschema.NewBytesLoader(argsJSON)
			result, err := gojsonschema.Validate(schemaLoader, argsLoader)
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.TextContent{
							Type: "text",
							Text: "Validation error: " + err.Error(),
						},
					},
					IsError: true,
				}, nil
			}
			if !result.Valid() {
				var missingFields []string
				var suggestions []string
				errMsgs := ""
				// Parse the input schema for property descriptions
				var schemaObj map[string]any
				_ = json.Unmarshal(inputSchemaJSON, &schemaObj)
				properties, _ := schemaObj["properties"].(map[string]any)
				for _, verr := range result.Errors() {
					errMsg := verr.String()
					// If missing required property, add description
					if verr.Type() == "required" {
						if missingRaw, ok := verr.Details()["property"]; ok {
							if missing, ok := missingRaw.(string); ok {
								missingFields = append(missingFields, missing)
								if prop, ok := properties[missing].(map[string]any); ok {
									desc, _ := prop["description"].(string)
									typeStr, _ := prop["type"].(string)
									info := ""
									if desc != "" {
										info = desc
									}
									if typeStr != "" {
										if info != "" {
											info += ", "
										}
										info += "type: " + typeStr
									}
									if info != "" {
										errMsg = "Missing required parameter: '" + missing + "' (" + info + "). Please provide this parameter."
									}
								}
							}
						}
					}
					errMsgs += errMsg + "\n"
				}
				// Suggest a retry with an example argument set
				exampleArgs := map[string]any{}
				for k, v := range properties {
					if prop, ok := v.(map[string]any); ok {
						typeStr, _ := prop["type"].(string)
						switch typeStr {
						case "string":
							exampleArgs[k] = "example"
						case "number", "integer":
							exampleArgs[k] = 123
						case "boolean":
							exampleArgs[k] = true
						case "array":
							exampleArgs[k] = []any{"item1", "item2"}
						case "object":
							exampleArgs[k] = map[string]any{"key": "value"}
						default:
							exampleArgs[k] = nil
						}
					} else {
						exampleArgs[k] = nil
					}
				}
				suggestionStr := "Try again with: call " + name + " "
				exampleJSON, _ := json.Marshal(exampleArgs)
				suggestionStr += string(exampleJSON)
				suggestions = append(suggestions, suggestionStr)
				errorObj := map[string]any{
					"error": map[string]any{
						"code":        "validation_failed",
						"message":     strings.TrimSpace(errMsgs),
						"missing":     missingFields,
						"suggestions": suggestions,
					},
				}
				apiResponse := map[string]any{"type": "api_response"}
				for k, v := range errorObj {
					apiResponse[k] = v
				}
				errorJSON, _ := json.MarshalIndent(apiResponse, "", "  ")
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.TextContent{
							Type: "json",
							Text: string(errorJSON),
						},
					},
					IsError:      true,
					Schema:       inputSchema,
					Arguments:    args,
					Examples:     []any{args},
					Usage:        "call <tool> <json-args>",
					NextSteps:    []string{"list", "schema <tool>"},
					OutputFormat: "structured",
					OutputType:   "json",
				}, nil
			}

			// Build URL path with path parameters
			path := opCopy.Path
			for _, paramRef := range opCopy.Parameters {
				if paramRef == nil || paramRef.Value == nil {
					continue
				}
				p := paramRef.Value
				if p.In == "path" {
					if val, ok := args[p.Name]; ok {
						path = strings.ReplaceAll(path, "{"+p.Name+"}", fmt.Sprintf("%v", val))
					}
				}
			}
			// Build query parameters
			query := url.Values{}
			for _, paramRef := range opCopy.Parameters {
				if paramRef == nil || paramRef.Value == nil {
					continue
				}
				p := paramRef.Value
				if p.In == "query" {
					if val, ok := args[p.Name]; ok {
						query.Set(p.Name, fmt.Sprintf("%v", val))
					}
				}
			}
			// Pick a random baseURL for each call using the global rand
			baseURL := baseURLs[rand.Intn(len(baseURLs))]
			fullURL := baseURL + path
			if len(query) > 0 {
				fullURL += "?" + query.Encode()
			}
			// Build request body if needed
			var body []byte
			if opCopy.RequestBody != nil && opCopy.RequestBody.Value != nil {
				if mt := opCopy.RequestBody.Value.Content.Get("application/json"); mt != nil && mt.Schema != nil && mt.Schema.Value != nil {
					if v, ok := args["requestBody"]; ok && v != nil {
						body, _ = json.Marshal(v)
					}
				}
			}
			// Build HTTP request
			method := strings.ToUpper(opCopy.Method)
			httpReq, err := http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(body))
			if err != nil {
				return nil, err
			}
			if len(body) > 0 {
				httpReq.Header.Set("Content-Type", "application/json")
			}
			// --- AUTH HANDLING: inject per-operation security requirements ---
			// For each security requirement object, try to satisfy at least one scheme
			securitySatisfied := false
			for _, secReq := range opCopy.Security {
				for secName := range secReq {
					if doc.Components != nil && doc.Components.SecuritySchemes != nil {
						if secSchemeRef, ok := doc.Components.SecuritySchemes[secName]; ok && secSchemeRef.Value != nil {
							secScheme := secSchemeRef.Value
							switch secScheme.Type {
							case "http":
								if secScheme.Scheme == "bearer" {
									if bearer := os.Getenv("BEARER_TOKEN"); bearer != "" {
										httpReq.Header.Set("Authorization", "Bearer "+bearer)
										securitySatisfied = true
									}
								} else if secScheme.Scheme == "basic" {
									if basic := os.Getenv("BASIC_AUTH"); basic != "" {
										encoded := base64.StdEncoding.EncodeToString([]byte(basic))
										httpReq.Header.Set("Authorization", "Basic "+encoded)
										securitySatisfied = true
									}
								}
							case "apiKey":
								if secScheme.In == "header" && secScheme.Name != "" {
									if apiKey := os.Getenv("API_KEY"); apiKey != "" {
										httpReq.Header.Set(secScheme.Name, apiKey)
										securitySatisfied = true
									}
								} else if secScheme.In == "query" && secScheme.Name != "" {
									if apiKey := os.Getenv("API_KEY"); apiKey != "" {
										q := httpReq.URL.Query()
										q.Set(secScheme.Name, apiKey)
										httpReq.URL.RawQuery = q.Encode()
										securitySatisfied = true
									}
								} else if secScheme.In == "cookie" && secScheme.Name != "" {
									if apiKey := os.Getenv("API_KEY"); apiKey != "" {
										cookie := httpReq.Header.Get("Cookie")
										if cookie != "" {
											cookie += "; "
										}
										cookie += secScheme.Name + "=" + apiKey
										httpReq.Header.Set("Cookie", cookie)
										securitySatisfied = true
									}
								}
							case "oauth2":
								if bearer := os.Getenv("BEARER_TOKEN"); bearer != "" {
									httpReq.Header.Set("Authorization", "Bearer "+bearer)
									securitySatisfied = true
								}
							}
						}
					}
				}
			}
			// If no security requirements, fallback to legacy env handling (for backward compatibility)
			if !securitySatisfied {
				if apiKey := os.Getenv("API_KEY"); apiKey != "" {
					httpReq.Header.Set(apiKeyHeader, apiKey)
				}
				if bearer := os.Getenv("BEARER_TOKEN"); bearer != "" {
					httpReq.Header.Set("Authorization", "Bearer "+bearer)
				} else if basic := os.Getenv("BASIC_AUTH"); basic != "" {
					encoded := base64.StdEncoding.EncodeToString([]byte(basic))
					httpReq.Header.Set("Authorization", "Basic "+encoded)
				}
			}
			// Add header parameters
			for _, paramRef := range opCopy.Parameters {
				if paramRef == nil || paramRef.Value == nil {
					continue
				}
				p := paramRef.Value
				if p.In == "header" {
					if val, ok := args[p.Name]; ok {
						httpReq.Header.Set(p.Name, fmt.Sprintf("%v", val))
					}
				}
			}
			// Add cookie parameters (RFC 6265)
			var cookiePairs []string
			for _, paramRef := range opCopy.Parameters {
				if paramRef == nil || paramRef.Value == nil {
					continue
				}
				p := paramRef.Value
				if p.In == "cookie" {
					if val, ok := args[p.Name]; ok {
						cookiePairs = append(cookiePairs, fmt.Sprintf("%s=%v", p.Name, val))
					}
				}
			}
			if len(cookiePairs) > 0 {
				httpReq.Header.Set("Cookie", strings.Join(cookiePairs, "; "))
			}
			resp, err := http.DefaultClient.Do(httpReq)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			respBody, _ := io.ReadAll(resp.Body)

			contentType := resp.Header.Get("Content-Type")
			isJSON := strings.HasPrefix(contentType, "application/json")
			isText := strings.HasPrefix(contentType, "text/")
			isBinary := !isJSON && !isText

			// LLM-friendly error handling for non-2xx responses
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				opSummary := opCopy.Summary
				if opSummary == "" {
					opSummary = opCopy.Description
				}
				opDesc := opCopy.Description
				suggestion := "Check the input parameters, authentication, and consult the tool schema. See the OpenAPI documentation for more details."
				if resp.StatusCode == 401 || resp.StatusCode == 403 {
					suggestion = "Authentication or authorization failed. Ensure you have provided valid credentials or tokens."
				} else if resp.StatusCode == 404 {
					suggestion = "The resource was not found. Check if the resource ID or path is correct."
				} else if resp.StatusCode == 400 {
					suggestion = "Bad request. Check if all required parameters are provided and valid."
				}
				// For binary error responses, include base64 and mime type
				if isBinary {
					fileBase64 := base64.StdEncoding.EncodeToString(respBody)
					fileName := "file"
					if cd := resp.Header.Get("Content-Disposition"); cd != "" {
						if parts := strings.Split(cd, "filename="); len(parts) > 1 {
							fileName = strings.Trim(parts[1], `"`)
						}
					}
					errorObj := map[string]any{
						"type": "api_response",
						"error": map[string]any{
							"code":        "http_error",
							"http_status": resp.StatusCode,
							"message":     fmt.Sprintf("%s (HTTP %d)", http.StatusText(resp.StatusCode), resp.StatusCode),
							"details":     "Binary response (see file_base64)",
							"suggestion":  suggestion,
							"mime_type":   contentType,
							"file_base64": fileBase64,
							"file_name":   fileName,
							"operation": map[string]any{
								"id":          opCopy.OperationID,
								"summary":     opSummary,
								"description": opDesc,
							},
						},
					}
					errorJSON, _ := json.MarshalIndent(errorObj, "", "  ")
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							mcp.TextContent{
								Type: "json",
								Text: string(errorJSON),
							},
						},
						IsError:      true,
						Schema:       inputSchema,
						Arguments:    args,
						Examples:     []any{args},
						Usage:        "call <tool> <json-args>",
						NextSteps:    []string{"list", "schema <tool>"},
						OutputFormat: "structured",
						OutputType:   "file",
					}, nil
				}
				errorObj := map[string]any{
					"type": "api_response",
					"error": map[string]any{
						"code":        "http_error",
						"http_status": resp.StatusCode,
						"message":     fmt.Sprintf("%s (HTTP %d)", http.StatusText(resp.StatusCode), resp.StatusCode),
						"details":     string(respBody),
						"suggestion":  suggestion,
						"operation": map[string]any{
							"id":          opCopy.OperationID,
							"summary":     opSummary,
							"description": opDesc,
						},
					},
				}
				errorJSON, _ := json.MarshalIndent(errorObj, "", "  ")
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.TextContent{
							Type: "json",
							Text: string(errorJSON),
						},
					},
					IsError:      true,
					Schema:       inputSchema,
					Arguments:    args,
					Examples:     []any{args},
					Usage:        "call <tool> <json-args>",
					NextSteps:    []string{"list", "schema <tool>"},
					OutputFormat: "structured",
					OutputType:   "json",
				}, nil
			}

			// Handle binary/file responses for success
			if isBinary && resp.StatusCode >= 200 && resp.StatusCode < 300 {
				fileBase64 := base64.StdEncoding.EncodeToString(respBody)
				fileName := "file"
				if cd := resp.Header.Get("Content-Disposition"); cd != "" {
					if parts := strings.Split(cd, "filename="); len(parts) > 1 {
						fileName = strings.Trim(parts[1], `"`)
					}
				}
				resultObj := map[string]any{
					"type":        "api_response",
					"http_status": resp.StatusCode,
					"mime_type":   contentType,
					"file_base64": fileBase64,
					"file_name":   fileName,
					"operation": map[string]any{
						"id":          opCopy.OperationID,
						"summary":     opCopy.Summary,
						"description": opCopy.Description,
					},
				}
				resultJSON, _ := json.MarshalIndent(resultObj, "", "  ")
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.TextContent{
							Type: "json",
							Text: string(resultJSON),
						},
					},
					Schema:       inputSchema,
					Arguments:    args,
					Examples:     []any{args},
					Usage:        "call <tool> <json-args>",
					NextSteps:    []string{"list", "schema <tool>"},
					OutputFormat: "structured",
					OutputType:   "file",
				}, nil
			}

			// Always format the response as: HTTP <METHOD> <URL>\nStatus: <status>\nResponse:\n<respBody>
			respText := fmt.Sprintf("HTTP %s %s\nStatus: %d\nResponse:\n%s", opCopy.Method, fullURL, resp.StatusCode, string(respBody))
			if args["stream"] == true {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.TextContent{
							Type: "text",
							Text: respText,
						},
					},
					Schema:       inputSchema,
					Arguments:    args,
					Examples:     []any{args},
					Usage:        "call <tool> <json-args>",
					NextSteps:    []string{"list", "schema <tool>"},
					Partial:      true,
					ResumeToken:  "stream-" + fmt.Sprintf("%d", rand.Intn(1000)),
					OutputFormat: "unstructured",
					OutputType:   "text",
				}, nil
			}
			if args["resume_token"] != "" {
				var resumeToken string
				if s, ok := args["resume_token"].(string); ok {
					resumeToken = s
				} else {
					resumeToken = fmt.Sprintf("%v", args["resume_token"])
				}
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.TextContent{
							Type: "text",
							Text: respText,
						},
					},
					Schema:       inputSchema,
					Arguments:    args,
					Examples:     []any{args},
					Usage:        "call <tool> <json-args>",
					NextSteps:    []string{"list", "schema <tool>"},
					Partial:      true,
					ResumeToken:  resumeToken,
					OutputFormat: "unstructured",
					OutputType:   "text",
				}, nil
			}
			if (opts == nil || opts.ConfirmDangerousActions) && (method == "PUT" || method == "POST" || method == "DELETE") {
				if _, confirmed := args["__confirmed"]; !confirmed {
					confirmObj := map[string]any{
						"confirmation_required": true,
						"message":               "This action is irreversible. Proceed?",
						"action":                name,
					}
					apiResponse := map[string]any{"type": "confirmation_request"}
					for k, v := range confirmObj {
						apiResponse[k] = v
					}
					jsonOut, _ := json.MarshalIndent(apiResponse, "", "  ")
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							mcp.TextContent{
								Type: "json",
								Text: string(jsonOut),
							},
						},
						OutputFormat: "structured",
						OutputType:   "json",
						IsError:      false,
					}, nil
				}
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: respText,
					},
				},
				Schema:       inputSchema,
				Arguments:    args,
				Examples:     []any{args},
				Usage:        "call <tool> <json-args>",
				NextSteps:    []string{"list", "schema <tool>"},
				OutputFormat: "unstructured",
				OutputType:   "text",
			}, nil
		})
		toolNames = append(toolNames, name)
	}

	// Add a tool for externalDocs if present
	if doc.ExternalDocs != nil && doc.ExternalDocs.URL != "" {
		desc := "Show the OpenAPI external documentation URL and description."
		inputSchema := map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
		inputSchemaJSON, _ := json.MarshalIndent(inputSchema, "", "  ")
		tool := mcp.NewToolWithRawSchema("externalDocs", desc, inputSchemaJSON)
		tool.Annotations = mcp.ToolAnnotation{}
		if opts != nil && opts.Version != "" {
			tool.Annotations.Title = "OpenAPI " + opts.Version
		}
		server.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			info := "External documentation URL: " + doc.ExternalDocs.URL
			if doc.ExternalDocs.Description != "" {
				info += "\nDescription: " + doc.ExternalDocs.Description
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: info,
					},
				},
				Schema:    inputSchema,
				Arguments: map[string]any{},
				Examples:  []any{},
				Usage:     "call externalDocs <json-args>",
				NextSteps: []string{"list", "schema externalDocs"},
			}, nil
		})
		toolNames = append(toolNames, "externalDocs")
	}

	// Add a tool for info if present
	if doc.Info != nil {
		desc := "Show API metadata: title, version, description, and terms of service."
		inputSchema := map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
		inputSchemaJSON, _ := json.MarshalIndent(inputSchema, "", "  ")
		tool := mcp.NewToolWithRawSchema("info", desc, inputSchemaJSON)
		tool.Annotations = mcp.ToolAnnotation{}
		if opts != nil && opts.Version != "" {
			tool.Annotations.Title = "OpenAPI " + opts.Version
		}
		server.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var sb strings.Builder
			if doc.Info.Title != "" {
				sb.WriteString("Title: " + doc.Info.Title + "\n")
			}
			if doc.Info.Version != "" {
				sb.WriteString("Version: " + doc.Info.Version + "\n")
			}
			if doc.Info.Description != "" {
				sb.WriteString("Description: " + doc.Info.Description + "\n")
			}
			if doc.Info.TermsOfService != "" {
				sb.WriteString("Terms of Service: " + doc.Info.TermsOfService + "\n")
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: strings.TrimSpace(sb.String()),
					},
				},
				Schema:    inputSchema,
				Arguments: map[string]any{},
				Examples:  []any{},
				Usage:     "call info <json-args>",
				NextSteps: []string{"list", "schema info"},
			}, nil
		})
		toolNames = append(toolNames, "info")
	}

	// After registering all OpenAPI tools, add a `describe` tool that returns the full schema and metadata for all tools.
	describeSchema := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
	describeSchemaJSON, _ := json.MarshalIndent(describeSchema, "", "  ")
	describeTool := mcp.NewToolWithRawSchema("describe", "Describe all available tools and their schemas in machine-readable form.", describeSchemaJSON)
	describeTool.Annotations = mcp.ToolAnnotation{Title: "Agent-Friendly Documentation"}
	server.AddTool(describeTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Gather all tools and their schemas
		tools := []map[string]any{}
		for _, tool := range server.ListTools() {
			toolInfo := map[string]any{
				"name":         tool.Name,
				"description":  tool.Description,
				"inputSchema":  tool.InputSchema,
				"annotations":  tool.Annotations,
				"output_type":  "text", // default, can be improved if richer info is available
				"example_call": map[string]any{"name": tool.Name, "arguments": map[string]any{}},
			}
			tools = append(tools, toolInfo)
		}
		response := map[string]any{
			"type":  "tool_descriptions",
			"tools": tools,
		}
		jsonOut, _ := json.MarshalIndent(response, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "json",
					Text: string(jsonOut),
				},
			},
			OutputFormat: "structured",
			OutputType:   "json",
		}, nil
	})
	toolNames = append(toolNames, "describe")

	if opts != nil && opts.DryRun {
		if opts.PrettyPrint {
			out, _ := json.MarshalIndent(toolSummaries, "", "  ")
			fmt.Println(string(out))
		} else {
			out, _ := json.Marshal(toolSummaries)
			fmt.Println(string(out))
		}
	}

	return toolNames
}

// RegisterExtraTool registers an additional custom MCP tool and its handler with the server.
func RegisterExtraTool(server *mcpserver.MCPServer, tool mcp.Tool, handler mcpserver.ToolHandlerFunc) {
	server.AddTool(tool, handler)
}
