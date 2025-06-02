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
	"github.com/jedisct1/openapi-mcp/pkg/mcp/mcp"
	mcpserver "github.com/jedisct1/openapi-mcp/pkg/mcp/server"
	"github.com/xeipuuv/gojsonschema"
)

// generateAI400ErrorResponse creates a comprehensive, AI-optimized error response for 400 HTTP errors
// that helps agents understand how to correctly use the tool.
func generateAI400ErrorResponse(op OpenAPIOperation, inputSchemaJSON []byte, args map[string]any, responseBody string) string {
	var response strings.Builder

	// Start with clear explanation
	response.WriteString("BAD REQUEST (400): The API call failed due to incorrect or invalid parameters.\n\n")

	// Operation context
	response.WriteString(fmt.Sprintf("OPERATION: %s", op.OperationID))
	if op.Summary != "" {
		response.WriteString(fmt.Sprintf(" - %s", op.Summary))
	}
	response.WriteString("\n")
	if op.Description != "" {
		response.WriteString(fmt.Sprintf("DESCRIPTION: %s\n", op.Description))
	}
	response.WriteString("\n")

	// Parse the input schema to provide detailed parameter guidance
	var schemaObj map[string]any
	_ = json.Unmarshal(inputSchemaJSON, &schemaObj)

	if properties, ok := schemaObj["properties"].(map[string]any); ok && len(properties) > 0 {
		response.WriteString("PARAMETER REQUIREMENTS:\n")

		// Required parameters
		if required, ok := schemaObj["required"].([]any); ok && len(required) > 0 {
			response.WriteString("• Required parameters:\n")
			for _, req := range required {
				if reqStr, ok := req.(string); ok {
					if prop, ok := properties[reqStr].(map[string]any); ok {
						response.WriteString(fmt.Sprintf("  - %s", reqStr))
						if typeStr, ok := prop["type"].(string); ok {
							response.WriteString(fmt.Sprintf(" (%s)", typeStr))
						}
						if desc, ok := prop["description"].(string); ok && desc != "" {
							response.WriteString(fmt.Sprintf(": %s", desc))
						}
						response.WriteString("\n")
					}
				}
			}
			response.WriteString("\n")
		}

		// All parameters with details
		response.WriteString("• All available parameters:\n")
		for paramName, paramDef := range properties {
			if prop, ok := paramDef.(map[string]any); ok {
				response.WriteString(fmt.Sprintf("  - %s", paramName))

				// Type information
				if typeStr, ok := prop["type"].(string); ok {
					response.WriteString(fmt.Sprintf(" (%s)", typeStr))
				}

				// Required indicator
				if required, ok := schemaObj["required"].([]any); ok {
					for _, req := range required {
						if reqStr, ok := req.(string); ok && reqStr == paramName {
							response.WriteString(" [REQUIRED]")
							break
						}
					}
				}

				// Description
				if desc, ok := prop["description"].(string); ok && desc != "" {
					response.WriteString(fmt.Sprintf(": %s", desc))
				}

				// Enum values
				if enum, ok := prop["enum"].([]any); ok && len(enum) > 0 {
					response.WriteString(" | Valid values: ")
					var enumStrs []string
					for _, e := range enum {
						enumStrs = append(enumStrs, fmt.Sprintf("%v", e))
					}
					response.WriteString(strings.Join(enumStrs, ", "))
				}

				response.WriteString("\n")
			}
		}
		response.WriteString("\n")
	}

	// Analyze current arguments
	if len(args) > 0 {
		response.WriteString("YOUR CURRENT ARGUMENTS:\n")
		argsJSON, _ := json.MarshalIndent(args, "", "  ")
		response.WriteString(string(argsJSON))
		response.WriteString("\n\n")
	}

	// Server error details if available
	if responseBody != "" {
		response.WriteString("SERVER ERROR DETAILS:\n")
		response.WriteString(responseBody)
		response.WriteString("\n\n")
	}

	// Generate example with correct parameters
	response.WriteString("EXAMPLE CORRECT USAGE:\n")
	if properties, ok := schemaObj["properties"].(map[string]any); ok {
		exampleArgs := map[string]any{}

		// Prioritize required parameters
		if required, ok := schemaObj["required"].([]any); ok {
			for _, req := range required {
				if reqStr, ok := req.(string); ok {
					if prop, ok := properties[reqStr].(map[string]any); ok {
						exampleArgs[reqStr] = generateExampleValue(prop)
					}
				}
			}
		}

		// Add some optional parameters for completeness
		count := 0
		for paramName, paramDef := range properties {
			if _, exists := exampleArgs[paramName]; !exists && count < 3 {
				if prop, ok := paramDef.(map[string]any); ok {
					exampleArgs[paramName] = generateExampleValue(prop)
					count++
				}
			}
		}

		exampleJSON, _ := json.MarshalIndent(exampleArgs, "", "  ")
		response.WriteString(fmt.Sprintf("call %s %s\n\n", op.OperationID, string(exampleJSON)))
	}

	// Actionable guidance
	response.WriteString("TROUBLESHOOTING STEPS:\n")
	response.WriteString("1. Verify all required parameters are provided\n")
	response.WriteString("2. Check parameter types match the schema (string, number, boolean, etc.)\n")
	response.WriteString("3. Ensure enum values are from the allowed list\n")
	response.WriteString("4. Validate parameter formats (dates, emails, URLs, etc.)\n")
	response.WriteString("5. Check for missing or incorrectly named parameters\n")
	response.WriteString("6. Review the server error details above for specific validation failures\n")

	return response.String()
}

