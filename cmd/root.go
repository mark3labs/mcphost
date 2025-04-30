package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/log"

	"github.com/charmbracelet/glamour"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcphost/pkg/history"
	"github.com/mark3labs/mcphost/pkg/llm"
	"github.com/mark3labs/mcphost/pkg/llm/anthropic"
	"github.com/mark3labs/mcphost/pkg/llm/google"
	"github.com/mark3labs/mcphost/pkg/llm/ollama"
	"github.com/mark3labs/mcphost/pkg/llm/openai"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	renderer         *glamour.TermRenderer
	configFile       string
	messageWindow    int
	modelFlag        string // New flag for model selection
	openaiBaseURL    string // Base URL for OpenAI API
	anthropicBaseURL string // Base URL for Anthropic API
	openaiAPIKey     string
	anthropicAPIKey  string
	googleAPIKey     string
	serverPort       string // HTTP server port
)

const (
	initialBackoff = 1 * time.Second
	maxBackoff     = 30 * time.Second
	maxRetries     = 5 // Will reach close to max backoff
)

var rootCmd = &cobra.Command{
	Use:   "mcphost",
	Short: "Chat with AI models through a unified interface",
	Long: `MCPHost is a CLI tool that allows you to interact with various AI models
through a unified interface. It supports various tools through MCP servers
and provides streaming responses.

Available models can be specified using the --model flag:
- Anthropic Claude (default): anthropic:claude-3-5-sonnet-latest
- OpenAI: openai:gpt-4
- Ollama models: ollama:modelname
- Google: google:modelname

Example:
  mcphost -m ollama:qwen2.5:3b
  mcphost -m openai:gpt-4
  mcphost -m google:gemini-2.0-flash`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPHost(context.Background())
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var debugMode bool

func init() {
	rootCmd.PersistentFlags().
		StringVar(&configFile, "config", "", "config file (default is $HOME/.mcp.json)")
	rootCmd.PersistentFlags().
		IntVar(&messageWindow, "message-window", 10, "number of messages to keep in context")
	rootCmd.PersistentFlags().
		StringVarP(&modelFlag, "model", "m", "anthropic:claude-3-5-sonnet-latest",
			"model to use (format: provider:model, e.g. anthropic:claude-3-5-sonnet-latest or ollama:qwen2.5:3b)")
	rootCmd.PersistentFlags().
		StringVarP(&serverPort, "port", "p", "9090", "port for the HTTP server")

	// Add debug flag
	rootCmd.PersistentFlags().
		BoolVar(&debugMode, "debug", false, "enable debug logging")

	flags := rootCmd.PersistentFlags()
	flags.StringVar(&openaiBaseURL, "openai-url", "", "base URL for OpenAI API (defaults to api.openai.com)")
	flags.StringVar(&anthropicBaseURL, "anthropic-url", "", "base URL for Anthropic API (defaults to api.anthropic.com)")
	flags.StringVar(&openaiAPIKey, "openai-api-key", "", "OpenAI API key")
	flags.StringVar(&anthropicAPIKey, "anthropic-api-key", "", "Anthropic API key")
	flags.StringVar(&googleAPIKey, "google-api-key", "", "Google (Gemini) API key")
}

// Add new function to create provider
func createProvider(ctx context.Context, modelString string) (llm.Provider, error) {
	parts := strings.SplitN(modelString, ":", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf(
			"invalid model format. Expected provider:model, got %s",
			modelString,
		)
	}

	provider := parts[0]
	model := parts[1]

	switch provider {
	case "anthropic":
		apiKey := anthropicAPIKey
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}

		if apiKey == "" {
			return nil, fmt.Errorf(
				"Anthropic API key not provided. Use --anthropic-api-key flag or ANTHROPIC_API_KEY environment variable",
			)
		}
		return anthropic.NewProvider(apiKey, anthropicBaseURL, model), nil

	case "ollama":
		return ollama.NewProvider(model)

	case "openai":
		apiKey := openaiAPIKey
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}

		if apiKey == "" {
			return nil, fmt.Errorf(
				"OpenAI API key not provided. Use --openai-api-key flag or OPENAI_API_KEY environment variable",
			)
		}
		return openai.NewProvider(apiKey, openaiBaseURL, model), nil

	case "google":
		apiKey := googleAPIKey
		if apiKey == "" {
			apiKey = os.Getenv("GOOGLE_API_KEY")
		}
		if apiKey == "" {
			// The project structure is provider specific, but Google calls this GEMINI_API_KEY in e.g. AI Studio. Support both.
			apiKey = os.Getenv("GEMINI_API_KEY")
		}
		return google.NewProvider(ctx, apiKey, model)

	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func pruneMessages(messages []history.HistoryMessage) []history.HistoryMessage {
	if len(messages) <= messageWindow {
		return messages
	}

	// Keep only the most recent messages based on window size
	messages = messages[len(messages)-messageWindow:]

	// Handle messages
	toolUseIds := make(map[string]bool)
	toolResultIds := make(map[string]bool)

	// First pass: collect all tool use and result IDs
	for _, msg := range messages {
		for _, block := range msg.Content {
			if block.Type == "tool_use" {
				toolUseIds[block.ID] = true
			} else if block.Type == "tool_result" {
				toolResultIds[block.ToolUseID] = true
			}
		}
	}

	// Second pass: filter out orphaned tool calls/results
	var prunedMessages []history.HistoryMessage
	for _, msg := range messages {
		var prunedBlocks []history.ContentBlock
		for _, block := range msg.Content {
			keep := true
			if block.Type == "tool_use" {
				keep = toolResultIds[block.ID]
			} else if block.Type == "tool_result" {
				keep = toolUseIds[block.ToolUseID]
			}
			if keep {
				prunedBlocks = append(prunedBlocks, block)
			}
		}
		// Only include messages that have content or are not assistant messages
		if (len(prunedBlocks) > 0 && msg.Role == "assistant") ||
			msg.Role != "assistant" {
			hasTextBlock := false
			for _, block := range msg.Content {
				if block.Type == "text" {
					hasTextBlock = true
					break
				}
			}
			if len(prunedBlocks) > 0 || hasTextBlock {
				msg.Content = prunedBlocks
				prunedMessages = append(prunedMessages, msg)
			}
		}
	}
	return prunedMessages
}

func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 80 // Fallback width
	}
	return width - 20
}

