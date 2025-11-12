package builtin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	maxResponseSize     = 5 * 1024 * 1024 // 5MB
	defaultFetchTimeout = 30 * time.Second
	maxFetchTimeout     = 120 * time.Second
)

// NewFetchServer creates a new MCP server that provides web content fetching capabilities.
// The server includes a single tool "fetch" that retrieves content from URLs and converts
// it to text, markdown, or HTML format. Returns an error if server initialization fails.
func NewFetchServer() (*server.MCPServer, error) {
	s := server.NewMCPServer("fetch-server", "1.0.0", server.WithToolCapabilities(true))

	// Register the fetch tool
	fetchTool := mcp.NewTool("fetch",
		mcp.WithDescription(fetchDescription),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The URL to fetch content from"),
		),
		mcp.WithString("format",
			mcp.Required(),
			mcp.Enum("text", "markdown", "html"),
			mcp.Description("The format to return the content in (text, markdown, or html)"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Optional timeout in seconds (max 120)"),
			mcp.Min(0),
			mcp.Max(120),
		),
	)

	s.AddTool(fetchTool, executeFetch)

	return s, nil
}

// executeFetch handles the fetch tool execution
func executeFetch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters
	urlStr, err := request.RequireString("url")
	if err != nil {
		return mcp.NewToolResultError("url parameter is required and must be a string"), nil
	}

	format, err := request.RequireString("format")
	if err != nil {
		return mcp.NewToolResultError("format parameter is required and must be a string"), nil
	}

	// Validate format
	if format != "text" && format != "markdown" && format != "html" {
		return mcp.NewToolResultError("format must be 'text', 'markdown', or 'html'"), nil
	}

	// Parse timeout (optional)
	timeout := defaultFetchTimeout
	if timeoutSec := request.GetFloat("timeout", 0); timeoutSec > 0 {
		timeoutDuration := time.Duration(timeoutSec) * time.Second
		if timeoutDuration > maxFetchTimeout {
			timeout = maxFetchTimeout
		} else {
			timeout = timeoutDuration
		}
	}

	// Validate URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid URL: %v", err)), nil
	}

	// Ensure URL has a scheme
	if parsedURL.Scheme == "" {
		urlStr = "https://" + urlStr
		parsedURL, err = url.Parse(urlStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid URL after adding https: %v", err)), nil
		}
	}

	// Only allow HTTP and HTTPS
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return mcp.NewToolResultError("URL must use http:// or https://"), nil
	}

	// Upgrade HTTP to HTTPS only for external URLs (not localhost/127.0.0.1)
	if parsedURL.Scheme == "http" && !isLocalhost(parsedURL.Host) {
		parsedURL.Scheme = "https"
		urlStr = parsedURL.String()
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: timeout,
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create request: %v", err)), nil
	}

	// Set headers to mimic a real browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("request failed: %v", err)), nil
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return mcp.NewToolResultError(fmt.Sprintf("request failed with status code: %d", resp.StatusCode)), nil
	}

	// Check content length
	if resp.ContentLength > maxResponseSize {
		return mcp.NewToolResultError("response too large (exceeds 5MB limit)"), nil
	}

	// Read response body with size limit
	limitedReader := io.LimitReader(resp.Body, maxResponseSize+1)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read response: %v", err)), nil
	}

	// Check if we exceeded the size limit
	if len(bodyBytes) > maxResponseSize {
		return mcp.NewToolResultError("response too large (exceeds 5MB limit)"), nil
	}

	content := string(bodyBytes)
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "unknown"
	}

	// Process content based on format
	var output string
	switch format {
	case "text":
		if strings.Contains(contentType, "text/html") {
			output, err = extractTextFromHTML(content)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to extract text from HTML: %v", err)), nil
			}
		} else {
			output = content
		}

	case "markdown":
		if strings.Contains(contentType, "text/html") {
			output, err = convertHTMLToMarkdown(content)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to convert HTML to markdown: %v", err)), nil
			}
		} else {
			output = "```\n" + content + "\n```"
		}

	case "html":
		output = content

	default:
		output = content
	}

	// Create result with metadata
	title := fmt.Sprintf("%s (%s)", urlStr, contentType)
	result := mcp.NewToolResultText(output)
	result.Meta = &mcp.Meta{
		AdditionalFields: map[string]any{
			"title": title,
		},
	}

	return result, nil
}

// extractTextFromHTML extracts plain text from HTML content
func extractTextFromHTML(htmlContent string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return "", err
	}

	// Remove script, style, and other non-content elements
	doc.Find("script, style, noscript, iframe, object, embed").Remove()

	// Extract text content
	text := doc.Text()

	// Clean up whitespace
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanLines = append(cleanLines, trimmed)
		}
	}

	return strings.Join(cleanLines, "\n"), nil
}

// convertHTMLToMarkdown converts HTML content to markdown
func convertHTMLToMarkdown(htmlContent string) (string, error) {
	converter := md.NewConverter("", true, nil)

	// Remove unwanted elements
	converter.Remove("script")
	converter.Remove("style")
	converter.Remove("meta")
	converter.Remove("link")

	markdown, err := converter.ConvertString(htmlContent)
	if err != nil {
		return "", err
	}

	return markdown, nil
}

// isLocalhost checks if the host is localhost or 127.0.0.1
func isLocalhost(host string) bool {
	return strings.HasPrefix(host, "localhost") ||
		strings.HasPrefix(host, "127.0.0.1") ||
		strings.HasPrefix(host, "::1")
}

const fetchDescription = `Fetches content from a specified URL and returns it in the requested format.

- Fetches content from a specified URL
- Takes a URL and format as input
- Fetches the URL content, converts HTML to markdown or text as requested
- Returns the content in the specified format
- Use this tool when you need to retrieve and analyze web content

Usage notes:
  - IMPORTANT: If an MCP-provided web fetch tool is available, prefer using that tool instead of this one, as it may have fewer restrictions. All MCP-provided tools start with "mcp__".
  - The URL must be a fully-formed valid URL
  - HTTP URLs will be automatically upgraded to HTTPS
  - This tool is read-only and does not modify any files
  - Results may be summarized if the content is very large (max 5MB)
  - Supports three output formats:
    - "text": Plain text extraction from HTML, or raw content for non-HTML
    - "markdown": HTML converted to markdown, or code-wrapped for non-HTML
    - "html": Raw HTML content
  - Timeout can be specified in seconds (default 30s, max 120s)`
