package openapi2mcp

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mark3labs/mcp-go/server"
)

func stringPtr(s string) *string {
	return &s
}

func minimalOpenAPIDoc() *openapi3.T {
	return &openapi3.T{
		Info: &openapi3.Info{Title: "Test API", Version: "1.0.0"},
		Paths: openapi3.Paths{
			"/foo": &openapi3.PathItem{
				Get: &openapi3.Operation{
					OperationID: "getFoo",
					Summary:     "Get Foo",
					Parameters:  openapi3.Parameters{},
				},
			},
		},
	}
}

func toolSetEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ma := map[string]struct{}{}
	mb := map[string]struct{}{}
	for _, x := range a {
		ma[x] = struct{}{}
	}
	for _, x := range b {
		mb[x] = struct{}{}
	}
	for k := range ma {
		if _, ok := mb[k]; !ok {
			return false
		}
	}
	return true
}

func TestRegisterOpenAPITools_Basic(t *testing.T) {
	doc := minimalOpenAPIDoc()
	srv := server.NewMCPServer("test", "1.0.0")
	ops := ExtractOpenAPIOperations(doc)
	opts := &ToolGenOptions{}
	names := RegisterOpenAPITools(srv, ops, doc, opts)
	expected := []string{"getFoo", "info", "describe"}
	if !toolSetEqual(names, expected) {
		t.Fatalf("expected tools %v, got: %v", expected, names)
	}
}

func TestRegisterOpenAPITools_TagFilter(t *testing.T) {
	doc := minimalOpenAPIDoc()
	doc.Paths["/foo"].Get.Tags = []string{"bar"}
	srv := server.NewMCPServer("test", "1.0.0")
	ops := ExtractOpenAPIOperations(doc)
	opts := &ToolGenOptions{
		TagFilter: []string{"baz"}, // should filter out
	}
	names := RegisterOpenAPITools(srv, ops, doc, opts)
	expected := []string{"info", "describe"}
	if !toolSetEqual(names, expected) {
		t.Fatalf("expected only meta tools %v, got: %v", expected, names)
	}
}

func TestSelfTestOpenAPIMCP_Pass(t *testing.T) {
	doc := minimalOpenAPIDoc()
	srv := server.NewMCPServer("test", "1.0.0")
	ops := ExtractOpenAPIOperations(doc)
	opts := &ToolGenOptions{}
	RegisterOpenAPITools(srv, ops, doc, opts)
	toolNames := make([]string, 0)
	for _, tool := range srv.ListTools() {
		toolNames = append(toolNames, tool.Name)
	}
	err := SelfTestOpenAPIMCP(doc, toolNames)
	if err != nil {
		t.Fatalf("expected selftest to pass, got: %v", err)
	}
}

func TestSelfTestOpenAPIMCP_MissingTool(t *testing.T) {
	doc := minimalOpenAPIDoc()
	err := SelfTestOpenAPIMCP(doc, []string{})
	if err == nil {
		t.Fatalf("expected selftest to fail due to missing tool")
	}
}

func TestNumberVsIntegerTypes(t *testing.T) {
	// Create a spec with both number and integer types
	doc := &openapi3.T{
		Info: &openapi3.Info{Title: "Number Test API", Version: "1.0.0"},
		Paths: openapi3.Paths{
			"/test": &openapi3.PathItem{
				Post: &openapi3.Operation{
					OperationID: "testNumbers",
					Summary:     "Test number types",
					RequestBody: &openapi3.RequestBodyRef{
						Value: &openapi3.RequestBody{
							Required: true,
							Content: openapi3.Content{
								"application/json": &openapi3.MediaType{
									Schema: &openapi3.SchemaRef{
										Value: &openapi3.Schema{
											Type: "object",
											Properties: openapi3.Schemas{
												"integerField": &openapi3.SchemaRef{
													Value: &openapi3.Schema{Type: "integer"},
												},
												"numberField": &openapi3.SchemaRef{
													Value: &openapi3.Schema{Type: "number"},
												},
											},
											Required: []string{"integerField", "numberField"},
										},
									},
								},
							},
						},
					},
					Responses: openapi3.Responses{
						"200": &openapi3.ResponseRef{
							Value: &openapi3.Response{Description: stringPtr("OK")},
						},
					},
				},
			},
		},
	}

	ops := ExtractOpenAPIOperations(doc)
	if len(ops) == 0 {
		t.Fatal("No operations extracted")
	}

	op := ops[0]
	if op.OperationID != "testNumbers" {
		t.Fatalf("Expected operation ID 'testNumbers', got '%s'", op.OperationID)
	}

	// Build the input schema and check that it handles number vs integer correctly
	inputSchema := BuildInputSchema(op.Parameters, op.RequestBody)

	// The schema should be valid and not cause any errors when processed
	if inputSchema == nil {
		t.Fatal("Input schema is nil")
	}

	// Verify that the schema contains the expected properties
	props, ok := inputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("Schema properties not found")
	}

	// Should have requestBody property
	requestBodyProp, ok := props["requestBody"].(map[string]any)
	if !ok {
		t.Fatal("requestBody property not found")
	}

	// Check that requestBody has the correct nested properties
	requestBodyProps, ok := requestBodyProp["properties"].(map[string]any)
	if !ok {
		t.Fatal("requestBody properties not found")
	}

	// Verify integerField has type integer
	if intField, ok := requestBodyProps["integerField"].(map[string]any); ok {
		if fieldType, ok := intField["type"].(string); !ok || fieldType != "integer" {
			t.Errorf("Expected integerField to have type 'integer', got '%v'", fieldType)
		}
	} else {
		t.Error("integerField not found in schema")
	}

	// Verify numberField has type number
	if numField, ok := requestBodyProps["numberField"].(map[string]any); ok {
		if fieldType, ok := numField["type"].(string); !ok || fieldType != "number" {
			t.Errorf("Expected numberField to have type 'number', got '%v'", fieldType)
		}
	} else {
		t.Error("numberField not found in schema")
	}
}

