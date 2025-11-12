package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/tidwall/gjson"
)

const (
	httpMaxResponseSize     = 5 * 1024 * 1024 // 5MB
	httpDefaultFetchTimeout = 30 * time.Second
	httpMaxFetchTimeout     = 120 * time.Second
)

// httpServerModel holds the model for the HTTP server
var httpServerModel model.ToolCallingChatModel

// NewHTTPServer creates a new MCP server providing advanced HTTP fetching capabilities.
// The server includes tools for fetching web content, summarizing pages, extracting
// specific information, and filtering JSON responses. If an LLM model is provided,
// AI-powered summarization and extraction tools are enabled. Returns an error if
// server initialization fails.
func NewHTTPServer(llmModel model.ToolCallingChatModel) (*server.MCPServer, error) {
	// Store the model globally for use in tool handlers
	httpServerModel = llmModel

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

	// Only add the summarize tool if we have a model
	if llmModel != nil {
		summarizeTool := mcp.NewTool("fetch_summarize",
			mcp.WithDescription(httpSummarizeDescription),
			mcp.WithString("url",
				mcp.Required(),
				mcp.Description("The URL to fetch and summarize"),
			),
			mcp.WithString("instructions",
				mcp.Description("Optional summarization instructions (default: 'Provide a concise summary')"),
			),
		)
		s.AddTool(summarizeTool, executeHTTPFetchSummarize)

		extractTool := mcp.NewTool("fetch_extract",
			mcp.WithDescription(httpExtractDescription),
			mcp.WithString("url",
				mcp.Required(),
				mcp.Description("The URL to fetch and extract data from"),
			),
			mcp.WithString("instructions",
				mcp.Required(),
				mcp.Description("Specific extraction instructions (e.g., 'Extract all product names and prices', 'Get the main article content', 'Find all email addresses')"),
			),
		)
		s.AddTool(extractTool, executeHTTPFetchExtract)

		filterJSONTool := mcp.NewTool("fetch_filtered_json",
			mcp.WithDescription(httpFilterJSONDescription),
			mcp.WithString("url",
				mcp.Required(),
				mcp.Description("The URL to fetch JSON content from"),
			),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("The gjson path expression to filter the JSON (e.g., 'users.#.name', 'data.items.0', 'results.#(age>25).name')"),
			),
			mcp.WithNumber("timeout",
				mcp.Description("Optional timeout in seconds (max 120)"),
				mcp.Min(0),
				mcp.Max(120),
			),
		)
		s.AddTool(filterJSONTool, executeHTTPFetchFilteredJSON)
	}

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
	result.Meta = &mcp.Meta{
		AdditionalFields: map[string]any{
			"title":       title,
			"url":         urlStr,
			"contentType": contentType,
			"bodyOnly":    bodyOnly,
		},
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

// executeHTTPFetchSummarize handles the fetch_summarize tool execution
func executeHTTPFetchSummarize(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get URL
	urlStr, err := request.RequireString("url")
	if err != nil {
		return mcp.NewToolResultError("url parameter is required and must be a string"), nil
	}

	// Get optional instructions
	instructions := request.GetString("instructions", "Provide a concise summary of this content.")

	// Fetch content as text (reuse existing logic)
	content, err := httpFetchAndExtractText(ctx, urlStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch content: %v", err)), nil
	}

	// Check if we have a model available
	if httpServerModel == nil {
		return mcp.NewToolResultError("LLM model not available for summarization"), nil
	}

	// Create messages for the LLM
	messages := []*schema.Message{
		schema.UserMessage(fmt.Sprintf("%s\n\nContent to summarize:\n%s", instructions, content)),
	}

	// Generate summary using the model directly
	response, err := httpServerModel.Generate(ctx, messages)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Summarization failed: %v", err)), nil
	}

	// Return summary
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: response.Content,
			},
		},
	}, nil
}

// executeHTTPFetchExtract handles the fetch_extract tool execution
func executeHTTPFetchExtract(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get URL
	urlStr, err := request.RequireString("url")
	if err != nil {
		return mcp.NewToolResultError("url parameter is required and must be a string"), nil
	}

	// Get extraction instructions
	instructions, err := request.RequireString("instructions")
	if err != nil {
		return mcp.NewToolResultError("instructions parameter is required and must be a string"), nil
	}

	// Fetch content as text (reuse existing logic)
	content, err := httpFetchAndExtractText(ctx, urlStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch content: %v", err)), nil
	}

	// Check if we have a model available
	if httpServerModel == nil {
		return mcp.NewToolResultError("LLM model not available for extraction"), nil
	}

	// Create extraction prompt
	extractionPrompt := fmt.Sprintf(`Extract the requested information from the following web content.

Extraction Instructions: %s

Web Content:
%s

Please extract only the requested information. If the requested information is not found, respond with "Information not found" and explain what was searched for.`, instructions, content)

	// Create messages for the LLM
	messages := []*schema.Message{
		schema.UserMessage(extractionPrompt),
	}

	// Generate extraction using the model directly
	response, err := httpServerModel.Generate(ctx, messages)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Extraction failed: %v", err)), nil
	}

	// Return extracted data
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: response.Content,
			},
		},
	}, nil
}

