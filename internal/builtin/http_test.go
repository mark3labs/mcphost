package builtin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewHTTPServer(t *testing.T) {
	server, err := NewHTTPServer()
	if err != nil {
		t.Fatalf("Failed to create HTTP server: %v", err)
	}

	if server == nil {
		t.Fatal("Expected server to be non-nil")
	}
}

func TestHTTPServerRegistry(t *testing.T) {
	registry := NewRegistry()

	// Test that HTTP server is registered
	servers := registry.ListServers()
	found := false
	for _, name := range servers {
		if name == "http" {
			found = true
			break
		}
	}

	if !found {
		t.Error("http server not found in registry")
	}

	// Test creating HTTP server through registry
	wrapper, err := registry.CreateServer("http", map[string]any{})
	if err != nil {
		t.Fatalf("Failed to create HTTP server through registry: %v", err)
	}

	if wrapper == nil {
		t.Fatal("Expected wrapper to be non-nil")
	}

	if wrapper.GetServer() == nil {
		t.Fatal("Expected wrapped server to be non-nil")
	}
}

func TestExecuteHTTPFetch(t *testing.T) {
	// Create a test HTTP server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/html":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
<h1>Hello World</h1>
<p>This is a test paragraph.</p>
</body>
</html>`))
		case "/text":
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("This is plain text content"))
		case "/large":
			// Return content larger than 5MB
			w.Header().Set("Content-Type", "text/plain")
			largeContent := strings.Repeat("x", 6*1024*1024)
			w.Write([]byte(largeContent))
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer testServer.Close()

	tests := []struct {
		name        string
		params      map[string]any
		expectError bool
		checkResult func(t *testing.T, result *mcp.CallToolResult)
	}{
		{
			name: "fetch HTML as HTML",
			params: map[string]any{
				"url":    testServer.URL + "/html",
				"format": "html",
			},
			expectError: false,
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
					if !strings.Contains(textContent.Text, "<h1>Hello World</h1>") {
						t.Error("Expected HTML content to contain h1 tag")
					}
				} else {
					t.Error("Expected text content")
				}
			},
		},
		{
			name: "fetch HTML as markdown",
			params: map[string]any{
				"url":    testServer.URL + "/html",
				"format": "markdown",
			},
			expectError: false,
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
					if !strings.Contains(textContent.Text, "# Hello World") {
						t.Error("Expected markdown content to contain heading")
					}
				} else {
					t.Error("Expected text content")
				}
			},
		},
		{
			name: "fetch HTML body only",
			params: map[string]any{
				"url":      testServer.URL + "/html",
				"format":   "html",
				"bodyOnly": true,
			},
			expectError: false,
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
					content := textContent.Text
					if strings.Contains(content, "<html>") || strings.Contains(content, "<head>") {
						t.Error("Expected body-only content to not contain html or head tags")
					}
					if !strings.Contains(content, "<h1>Hello World</h1>") {
						t.Error("Expected body-only content to contain h1 tag")
					}
				} else {
					t.Error("Expected text content")
				}
			},
		},
		{
			name: "fetch plain text as markdown",
			params: map[string]any{
				"url":    testServer.URL + "/text",
				"format": "markdown",
			},
			expectError: false,
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
					if !strings.Contains(textContent.Text, "```") {
						t.Error("Expected plain text to be wrapped in code block for markdown")
					}
				} else {
					t.Error("Expected text content")
				}
			},
		},
		{
			name: "missing URL parameter",
			params: map[string]any{
				"format": "html",
			},
			expectError: true,
		},
		{
			name: "missing format parameter",
			params: map[string]any{
				"url": testServer.URL + "/html",
			},
			expectError: true,
		},
		{
			name: "invalid format",
			params: map[string]any{
				"url":    testServer.URL + "/html",
				"format": "invalid",
			},
			expectError: true,
		},
		{
			name: "server error response",
			params: map[string]any{
				"url":    testServer.URL + "/error",
				"format": "html",
			},
			expectError: true,
		},
		{
			name: "response too large",
			params: map[string]any{
				"url":    testServer.URL + "/large",
				"format": "html",
			},
			expectError: true,
		},
		{
			name: "invalid URL",
			params: map[string]any{
				"url":    "not-a-valid-url",
				"format": "html",
			},
			expectError: true,
		},
		{
			name: "with custom timeout",
			params: map[string]any{
				"url":     testServer.URL + "/html",
				"format":  "html",
				"timeout": 10,
			},
			expectError: false,
			checkResult: func(t *testing.T, result *mcp.CallToolResult) {
				if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
					if !strings.Contains(textContent.Text, "<h1>Hello World</h1>") {
						t.Error("Expected HTML content with custom timeout")
					}
				} else {
					t.Error("Expected text content")
				}
			},
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "fetch",
					Arguments: tt.params,
				},
			}

			result, err := executeHTTPFetch(ctx, request)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.expectError {
				if !result.IsError {
					t.Error("Expected error result but got success")
				}
			} else {
				if result.IsError {
					if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
						t.Errorf("Expected success but got error: %v", textContent.Text)
					} else {
						t.Error("Expected error to have text content")
					}
				} else if tt.checkResult != nil {
					tt.checkResult(t, result)
				}
			}
		})
	}
}

func TestExtractBodyContent(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name: "extract body content",
			html: `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<h1>Content</h1>
<p>Paragraph</p>
</body>
</html>`,
			expected: "\n<h1>Content</h1>\n<p>Paragraph</p>\n\n",
		},
		{
			name:     "no body tag",
			html:     `<div>No body tag</div>`,
			expected: `<div>No body tag</div>`,
		},
		{
			name:     "empty body",
			html:     `<html><body></body></html>`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractBodyContent(tt.html)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
