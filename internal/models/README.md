# Custom Gemini Tool Calling Model Implementation

This directory contains a custom implementation of the `ToolCallingChatModel` interface for Google's Gemini API, built using the official `google/generative-ai-go` package.

## Features

- **Direct Gemini API Integration**: Uses the official Google Generative AI Go client
- **Tool Calling Support**: Full support for function calling with Gemini models
- **Streaming Support**: Implements both synchronous and streaming generation
- **Schema Conversion**: Automatically converts eino tool schemas to Gemini format
- **Thread-Safe**: Implements the `WithTools` pattern for safe concurrent usage

## Files

- `gemini_custom.go` - Main implementation of the custom Gemini model
- `providers.go` - Updated to support the custom implementation via `UseCustomGemini` flag
- `examples/custom_gemini_example.go` - Example usage of the custom implementation

## Usage

### Basic Usage

```go
import (
    "context"
    "github.com/mark3labs/mcphost/internal/models"
    "github.com/cloudwego/eino/schema"
)

ctx := context.Background()

// Create custom Gemini model
model, err := models.NewGeminiToolCallingModel(ctx, "your-api-key", "gemini-2.0-flash")
if err != nil {
    log.Fatal(err)
}

// Define tools
tools := []*schema.ToolInfo{
    {
        Name: "calculator",
        Desc: "Perform arithmetic operations",
        ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
            "operation": {
                Type:     schema.String,
                Desc:     "The operation to perform",
                Required: true,
            },
            "a": {
                Type:     schema.Number,
                Desc:     "First number",
                Required: true,
            },
            "b": {
                Type:     schema.Number,
                Desc:     "Second number",
                Required: true,
            },
        }),
    },
}

// Bind tools to model
modelWithTools, err := model.WithTools(tools)
if err != nil {
    log.Fatal(err)
}

// Generate response
messages := []*schema.Message{
    schema.UserMessage("What is 15 + 27?"),
}

response, err := modelWithTools.Generate(ctx, messages)
if err != nil {
    log.Fatal(err)
}
```

### Using with Provider Configuration

```go
config := &models.ProviderConfig{
    ModelString:     "google:gemini-2.0-flash",
    GoogleAPIKey:    "your-api-key",
    UseCustomGemini: true, // Enable custom implementation
}

model, err := models.CreateProvider(ctx, config)
```

### Streaming

```go
streamReader, err := modelWithTools.Stream(ctx, messages)
if err != nil {
    log.Fatal(err)
}

for {
    message, err := streamReader.Recv()
    if err != nil {
        if errors.Is(err, io.EOF) {
            break
        }
        log.Fatal(err)
    }
    
    fmt.Printf("Chunk: %s\n", message.Content)
    
    if len(message.ToolCalls) > 0 {
        for _, toolCall := range message.ToolCalls {
            fmt.Printf("Tool Call: %s(%s)\n", toolCall.Function.Name, toolCall.Function.Arguments)
        }
    }
}
```

## Implementation Details

### Schema Conversion

The implementation automatically converts eino tool schemas to Gemini's format:

1. **Tool Info → Function Declaration**: Converts `schema.ToolInfo` to `genai.FunctionDeclaration`
2. **Parameters → Schema**: Converts eino parameter definitions to Gemini schema format
3. **OpenAPI Support**: Uses the eino `ToOpenAPIV3()` method for complex schema conversion

### Message Handling

- **Role Mapping**: Maps eino roles (User, Assistant, System, Tool) to Gemini roles
- **Tool Calls**: Converts between eino `ToolCall` format and Gemini `FunctionCall`
- **Tool Responses**: Handles tool response messages via `ToolCallID` and `ToolName`

### Error Handling

- Comprehensive error handling with descriptive messages
- Graceful fallbacks for missing or invalid schemas
- Proper stream error propagation

## API Key Configuration

The implementation supports multiple ways to provide the API key:

1. **Direct parameter**: Pass to `NewGeminiToolCallingModel(ctx, "api-key", "model")`
2. **Environment variables**: `GEMINI_API_KEY` or `GOOGLE_API_KEY`
3. **Provider config**: Via `ProviderConfig.GoogleAPIKey`

## Supported Models

Works with all Gemini models that support function calling:
- `gemini-2.0-flash`
- `gemini-1.5-pro`
- `gemini-1.5-flash`

## Comparison with eino-ext Implementation

| Feature | Custom Implementation | eino-ext Implementation |
|---------|----------------------|------------------------|
| Direct API Control | ✅ Full control | ❌ Abstracted |
| Custom Configuration | ✅ Easy to modify | ❌ Limited |
| Latest API Features | ✅ Always up-to-date | ❌ Depends on updates |
| Debugging | ✅ Full visibility | ❌ Black box |
| Dependencies | ✅ Minimal | ❌ Additional layers |

## Example Output

When running the example with a tool calling request:

```
Response: I'll help you add those numbers together.

Tool Call: add_numbers with args: {"a":15,"b":27}
```

## Contributing

To extend or modify the custom implementation:

1. Update `gemini_custom.go` for core functionality
2. Modify `providers.go` to change provider integration
3. Add tests in a new `gemini_custom_test.go` file
4. Update examples for new features

## References

- [Google Generative AI Go Client](https://github.com/google/generative-ai-go)
- [Gemini Function Calling Examples](https://github.com/google-gemini/api-examples/tree/main/go)
- [eino Framework](https://github.com/cloudwego/eino)