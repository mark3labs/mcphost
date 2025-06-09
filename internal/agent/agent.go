package agent

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcphost/internal/config"
	"github.com/mark3labs/mcphost/internal/models"
	"github.com/mark3labs/mcphost/internal/tools"
)

type state struct {
	Messages                 []*schema.Message
	ReturnDirectlyToolCallID string
}

const (
	nodeKeyTools = "tools"
	nodeKeyModel = "chat"
)

// MessageModifier modify the input messages before the model is called.
type MessageModifier func(ctx context.Context, input []*schema.Message) []*schema.Message

// AgentConfig is the config for agent.
type AgentConfig struct {
	ModelConfig      *models.ProviderConfig
	MCPConfig        *config.Config
	SystemPrompt     string
	MaxSteps         int
	MessageWindow    int
	
	// MessageModifier.
	// modify the input messages before the model is called, it's useful when you want to add some system prompt or other messages.
	MessageModifier MessageModifier

	// Tools that will make agent return directly when the tool is called.
	// When multiple tools are called and more than one tool is in the return directly list, only the first one will be returned.
	ToolReturnDirectly map[string]struct{}

	// StreamOutputHandler is a function to determine whether the model's streaming output contains tool calls.
	StreamToolCallChecker func(ctx context.Context, modelOutput *schema.StreamReader[*schema.Message]) (bool, error)
}

// ToolCallHandler is a function type for handling tool calls as they happen
type ToolCallHandler func(toolName, toolArgs string)

// ToolResultHandler is a function type for handling tool results
type ToolResultHandler func(toolName, toolArgs, result string, isError bool)

// ResponseHandler is a function type for handling LLM responses
type ResponseHandler func(content string)

// ToolCallContentHandler is a function type for handling content that accompanies tool calls
type ToolCallContentHandler func(content string)

func firstChunkStreamToolCallChecker(_ context.Context, sr *schema.StreamReader[*schema.Message]) (bool, error) {
	defer sr.Close()

	for {
		msg, err := sr.Recv()
		if err == io.EOF {
			return false, nil
		}
		if err != nil {
			return false, err
		}

		if len(msg.ToolCalls) > 0 {
			return true, nil
		}

		if len(msg.Content) == 0 { // skip empty chunks at the front
			continue
		}

		return false, nil
	}
}

const (
	GraphName     = "Agent"
	ModelNodeName = "ChatModel"
	ToolsNodeName = "Tools"
)

// Agent is the agent with real-time tool call display.
type Agent struct {
	runnable         compose.Runnable[[]*schema.Message, *schema.Message]
	graph            *compose.Graph[[]*schema.Message, *schema.Message]
	graphAddNodeOpts []compose.GraphAddNodeOpt
	toolManager      *tools.MCPToolManager
	model            model.ToolCallingChatModel
	maxSteps         int
	systemPrompt     string
}

var registerStateOnce sync.Once