func TestFormatPreservation(t *testing.T) {
	// Create a spec with various format specifiers
	doc := &openapi3.T{
		Info: &openapi3.Info{Title: "Format Test API", Version: "1.0.0"},
		Paths: openapi3.Paths{
			"/test": &openapi3.PathItem{
				Post: &openapi3.Operation{
					OperationID: "testFormats",
					Summary:     "Test format preservation",
					RequestBody: &openapi3.RequestBodyRef{
						Value: &openapi3.RequestBody{
							Required: true,
							Content: openapi3.Content{
								"application/json": &openapi3.MediaType{
									Schema: &openapi3.SchemaRef{
										Value: &openapi3.Schema{
											Type: "object",
											Properties: openapi3.Schemas{
												"int32Field": &openapi3.SchemaRef{
													Value: &openapi3.Schema{Type: "integer", Format: "int32"},
												},
												"floatField": &openapi3.SchemaRef{
													Value: &openapi3.Schema{Type: "number", Format: "float"},
												},
												"dateField": &openapi3.SchemaRef{
													Value: &openapi3.Schema{Type: "string", Format: "date"},
												},
											},
										},
									},
								},
							},
						},
					},
					Responses: openapi3.Responses{
						"200": &openapi3.ResponseRef{
							Value: &openapi3.Response{Description: stringPtr("OK")},
						},
					},
				},
			},
		},
	}

	ops := ExtractOpenAPIOperations(doc)
	if len(ops) == 0 {
		t.Fatal("No operations extracted")
	}

	op := ops[0]
	inputSchema := BuildInputSchema(op.Parameters, op.RequestBody)

	// Navigate to request body properties
	props, ok := inputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("Schema properties not found")
	}

	requestBodyProp, ok := props["requestBody"].(map[string]any)
	if !ok {
		t.Fatal("requestBody property not found")
	}

	requestBodyProps, ok := requestBodyProp["properties"].(map[string]any)
	if !ok {
		t.Fatal("requestBody properties not found")
	}

	// Verify format preservation for int32Field
	if int32Field, ok := requestBodyProps["int32Field"].(map[string]any); ok {
		if format, ok := int32Field["format"].(string); !ok || format != "int32" {
			t.Errorf("Expected int32Field to have format 'int32', got '%v'", format)
		}
		if fieldType, ok := int32Field["type"].(string); !ok || fieldType != "integer" {
			t.Errorf("Expected int32Field to have type 'integer', got '%v'", fieldType)
		}
	} else {
		t.Error("int32Field not found in schema")
	}

	// Verify format preservation for floatField
	if floatField, ok := requestBodyProps["floatField"].(map[string]any); ok {
		if format, ok := floatField["format"].(string); !ok || format != "float" {
			t.Errorf("Expected floatField to have format 'float', got '%v'", format)
		}
		if fieldType, ok := floatField["type"].(string); !ok || fieldType != "number" {
			t.Errorf("Expected floatField to have type 'number', got '%v'", fieldType)
		}
	} else {
		t.Error("floatField not found in schema")
	}

	// Verify format preservation for dateField
	if dateField, ok := requestBodyProps["dateField"].(map[string]any); ok {
		if format, ok := dateField["format"].(string); !ok || format != "date" {
			t.Errorf("Expected dateField to have format 'date', got '%v'", format)
		}
		if fieldType, ok := dateField["type"].(string); !ok || fieldType != "string" {
			t.Errorf("Expected dateField to have type 'string', got '%v'", fieldType)
		}
	} else {
		t.Error("dateField not found in schema")
	}
}
