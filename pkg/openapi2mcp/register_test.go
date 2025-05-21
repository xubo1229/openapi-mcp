package openapi2mcp

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mark3labs/mcp-go/server"
)

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