// httpFetchAndExtractText fetches content from URL and extracts as text
func httpFetchAndExtractText(ctx context.Context, urlStr string) (string, error) {
	// Parse timeout (use default)
	timeout := httpDefaultFetchTimeout

	// Validate URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %v", err)
	}

	// Ensure URL has a scheme
	if parsedURL.Scheme == "" {
		urlStr = "https://" + urlStr
		parsedURL, err = url.Parse(urlStr)
		if err != nil {
			return "", fmt.Errorf("invalid URL after adding https: %v", err)
		}
	}

	// Only allow HTTP and HTTPS
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("URL must use http:// or https://")
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: timeout,
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers to mimic a real browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}

	// Check content length
	if resp.ContentLength > httpMaxResponseSize {
		return "", fmt.Errorf("response too large (exceeds 5MB limit)")
	}

	// Read response body with size limit
	limitedReader := io.LimitReader(resp.Body, httpMaxResponseSize+1)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Check if we exceeded the size limit
	if len(bodyBytes) > httpMaxResponseSize {
		return "", fmt.Errorf("response too large (exceeds 5MB limit)")
	}

	content := string(bodyBytes)
	contentType := resp.Header.Get("Content-Type")

	// Extract text content
	if strings.Contains(contentType, "text/html") {
		return httpExtractTextFromHTML(content)
	}
	return content, nil
}

// httpExtractTextFromHTML extracts plain text from HTML content
func httpExtractTextFromHTML(htmlContent string) (string, error) {
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

// executeHTTPFetchFilteredJSON handles the fetch_filtered_json tool execution
func executeHTTPFetchFilteredJSON(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters
	urlStr, err := request.RequireString("url")
	if err != nil {
		return mcp.NewToolResultError("url parameter is required and must be a string"), nil
	}

	path, err := request.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path parameter is required and must be a string"), nil
	}

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

	// Set headers to mimic a real browser and accept JSON
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
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

	// Validate that the content is valid JSON
	if !json.Valid(bodyBytes) {
		return mcp.NewToolResultError("response is not valid JSON"), nil
	}

	// Apply gjson path to filter the JSON
	result := gjson.Get(content, path)
	if !result.Exists() {
		return mcp.NewToolResultError(fmt.Sprintf("gjson path '%s' did not match any data", path)), nil
	}

	// Get the filtered JSON as a string
	var filteredJSON string
	if result.IsArray() || result.IsObject() {
		filteredJSON = result.Raw
	} else {
		// For primitive values, wrap in quotes if it's a string
		if result.Type == gjson.String {
			filteredJSON = fmt.Sprintf(`"%s"`, result.Str)
		} else {
			filteredJSON = result.Raw
		}
	}

	// Create result with metadata
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}

	title := fmt.Sprintf("Filtered JSON from %s (path: %s)", urlStr, path)
	mcpResult := mcp.NewToolResultText(filteredJSON)
	mcpResult.Meta = &mcp.Meta{
		AdditionalFields: map[string]any{
			"title":       title,
			"url":         urlStr,
			"contentType": contentType,
			"gjsonPath":   path,
			"resultType":  result.Type.String(),
		},
	}

	return mcpResult, nil
}

// httpGetTextFromSamplingResult extracts text from sampling result
func httpGetTextFromSamplingResult(result *mcp.CreateMessageResult) string {
	if textContent, ok := result.Content.(mcp.TextContent); ok {
		return textContent.Text
	}
	return fmt.Sprintf("%v", result.Content)
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

const httpSummarizeDescription = `Fetches web content and returns an AI-generated summary using LLM sampling.

- Fetches content from a specified URL using HTTP GET
- Uses the client's LLM to generate an intelligent summary
- Supports custom summarization instructions
- Returns a concise AI-generated summary of the content

Usage notes:
  - Requires a client with sampling capability (LLM access)
  - The URL must be a fully-formed valid URL
  - Content is automatically extracted as text for summarization
  - Default instruction: "Provide a concise summary of this content"
  - Summary is limited to approximately 500 tokens`

const httpExtractDescription = `Fetches web content and extracts specific data or sections using AI-powered extraction.

- Fetches content from a specified URL using HTTP GET
- Uses the client's LLM to extract specific information based on instructions
- Supports flexible extraction instructions for various data types
- Returns only the requested information from the web content

Usage notes:
  - Requires a client with sampling capability (LLM access)
  - The URL must be a fully-formed valid URL
  - Content is automatically extracted as text for processing
  - Instructions should be specific (e.g., "Extract all product names and prices", "Get the main article content", "Find all email addresses")
  - Returns "Information not found" if the requested data is not available
  - Ideal for structured data extraction, content parsing, and targeted information retrieval`

const httpFilterJSONDescription = `Fetches JSON content from a URL and applies gjson path filtering to extract specific data.

- Fetches JSON content from a specified URL using HTTP GET
- Uses gjson path syntax to filter and extract specific parts of the JSON
- Returns filtered JSON results based on the provided path expression
- Supports all gjson features: wildcards, arrays, queries, modifiers, and more

Usage notes:
  - The URL must return valid JSON content
  - Uses gjson path syntax for filtering (see https://github.com/tidwall/gjson/blob/master/SYNTAX.md)
  - Common path examples:
    - "users.#.name" - Get all user names from an array
    - "data.items.0" - Get the first item from data.items array
    - "results.#(age>25).name" - Get names where age > 25
    - "friends.#(last==\"Murphy\")#.first" - Get first names of all Murphys
    - "@reverse" - Reverse an array
    - "users.#.{name,email}" - Create new objects with only name and email
  - Returns error if path doesn't match any data
  - Maximum response size is 5MB
  - Timeout can be specified in seconds (default 30s, max 120s)`
