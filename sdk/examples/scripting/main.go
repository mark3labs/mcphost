package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcphost/sdk"
)

func main() {
	ctx := context.Background()

	// Create MCPHost with environment variable for API key
	// Expects ANTHROPIC_API_KEY or appropriate provider key to be set
	host, err := sdk.New(ctx, &sdk.Options{
		Quiet: true, // Suppress debug output for scripting
	})
	if err != nil {
		log.Fatal(err)
	}
	defer host.Close()

	// Process command line arguments
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go \"your prompt here\"")
		os.Exit(1)
	}

	prompt := os.Args[1]

	// Send prompt and get response
	response, err := host.Prompt(ctx, prompt)
	if err != nil {
		log.Fatal(err)
	}

	// Output only the response (useful for piping)
	fmt.Println(response)
}
