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
	httpMaxResponseSize     = 5 * 1024 * 1024 // 5MB
	httpDefaultFetchTimeout = 30 * time.Second
	httpMaxFetchTimeout     = 120 * time.Second
)

// NewHTTPServer creates a new HTTP MCP server
func NewHTTPServer() (*server.MCPServer, error) {
	s := server.NewMCPServer("http-server", "1.0.0", server.WithToolCapabilities(true))

	// Register the fetch tool
	fetchTool := mcp.NewTool("fetch",
		mcp.WithDescription(httpFetchDescription),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The URL to fetch content from"),
		),
		mcp.WithString("format",
			mcp.Required(),
			mcp.Enum("html", "markdown"),
			mcp.Description("The format to return the content in (html or markdown)"),
		),
		mcp.WithBoolean("bodyOnly",
			mcp.Description("Extract only the <body> tag content (default: false)"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Optional timeout in seconds (max 120)"),
			mcp.Min(0),
			mcp.Max(120),
		),
	)

	s.AddTool(fetchTool, executeHTTPFetch)

	return s, nil
}

// executeHTTPFetch handles the fetch tool execution
func executeHTTPFetch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	if format != "html" && format != "markdown" {
		return mcp.NewToolResultError("format must be 'html' or 'markdown'"), nil
	}

	// Get bodyOnly parameter (optional, defaults to false)
	bodyOnly := request.GetBool("bodyOnly", false)

	// Parse timeout (optional)
	timeout := httpDefaultFetchTimeout
	if timeoutSec := request.GetFloat("timeout", 0); timeoutSec > 0 {
		timeoutDuration := time.Duration(timeoutSec) * time.Second
		if timeoutDuration > httpMaxFetchTimeout {
			timeout = httpMaxFetchTimeout
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
	if resp.ContentLength > httpMaxResponseSize {
		return mcp.NewToolResultError("response too large (exceeds 5MB limit)"), nil
	}

	// Read response body with size limit
	limitedReader := io.LimitReader(resp.Body, httpMaxResponseSize+1)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read response: %v", err)), nil
	}

	// Check if we exceeded the size limit
	if len(bodyBytes) > httpMaxResponseSize {
		return mcp.NewToolResultError("response too large (exceeds 5MB limit)"), nil
	}

	content := string(bodyBytes)
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "unknown"
	}

	// Extract body content if requested
	if bodyOnly && strings.Contains(contentType, "text/html") {
		content, err = extractBodyContent(content)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to extract body content: %v", err)), nil
		}
	}

	// Process content based on format
	var output string
	switch format {
	case "html":
		output = content

	case "markdown":
		if strings.Contains(contentType, "text/html") {
			output, err = httpConvertHTMLToMarkdown(content)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to convert HTML to markdown: %v", err)), nil
			}
		} else {
			// Non-HTML content, wrap in code block
			output = "```\n" + content + "\n```"
		}
	}

	// Create result with metadata
	title := fmt.Sprintf("%s (%s)", urlStr, contentType)
	result := mcp.NewToolResultText(output)
	result.Meta = map[string]any{
		"title":       title,
		"url":         urlStr,
		"contentType": contentType,
		"bodyOnly":    bodyOnly,
	}

	return result, nil
}

// extractBodyContent extracts only the <body> tag content from HTML
func extractBodyContent(htmlContent string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return "", err
	}

	// Find the body tag
	bodySelection := doc.Find("body")
	if bodySelection.Length() == 0 {
		// No body tag found, return the original content
		return htmlContent, nil
	}

	// Get the inner HTML of the body tag
	bodyHTML, err := bodySelection.Html()
	if err != nil {
		return "", err
	}

	return bodyHTML, nil
}

// httpConvertHTMLToMarkdown converts HTML content to markdown
func httpConvertHTMLToMarkdown(htmlContent string) (string, error) {
	converter := md.NewConverter("", true, nil)

	// Remove unwanted elements
	converter.Remove("script")
	converter.Remove("style")
	converter.Remove("meta")
	converter.Remove("link")
	converter.Remove("noscript")

	markdown, err := converter.ConvertString(htmlContent)
	if err != nil {
		return "", err
	}

	return markdown, nil
}

const httpFetchDescription = `Performs HTTP GET requests and returns content in HTML or Markdown format.

- Fetches content from a specified URL using HTTP GET
- Returns content in either original HTML or converted Markdown format
- Can optionally extract only the <body> tag content to reduce text size
- Supports custom timeout configuration

Usage notes:
  - The URL must be a fully-formed valid URL
  - Only HTTP GET requests are supported
  - Maximum response size is 5MB
  - Supports two output formats:
    - "html": Raw HTML content
    - "markdown": HTML converted to markdown format
  - Use bodyOnly=true to extract only the <body> tag content (useful for reducing text)
  - Timeout can be specified in seconds (default 30s, max 120s)`