// generateExampleValue creates appropriate example values based on the parameter schema
func generateExampleValue(prop map[string]any) any {
	typeStr, _ := prop["type"].(string)

	// Check for enum values first
	if enum, ok := prop["enum"].([]any); ok && len(enum) > 0 {
		return enum[0]
	}

	// Check for example values in schema
	if example, ok := prop["example"]; ok {
		return example
	}

	// Generate based on type
	switch typeStr {
	case "string":
		if format, ok := prop["format"].(string); ok {
			switch format {
			case "email":
				return "user@example.com"
			case "uri", "url":
				return "https://example.com"
			case "date":
				return "2024-01-01"
			case "date-time":
				return "2024-01-01T00:00:00Z"
			case "uuid":
				return "123e4567-e89b-12d3-a456-426614174000"
			default:
				return "example_string"
			}
		}
		return "example_string"
	case "number":
		return 123.45
	case "integer":
		return 123
	case "boolean":
		return true
	case "array":
		if items, ok := prop["items"].(map[string]any); ok {
			return []any{generateExampleValue(items)}
		}
		return []any{"item1", "item2"}
	case "object":
		return map[string]any{"key": "value"}
	default:
		return nil
	}
}

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
					errMsg := ""

					// Handle different validation error types with plain text messages
					switch verr.Type() {
					case "required":
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
									} else {
										errMsg = "Missing required parameter: '" + missing + "'"
									}
								} else {
									errMsg = "Missing required parameter: '" + missing + "'"
								}
							}
						}
					case "invalid_type":
						// Convert "Invalid type. Expected: string, given: integer" to plain text
						errMsg = verr.String()
					case "enum":
						// Convert enum validation errors to plain text
						errMsg = verr.String()
					case "invalid_union", "one_of", "any_of":
						// Convert union/oneOf/anyOf errors to plain text
						errMsg = "Invalid value. " + verr.String()
					default:
						// For any other validation error types, ensure it's plain text
						errMsg = verr.String()
					}

					if errMsg != "" {
						errMsgs += errMsg + "\n"
					}
				}
				// Suggest a retry with an example argument set
				exampleArgs := map[string]any{}
				for k, v := range properties {
					if prop, ok := v.(map[string]any); ok {
						typeStr, _ := prop["type"].(string)
						switch typeStr {
						case "string":
							exampleArgs[k] = "example"
						case "number":
							exampleArgs[k] = 123.45
						case "integer":
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

				// Create a simple text error message
				errorText := strings.TrimSpace(errMsgs)
				if len(suggestions) > 0 {
					errorText += "\n\n" + strings.Join(suggestions, "\n")
				}

				return mcp.NewToolResultError(
					errorText,
					inputSchema,
					args,
					[]any{args},
					"call <tool> <json-args>",
					[]string{"list", "schema <tool>"},
				), nil
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
					suggestion = generateAI400ErrorResponse(opCopy, inputSchemaJSON, args, string(respBody))
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
				// Create a simple text error message
				errorText := fmt.Sprintf("HTTP Error: %s (HTTP %d)", http.StatusText(resp.StatusCode), resp.StatusCode)
				if len(respBody) > 0 {
					errorText += "\nDetails: " + string(respBody)
				}
				if suggestion != "" {
					errorText += "\nSuggestion: " + suggestion
				}
				errorText += fmt.Sprintf("\nOperation: %s (%s)", opCopy.OperationID, opSummary)

				return mcp.NewToolResultError(
					errorText,
					inputSchema,
					args,
					[]any{args},
					"call <tool> <json-args>",
					[]string{"list", "schema <tool>"},
				), nil
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
					confirmText := fmt.Sprintf("⚠️  CONFIRMATION REQUIRED\n\nAction: %s\nThis action is irreversible. Proceed?\n\nTo confirm, retry the call with {\"__confirmed\": true} added to your arguments.", name)
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							mcp.TextContent{
								Type: "text",
								Text: confirmText,
							},
						},
						OutputFormat: "unstructured",
						OutputType:   "text",
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
	if doc.ExternalDocs != nil && doc.ExternalDocs.URL != "" && (opts == nil || !opts.DryRun) {
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
	if doc.Info != nil && (opts == nil || !opts.DryRun) {
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
	if opts == nil || !opts.DryRun {
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
	}

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