// NewAgent creates an agent with MCP tool integration and real-time tool call display
func NewAgent(ctx context.Context, config *AgentConfig) (*Agent, error) {
	// Create the LLM provider
	model, err := models.CreateProvider(ctx, config.ModelConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create model provider: %v", err)
	}

	// Create and load MCP tools
	toolManager := tools.NewMCPToolManager()
	if err := toolManager.LoadTools(ctx, config.MCPConfig); err != nil {
		return nil, fmt.Errorf("failed to load MCP tools: %v", err)
	}

	var (
		toolsNode       *compose.ToolsNode
		toolInfos       []*schema.ToolInfo
		toolCallChecker = config.StreamToolCallChecker
		messageModifier = config.MessageModifier
	)

	registerStateOnce.Do(func() {
		err = compose.RegisterSerializableType[state]("_eino_agent_state")
	})
	if err != nil {
		return nil, err
	}

	if toolCallChecker == nil {
		toolCallChecker = firstChunkStreamToolCallChecker
	}

	// Create tools config
	toolsConfig := compose.ToolsNodeConfig{
		Tools: toolManager.GetTools(),
	}

	if toolInfos, err = genToolInfos(ctx, toolsConfig); err != nil {
		return nil, err
	}

	chatModel, err := agent.ChatModelWithTools(nil, model, toolInfos)
	if err != nil {
		return nil, err
	}

	if toolsNode, err = compose.NewToolNode(ctx, &toolsConfig); err != nil {
		return nil, err
	}

	maxSteps := config.MaxSteps
	if maxSteps == 0 {
		maxSteps = 20
	}

	graph := compose.NewGraph[[]*schema.Message, *schema.Message](compose.WithGenLocalState(func(ctx context.Context) *state {
		return &state{Messages: make([]*schema.Message, 0, maxSteps+1)}
	}))

	modelPreHandle := func(ctx context.Context, input []*schema.Message, state *state) ([]*schema.Message, error) {
		state.Messages = append(state.Messages, input...)

		// Add system prompt if provided and not already present
		if config.SystemPrompt != "" {
			hasSystemMessage := false
			if len(state.Messages) > 0 && state.Messages[0].Role == schema.System {
				hasSystemMessage = true
			}
			
			if !hasSystemMessage {
				systemMsg := schema.SystemMessage(config.SystemPrompt)
				state.Messages = append([]*schema.Message{systemMsg}, state.Messages...)
			}
		}

		if messageModifier == nil {
			return state.Messages, nil
		}

		modifiedInput := make([]*schema.Message, len(state.Messages))
		copy(modifiedInput, state.Messages)
		return messageModifier(ctx, modifiedInput), nil
	}

	if err = graph.AddChatModelNode(nodeKeyModel, chatModel, compose.WithStatePreHandler(modelPreHandle), compose.WithNodeName(ModelNodeName)); err != nil {
		return nil, err
	}

	if err = graph.AddEdge(compose.START, nodeKeyModel); err != nil {
		return nil, err
	}

	toolsNodePreHandle := func(ctx context.Context, input *schema.Message, state *state) (*schema.Message, error) {
		if input == nil {
			return state.Messages[len(state.Messages)-1], nil // used for rerun interrupt resume
		}
		state.Messages = append(state.Messages, input)
		state.ReturnDirectlyToolCallID = getReturnDirectlyToolCallID(input, config.ToolReturnDirectly)
		return input, nil
	}
	if err = graph.AddToolsNode(nodeKeyTools, toolsNode, compose.WithStatePreHandler(toolsNodePreHandle), compose.WithNodeName(ToolsNodeName)); err != nil {
		return nil, err
	}

	modelPostBranchCondition := func(_ context.Context, sr *schema.StreamReader[*schema.Message]) (endNode string, err error) {
		if isToolCall, err := toolCallChecker(ctx, sr); err != nil {
			return "", err
		} else if isToolCall {
			return nodeKeyTools, nil
		}
		return compose.END, nil
	}

	if err = graph.AddBranch(nodeKeyModel, compose.NewStreamGraphBranch(modelPostBranchCondition, map[string]bool{nodeKeyTools: true, compose.END: true})); err != nil {
		return nil, err
	}

	if len(config.ToolReturnDirectly) > 0 {
		if err = buildReturnDirectly(graph); err != nil {
			return nil, err
		}
	} else if err = graph.AddEdge(nodeKeyTools, nodeKeyModel); err != nil {
		return nil, err
	}

	compileOpts := []compose.GraphCompileOption{compose.WithMaxRunSteps(maxSteps), compose.WithNodeTriggerMode(compose.AnyPredecessor), compose.WithGraphName(GraphName)}
	runnable, err := graph.Compile(ctx, compileOpts...)
	if err != nil {
		return nil, err
	}

	return &Agent{
		runnable:         runnable,
		graph:            graph,
		graphAddNodeOpts: []compose.GraphAddNodeOpt{compose.WithGraphCompileOptions(compileOpts...)},
		toolManager:      toolManager,
		model:            model,
		maxSteps:         maxSteps,
		systemPrompt:     config.SystemPrompt,
	}, nil
}

