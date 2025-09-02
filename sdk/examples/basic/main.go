package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mark3labs/mcphost/sdk"
)

func main() {
	ctx := context.Background()

	// Example 1: Use all defaults (loads ~/.mcphost.yml)
	fmt.Println("=== Example 1: Default configuration ===")
	host, err := sdk.New(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer host.Close()

	response, err := host.Prompt(ctx, "What is 2+2?")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Response: %s\n\n", response)

	// Example 2: Override model
	fmt.Println("=== Example 2: Custom model ===")
	host2, err := sdk.New(ctx, &sdk.Options{
		Model: "ollama:qwen3:8b",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer host2.Close()

	response, err = host2.Prompt(ctx, "Tell me a short joke")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Response: %s\n\n", response)

	// Example 3: With callbacks
	fmt.Println("=== Example 3: With tool callbacks ===")
	host3, err := sdk.New(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer host3.Close()

	response, err = host3.PromptWithCallbacks(
		ctx,
		"List files in the current directory",
		func(name, args string) {
			fmt.Printf("🔧 Calling tool: %s\n", name)
		},
		func(name, args, result string, isError bool) {
			if isError {
				fmt.Printf("❌ Tool %s failed\n", name)
			} else {
				fmt.Printf("✅ Tool %s completed\n", name)
			}
		},
		func(chunk string) {
			fmt.Print(chunk) // Stream output
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nFinal response: %s\n", response)

	// Example 4: Session management
	fmt.Println("\n=== Example 4: Session management ===")
	host4, err := sdk.New(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer host4.Close()

	// First message
	_, err = host4.Prompt(ctx, "Remember that my favorite color is blue")
	if err != nil {
		log.Fatal(err)
	}

	// Second message (should remember context)
	response, err = host4.Prompt(ctx, "What's my favorite color?")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Response: %s\n", response)

	// Save session
	if err := host4.SaveSession("./session.json"); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Session saved to ./session.json")
}
