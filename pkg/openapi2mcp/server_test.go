package openapi2mcp

import (
	"testing"
)

func TestGetSSEURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected string
	}{
		{
			name:     "basic URL",
			baseURL:  "http://localhost:8080",
			expected: "http://localhost:8080/mcp/sse",
		},
		{
			name:     "URL with trailing slash",
			baseURL:  "http://localhost:8080/",
			expected: "http://localhost:8080/mcp/sse",
		},
		{
			name:     "HTTPS URL",
			baseURL:  "https://api.example.com",
			expected: "https://api.example.com/mcp/sse",
		},
		{
			name:     "URL with port and path",
			baseURL:  "http://example.com:3000",
			expected: "http://example.com:3000/mcp/sse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetSSEURL(tt.baseURL)
			if result != tt.expected {
				t.Errorf("GetSSEURL(%q) = %q, want %q", tt.baseURL, result, tt.expected)
			}
		})
	}
}

func TestGetMessageURL(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		sessionID string
		expected  string
	}{
		{
			name:      "basic URL with session",
			baseURL:   "http://localhost:8080",
			sessionID: "session-123",
			expected:  "http://localhost:8080/mcp/message?sessionId=session-123",
		},
		{
			name:      "URL with trailing slash",
			baseURL:   "http://localhost:8080/",
			sessionID: "abc-def-ghi",
			expected:  "http://localhost:8080/mcp/message?sessionId=abc-def-ghi",
		},
		{
			name:      "HTTPS URL with UUID session",
			baseURL:   "https://api.example.com",
			sessionID: "550e8400-e29b-41d4-a716-446655440000",
			expected:  "https://api.example.com/mcp/message?sessionId=550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:      "empty session ID",
			baseURL:   "http://localhost:8080",
			sessionID: "",
			expected:  "http://localhost:8080/mcp/message?sessionId=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetMessageURL(tt.baseURL, tt.sessionID)
			if result != tt.expected {
				t.Errorf("GetMessageURL(%q, %q) = %q, want %q", tt.baseURL, tt.sessionID, result, tt.expected)
			}
		})
	}
}
