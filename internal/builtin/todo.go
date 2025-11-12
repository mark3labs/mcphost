package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TodoInfo represents a single todo item with content, status, priority, and ID.
// Status can be "pending", "in_progress", or "completed". Priority can be "high",
// "medium", or "low". Each todo must have a unique ID.
type TodoInfo struct {
	Content  string `json:"content"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
	ID       string `json:"id"`
}

// TodoServer implements a todo management MCP server with in-memory storage.
// It provides thread-safe operations for reading and writing todo lists, with
// support for task status tracking and priority levels.
type TodoServer struct {
	todos []TodoInfo
	mutex sync.RWMutex
}

// NewTodoServer creates a new MCP server that provides todo list management capabilities.
// The server includes two tools: "todowrite" for updating the todo list and "todoread"
// for retrieving the current list. Todos are stored in memory and not persisted.
// Returns an error if server initialization fails.
func NewTodoServer() (*server.MCPServer, error) {
	todoServer := &TodoServer{
		todos: make([]TodoInfo, 0),
	}

	s := server.NewMCPServer("todo-server", "1.0.0", server.WithToolCapabilities(true))

	// Register todowrite tool
	todoWriteTool := mcp.NewTool("todowrite",
		mcp.WithDescription(todoWriteDescription),
		mcp.WithArray("todos",
			mcp.Required(),
			mcp.Description("The updated todo list"),
			mcp.Items(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"content": map[string]any{
						"type":        "string",
						"minLength":   1,
						"description": "Brief description of the task",
					},
					"status": map[string]any{
						"type":        "string",
						"enum":        []string{"pending", "in_progress", "completed"},
						"description": "Current status of the task",
					},
					"priority": map[string]any{
						"type":        "string",
						"enum":        []string{"high", "medium", "low"},
						"description": "Priority level of the task",
					},
					"id": map[string]any{
						"type":        "string",
						"description": "Unique identifier for the todo item",
					},
				},
				"required": []string{"content", "status", "priority", "id"},
			}),
		),
	)

	// Register todoread tool
	todoReadTool := mcp.NewTool("todoread",
		mcp.WithDescription("Use this tool to read your todo list"),
	)

	s.AddTool(todoWriteTool, todoServer.executeTodoWrite)
	s.AddTool(todoReadTool, todoServer.executeTodoRead)

	return s, nil
}

// getTodos retrieves all todos from memory
func (ts *TodoServer) getTodos() []TodoInfo {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	// Return a copy to avoid race conditions
	todos := make([]TodoInfo, len(ts.todos))
	copy(todos, ts.todos)
	return todos
}

// formatTodos returns a nice readable format for todos
func formatTodos(todos []TodoInfo) string {
	if len(todos) == 0 {
		return "\n\nNo todos"
	}

	var result strings.Builder
	result.WriteString("\n\n")
	for _, todo := range todos {
		var checkbox string
		switch todo.Status {
		case "completed":
			checkbox = "[X]"
		case "in_progress":
			checkbox = "[~]"
		default: // pending
			checkbox = "[ ]"
		}
		result.WriteString(fmt.Sprintf("%s %s\n", checkbox, todo.Content))
	}

	// Remove trailing newline
	output := result.String()
	if len(output) > 0 {
		output = output[:len(output)-1]
	}

	return output
}

// setTodos stores todos in memory
func (ts *TodoServer) setTodos(todos []TodoInfo) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	ts.todos = make([]TodoInfo, len(todos))
	copy(ts.todos, todos)
}

// executeTodoWrite handles the todowrite tool execution
func (ts *TodoServer) executeTodoWrite(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse todos from arguments
	todosArg := request.GetArguments()["todos"]
	if todosArg == nil {
		return mcp.NewToolResultError("todos parameter is required"), nil
	}

	// Convert to JSON and back to ensure proper structure
	todosJSON, err := json.Marshal(todosArg)
	if err != nil {
		return mcp.NewToolResultError("invalid todos format"), nil
	}

	var todos []TodoInfo
	if err := json.Unmarshal(todosJSON, &todos); err != nil {
		return mcp.NewToolResultError("invalid todos structure"), nil
	}

	// Validate todos
	for i, todo := range todos {
		if strings.TrimSpace(todo.Content) == "" {
			return mcp.NewToolResultError(fmt.Sprintf("todo %d: content cannot be empty", i)), nil
		}
		if todo.Status != "pending" && todo.Status != "in_progress" && todo.Status != "completed" {
			return mcp.NewToolResultError(fmt.Sprintf("todo %d: invalid status '%s'", i, todo.Status)), nil
		}
		if todo.Priority != "high" && todo.Priority != "medium" && todo.Priority != "low" {
			return mcp.NewToolResultError(fmt.Sprintf("todo %d: invalid priority '%s'", i, todo.Priority)), nil
		}
		if strings.TrimSpace(todo.ID) == "" {
			return mcp.NewToolResultError(fmt.Sprintf("todo %d: id cannot be empty", i)), nil
		}
	}

	// Store todos in memory
	ts.setTodos(todos)

	// Format output in readable format
	output := formatTodos(todos)

	// Create result with formatted output
	result := mcp.NewToolResultText(output)
	result.Meta = &mcp.Meta{
		AdditionalFields: map[string]any{
			"todos": todos,
		},
	}

	return result, nil
}

// executeTodoRead handles the todoread tool execution
func (ts *TodoServer) executeTodoRead(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get todos from memory
	todos := ts.getTodos()

	// Format output in readable format
	output := formatTodos(todos)

	// Create result with formatted output
	result := mcp.NewToolResultText(output)
	result.Meta = &mcp.Meta{
		AdditionalFields: map[string]any{
			"todos": todos,
		},
	}

	return result, nil
}

const todoWriteDescription = `Use this tool to create and manage a structured task list for your current coding session. This helps you track progress, organize complex tasks, and demonstrate thoroughness to the user.
It also helps the user understand the progress of the task and overall progress of their requests.

## When to Use This Tool
Use this tool proactively in these scenarios:

1. Complex multi-step tasks - When a task requires 3 or more distinct steps or actions
2. Non-trivial and complex tasks - Tasks that require careful planning or multiple operations
3. User explicitly requests todo list - When the user directly asks you to use the todo list
4. User provides multiple tasks - When users provide a list of things to be done (numbered or comma-separated)
5. After receiving new instructions - Immediately capture user requirements as todos. Feel free to edit the todo list based on new information.
6. After completing a task - Mark it complete and add any new follow-up tasks
7. When you start working on a new task, mark the todo as in_progress. Ideally you should only have one todo as in_progress at a time. Complete existing tasks before starting new ones.

## When NOT to Use This Tool

Skip using this tool when:
1. There is only a single, straightforward task
2. The task is trivial and tracking it provides no organizational benefit
3. The task can be completed in less than 3 trivial steps
4. The task is purely conversational or informational

NOTE that you should not use this tool if there is only one trivial task to do. In this case you are better off just doing the task directly.

## Task States and Management

1. **Task States**: Use these states to track progress:
   - pending: Task not yet started
   - in_progress: Currently working on (limit to ONE task at a time)
   - completed: Task finished successfully

2. **Task Management**:
   - Update task status in real-time as you work
   - Mark tasks complete IMMEDIATELY after finishing (don't batch completions)
   - Only have ONE task in_progress at any time
   - Complete current tasks before starting new ones

3. **Task Breakdown**:
   - Create specific, actionable items
   - Break complex tasks into smaller, manageable steps
   - Use clear, descriptive task names

When in doubt, use this tool. Being proactive with task management demonstrates attentiveness and ensures you complete all requirements successfully.`