func buildReturnDirectly(graph *compose.Graph[[]*schema.Message, *schema.Message]) (err error) {
	directReturn := func(ctx context.Context, msgs *schema.StreamReader[[]*schema.Message]) (*schema.StreamReader[*schema.Message], error) {
		return schema.StreamReaderWithConvert(msgs, func(msgs []*schema.Message) (*schema.Message, error) {
			var msg *schema.Message
			err = compose.ProcessState[*state](ctx, func(_ context.Context, state *state) error {
				for i := range msgs {
					if msgs[i] != nil && msgs[i].ToolCallID == state.ReturnDirectlyToolCallID {
						msg = msgs[i]
						return nil
					}
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
			if msg == nil {
				return nil, schema.ErrNoValue
			}
			return msg, nil
		}), nil
	}

	nodeKeyDirectReturn := "direct_return"
	if err = graph.AddLambdaNode(nodeKeyDirectReturn, compose.TransformableLambda(directReturn)); err != nil {
		return err
	}

	// this branch checks if the tool called should return directly. It either leads to END or back to ChatModel
	err = graph.AddBranch(nodeKeyTools, compose.NewStreamGraphBranch(func(ctx context.Context, msgsStream *schema.StreamReader[[]*schema.Message]) (endNode string, err error) {
		msgsStream.Close()

		err = compose.ProcessState[*state](ctx, func(_ context.Context, state *state) error {
			if len(state.ReturnDirectlyToolCallID) > 0 {
				endNode = nodeKeyDirectReturn
			} else {
				endNode = nodeKeyModel
			}
			return nil
		})
		if err != nil {
			return "", err
		}
		return endNode, nil
	}, map[string]bool{nodeKeyModel: true, nodeKeyDirectReturn: true}))
	if err != nil {
		return err
	}

	return graph.AddEdge(nodeKeyDirectReturn, compose.END)
}

func genToolInfos(ctx context.Context, config compose.ToolsNodeConfig) ([]*schema.ToolInfo, error) {
	toolInfos := make([]*schema.ToolInfo, 0, len(config.Tools))
	for _, t := range config.Tools {
		tl, err := t.Info(ctx)
		if err != nil {
			return nil, err
		}

		toolInfos = append(toolInfos, tl)
	}

	return toolInfos, nil
}

func getReturnDirectlyToolCallID(input *schema.Message, toolReturnDirectly map[string]struct{}) string {
	if len(toolReturnDirectly) == 0 {
		return ""
	}

	for _, toolCall := range input.ToolCalls {
		if _, ok := toolReturnDirectly[toolCall.Function.Name]; ok {
			return toolCall.ID
		}
	}

	return ""
}

// Generate generates a response from the agent.
func (a *Agent) Generate(ctx context.Context, input []*schema.Message, opts ...compose.Option) (*schema.Message, error) {
	// Convert compose options to agent options
	agentOpts := []agent.AgentOption{}
	if len(opts) > 0 {
		agentOpts = append(agentOpts, agent.WithComposeOptions(opts...))
	}
	return a.runnable.Invoke(ctx, input, agent.GetComposeOptions(agentOpts...)...)
}

// Stream calls the agent and returns a stream response.
func (a *Agent) Stream(ctx context.Context, input []*schema.Message, opts ...compose.Option) (output *schema.StreamReader[*schema.Message], err error) {
	// Convert compose options to agent options
	agentOpts := []agent.AgentOption{}
	if len(opts) > 0 {
		agentOpts = append(agentOpts, agent.WithComposeOptions(opts...))
	}
	return a.runnable.Stream(ctx, input, agent.GetComposeOptions(agentOpts...)...)
}

// GenerateWithLoop processes messages with a custom loop that displays tool calls in real-time
func (a *Agent) GenerateWithLoop(ctx context.Context, messages []*schema.Message, 
	onToolCall ToolCallHandler, onToolResult ToolResultHandler, onResponse ResponseHandler, onToolCallContent ToolCallContentHandler) (*schema.Message, error) {
	
	// Create a copy of messages to avoid modifying the original
	workingMessages := make([]*schema.Message, len(messages))
	copy(workingMessages, messages)

	// Add system prompt if provided
	if a.systemPrompt != "" {
		hasSystemMessage := false
		if len(workingMessages) > 0 && workingMessages[0].Role == schema.System {
			hasSystemMessage = true
		}
		
		if !hasSystemMessage {
			systemMsg := schema.SystemMessage(a.systemPrompt)
			workingMessages = append([]*schema.Message{systemMsg}, workingMessages...)
		}
	}

	// Get available tools
	availableTools := a.toolManager.GetTools()
	var toolInfos []*schema.ToolInfo
	toolMap := make(map[string]tool.BaseTool)
	
	for _, t := range availableTools {
		info, err := t.Info(ctx)
		if err != nil {
			continue
		}
		toolInfos = append(toolInfos, info)
		toolMap[info.Name] = t
	}

	// Main loop
	for step := 0; step < a.maxSteps; step++ {
		// Call the LLM
		response, err := a.model.Generate(ctx, workingMessages, model.WithTools(toolInfos))
		if err != nil {
			return nil, fmt.Errorf("failed to generate response: %v", err)
		}

		// Add response to working messages
		workingMessages = append(workingMessages, response)

		// Check if this is a tool call or final response
		if len(response.ToolCalls) > 0 {
			// Display any content that accompanies the tool calls
			if response.Content != "" && onToolCallContent != nil {
				onToolCallContent(response.Content)
			}
			
			// Handle tool calls
			for _, toolCall := range response.ToolCalls {
				// Notify about tool call
				if onToolCall != nil {
					onToolCall(toolCall.Function.Name, toolCall.Function.Arguments)
				}

				// Execute the tool
				if selectedTool, exists := toolMap[toolCall.Function.Name]; exists {
					output, err := selectedTool.(tool.InvokableTool).InvokableRun(ctx, toolCall.Function.Arguments)
					if err != nil {
						errorMsg := fmt.Sprintf("Tool execution error: %v", err)
						toolMessage := schema.ToolMessage(errorMsg, toolCall.ID)
						workingMessages = append(workingMessages, toolMessage)
						
						if onToolResult != nil {
							onToolResult(toolCall.Function.Name, toolCall.Function.Arguments, errorMsg, true)
						}
					} else {
						toolMessage := schema.ToolMessage(output, toolCall.ID)
						workingMessages = append(workingMessages, toolMessage)
						
						if onToolResult != nil {
							onToolResult(toolCall.Function.Name, toolCall.Function.Arguments, output, false)
						}
					}
				} else {
					errorMsg := fmt.Sprintf("Tool not found: %s", toolCall.Function.Name)
					toolMessage := schema.ToolMessage(errorMsg, toolCall.ID)
					workingMessages = append(workingMessages, toolMessage)
					
					if onToolResult != nil {
						onToolResult(toolCall.Function.Name, toolCall.Function.Arguments, errorMsg, true)
					}
				}
			}
		} else {
			// This is a final response
			if onResponse != nil && response.Content != "" {
				onResponse(response.Content)
			}
			return response, nil
		}
	}

	// If we reach here, we've exceeded max steps
	return schema.AssistantMessage("Maximum number of steps reached.", nil), nil
}

// GetTools returns the list of available tools
func (a *Agent) GetTools() []tool.BaseTool {
	return a.toolManager.GetTools()
}

// Close closes the agent and cleans up resources
func (a *Agent) Close() error {
	return a.toolManager.Close()
}

// ExportGraph exports the underlying graph from Agent, along with the []compose.GraphAddNodeOpt to be used when adding this graph to another graph.
func (a *Agent) ExportGraph() (compose.AnyGraph, []compose.GraphAddNodeOpt) {
	return a.graph, a.graphAddNodeOpts
}