func handleHistoryCommand(messages []history.HistoryMessage) {
	displayMessageHistory(messages)
}

func updateRenderer() error {
	width := getTerminalWidth()
	var err error
	renderer, err = glamour.NewTermRenderer(
		glamour.WithStandardStyle(styles.TokyoNightStyle),
		glamour.WithWordWrap(width),
	)
	return err
}

// MessageResponse represents the response structure for API clients
type MessageResponse struct {
	ID          string                   `json:"id"`
	Content     string                   `json:"content"`
	Role        string                   `json:"role"`
	ToolCalls   []map[string]interface{} `json:"tool_calls,omitempty"`
	ToolResults []map[string]interface{} `json:"tool_results,omitempty"`
	Usage       map[string]int           `json:"usage,omitempty"`
	Error       string                   `json:"error,omitempty"`
}

// ChatRequest represents the incoming chat message request
type ChatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id,omitempty"`
}

// Method implementations for API
func runPrompt(
	ctx context.Context,
	provider llm.Provider,
	mcpClients map[string]mcpclient.MCPClient,
	tools []llm.Tool,
	prompt string,
	messages *[]history.HistoryMessage,
) ([]MessageResponse, error) {
	var responses []MessageResponse

	// Add the user's prompt to messages if it's not empty
	if prompt != "" {
		*messages = append(
			*messages,
			history.HistoryMessage{
				Role: "user",
				Content: []history.ContentBlock{{
					Type: "text",
					Text: prompt,
				}},
			},
		)
	}

	var message llm.Message
	var err error
	backoff := initialBackoff
	retries := 0

	// Convert MessageParam to llm.Message for provider
	// Messages already implement llm.Message interface
	llmMessages := make([]llm.Message, len(*messages))
	for i := range *messages {
		llmMessages[i] = &(*messages)[i]
	}

	for {
		message, err = provider.CreateMessage(
			ctx,
			prompt,
			llmMessages,
			tools,
		)

		if err != nil {
			// Check if it's an overloaded error
			if strings.Contains(err.Error(), "overloaded_error") {
				if retries >= maxRetries {
					return nil, fmt.Errorf(
						"model is currently overloaded. please wait a few minutes and try again",
					)
				}

				log.Warn("Model API is overloaded, backing off...",
					"attempt", retries+1,
					"backoff", backoff.String())

				time.Sleep(backoff)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				retries++
				continue
			}
			// If it's not an overloaded error, return the error immediately
			return nil, err
		}
		// If we got here, the request succeeded
		break
	}

	var messageContent []history.ContentBlock
	var toolResults []history.ContentBlock

	// Create response object
	inputTokens, outputTokens := message.GetUsage()
	response := MessageResponse{
		ID:      fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		Content: message.GetContent(),
		Role:    message.GetRole(),
		Usage: map[string]int{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
		},
	}

	// Add text content
	if message.GetContent() != "" {
		messageContent = append(messageContent, history.ContentBlock{
			Type: "text",
			Text: message.GetContent(),
		})
	}

	// Handle tool calls
	for _, toolCall := range message.GetToolCalls() {
		log.Info("🔧 Using tool", "name", toolCall.GetName())

		input, _ := json.Marshal(toolCall.GetArguments())
		messageContent = append(messageContent, history.ContentBlock{
			Type:  "tool_use",
			ID:    toolCall.GetID(),
			Name:  toolCall.GetName(),
			Input: input,
		})

		// Add tool call to response
		toolCallMap := map[string]interface{}{
			"id":        toolCall.GetID(),
			"name":      toolCall.GetName(),
			"arguments": toolCall.GetArguments(),
		}
		response.ToolCalls = append(response.ToolCalls, toolCallMap)

		parts := strings.Split(toolCall.GetName(), "__")
		if len(parts) != 2 {
			errMsg := fmt.Sprintf("Invalid tool name format: %s", toolCall.GetName())
			log.Error(errMsg)

			toolResults = append(toolResults, history.ContentBlock{
				Type:      "tool_result",
				ToolUseID: toolCall.GetID(),
				Content: []history.ContentBlock{{
					Type: "text",
					Text: errMsg,
				}},
			})

			response.ToolResults = append(response.ToolResults, map[string]interface{}{
				"tool_call_id": toolCall.GetID(),
				"content":      errMsg,
				"error":        true,
			})
			continue
		}

		serverName, toolName := parts[0], parts[1]
		mcpClient, ok := mcpClients[serverName]
		if !ok {
			errMsg := fmt.Sprintf("Server not found: %s", serverName)
			log.Error(errMsg)

			toolResults = append(toolResults, history.ContentBlock{
				Type:      "tool_result",
				ToolUseID: toolCall.GetID(),
				Content: []history.ContentBlock{{
					Type: "text",
					Text: errMsg,
				}},
			})

			response.ToolResults = append(response.ToolResults, map[string]interface{}{
				"tool_call_id": toolCall.GetID(),
				"content":      errMsg,
				"error":        true,
			})
			continue
		}

		var toolArgs map[string]interface{}
		if err := json.Unmarshal(input, &toolArgs); err != nil {
			errMsg := fmt.Sprintf("Error parsing tool arguments: %v", err)
			log.Error(errMsg)

			toolResults = append(toolResults, history.ContentBlock{
				Type:      "tool_result",
				ToolUseID: toolCall.GetID(),
				Content: []history.ContentBlock{{
					Type: "text",
					Text: errMsg,
				}},
			})

			response.ToolResults = append(response.ToolResults, map[string]interface{}{
				"tool_call_id": toolCall.GetID(),
				"content":      errMsg,
				"error":        true,
			})
			continue
		}

		toolArgs["skp-authorization"] = fmt.Sprintf("Bearer %s", "token-some"+os.Getenv("MCP_AUTH_TOKEN"))
		req := mcp.CallToolRequest{}
		req.Params.Name = toolName
		req.Params.Arguments = toolArgs
		// add auth token to the request
		authCtx := context.WithValue(ctx, "mcp.AuthTokenKey", os.Getenv("MCP_AUTH_TOKEN"))
		toolResultPtr, err := mcpClient.CallTool(authCtx, req)

		if err != nil {
			errMsg := fmt.Sprintf("Error calling tool %s: %v", toolName, err)
			log.Error(errMsg)

			toolResults = append(toolResults, history.ContentBlock{
				Type:      "tool_result",
				ToolUseID: toolCall.GetID(),
				Content: []history.ContentBlock{{
					Type: "text",
					Text: errMsg,
				}},
			})

			response.ToolResults = append(response.ToolResults, map[string]interface{}{
				"tool_call_id": toolCall.GetID(),
				"content":      errMsg,
				"error":        true,
			})
			continue
		}

		toolResult := *toolResultPtr

		if toolResult.Content != nil {
			log.Debug("raw tool result content", "content", toolResult.Content)

			// Create the tool result block
			resultBlock := history.ContentBlock{
				Type:      "tool_result",
				ToolUseID: toolCall.GetID(),
				Content:   toolResult.Content,
			}

			// Extract text content
			var resultText string
			// Handle array content directly since we know it's []interface{}
			for _, item := range toolResult.Content {
				if contentMap, ok := item.(mcp.TextContent); ok {
					resultText += fmt.Sprintf("%v ", contentMap.Text)
				}
			}

			resultBlock.Text = strings.TrimSpace(resultText)
			log.Debug("created tool result block",
				"block", resultBlock,
				"tool_id", toolCall.GetID())

			toolResults = append(toolResults, resultBlock)

			// Add result to response
			response.ToolResults = append(response.ToolResults, map[string]interface{}{
				"tool_call_id": toolCall.GetID(),
				"content":      resultBlock.Text,
				"raw_content":  toolResult.Content,
				"error":        false,
			})
		}
	}

	// Add the assistant message to history
	*messages = append(*messages, history.HistoryMessage{
		Role:    message.GetRole(),
		Content: messageContent,
	})

	// Add initial response to responses array
	responses = append(responses, response)

	// If we have tool results, add them to messages and get a follow-up response
	if len(toolResults) > 0 {
		*messages = append(*messages, history.HistoryMessage{
			Role:    "user",
			Content: toolResults,
		})

		// Get follow-up response to the tool results
		followupResponses, err := runPrompt(ctx, provider, mcpClients, tools, "", messages)
		if err != nil {
			return responses, err
		}

		// Append follow-up responses
		responses = append(responses, followupResponses...)
	}

	return responses, nil
}

// Sessions store
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string][]history.HistoryMessage
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string][]history.HistoryMessage),
	}
}

func (s *SessionStore) GetMessages(sessionID string) []history.HistoryMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages, ok := s.sessions[sessionID]
	if !ok {
		return []history.HistoryMessage{}
	}
	return messages
}

func (s *SessionStore) SetMessages(sessionID string, messages []history.HistoryMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if messages == nil {
		messages = []history.HistoryMessage{}
	}
	s.sessions[sessionID] = messages
}

func runMCPHost(ctx context.Context) error {
	// Set up logging based on debug flag
	if debugMode {
		log.SetLevel(log.DebugLevel)
		// Enable caller information for debug logs
		log.SetReportCaller(true)
	} else {
		log.SetLevel(log.InfoLevel)
		log.SetReportCaller(false)
	}

	// Create the provider based on the model flag
	provider, err := createProvider(ctx, modelFlag)
	if err != nil {
		return fmt.Errorf("error creating provider: %v", err)
	}

	// Split the model flag and get just the model name
	parts := strings.SplitN(modelFlag, ":", 2)
	log.Info("Model loaded",
		"provider", provider.Name(),
		"model", parts[1])

	mcpConfig, err := loadMCPConfig()
	if err != nil {
		return fmt.Errorf("error loading MCP config: %v", err)
	}

	mcpClients, err := createMCPClients(mcpConfig)
	if err != nil {
		return fmt.Errorf("error creating MCP clients: %v", err)
	}

	defer func() {
		log.Info("Shutting down MCP servers...")
		for name, client := range mcpClients {
			if err := client.Close(); err != nil {
				log.Error("Failed to close server", "name", name, "error", err)
			} else {
				log.Info("Server closed", "name", name)
			}
		}
	}()

	for name := range mcpClients {
		log.Info("Server connected", "name", name)
	}

	var allTools []llm.Tool
	for serverName, mcpClient := range mcpClients {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		toolsResult, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
		cancel()

		if err != nil {
			log.Error(
				"Error fetching tools",
				"server",
				serverName,
				"error",
				err,
			)
			continue
		}

		serverTools := mcpToolsToAnthropicTools(serverName, toolsResult.Tools)
		allTools = append(allTools, serverTools...)
		log.Info(
			"Tools loaded",
			"server",
			serverName,
			"count",
			len(toolsResult.Tools),
		)
	}

	if err := updateRenderer(); err != nil {
		return fmt.Errorf("error initializing renderer: %v", err)
	}

	// Create session store
	sessionStore := NewSessionStore()

	// Set up HTTP server
	mux := http.NewServeMux()

	// Define routes

	// Health check endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"model":  modelFlag,
		})
	})

	// Chat API endpoint
	mux.HandleFunc("POST /api/chat", func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("Invalid request: %v", err),
			})
			return
		}

		// Generate a session ID if not provided
		sessionID := req.SessionID
		if sessionID == "" {
			sessionID = fmt.Sprintf("session_%d", time.Now().UnixNano())
		}

		// Get messages for this session
		messages := sessionStore.GetMessages(sessionID)

		// Process the message
		responses, err := runPrompt(r.Context(), provider, mcpClients, allTools, req.Message, &messages)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("Error processing message: %v", err),
			})
			return
		}

		// Prune and save the updated messages
		if len(messages) > 0 {
			messages = pruneMessages(messages)
		}
		sessionStore.SetMessages(sessionID, messages)

		// Prepare response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return response with session ID
		response := map[string]interface{}{
			"session_id": sessionID,
			"responses":  responses,
		}
		json.NewEncoder(w).Encode(response)
	})

	// Tools info endpoint
	mux.HandleFunc("GET /api/tools", func(w http.ResponseWriter, r *http.Request) {
		tools := make(map[string][]map[string]interface{})

		for serverName, mcpClient := range mcpClients {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			toolsResult, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
			if err != nil {
				continue
			}

			serverTools := make([]map[string]interface{}, 0)
			for _, tool := range toolsResult.Tools {
				serverTools = append(serverTools, map[string]interface{}{
					"name":         tool.Name,
					"description":  tool.Description,
					"input_schema": tool.InputSchema,
				})
			}
			tools[serverName] = serverTools
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tools)
	})

	// Start the HTTP server
	log.Info("Starting HTTP server", "port", serverPort)
	serverAddr := fmt.Sprintf(":%s", serverPort)
	return http.ListenAndServe(serverAddr, mux)
}
