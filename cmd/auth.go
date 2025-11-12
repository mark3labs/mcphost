package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcphost/internal/auth"
	"github.com/spf13/cobra"
)

// authCmd represents the auth command for managing AI provider authentication.
// This command provides subcommands for login, logout, and status checking
// of authentication credentials for various AI providers, with OAuth support
// for providers like Anthropic.
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication credentials for AI providers",
	Long: `Manage authentication credentials for AI providers.

This command allows you to securely authenticate and manage credentials for various AI providers
using OAuth flows. Stored credentials take precedence over environment variables.

Available providers:
  - anthropic: Anthropic Claude API (OAuth)

Examples:
  mcphost auth login anthropic
  mcphost auth logout anthropic
  mcphost auth status`,
}

// authLoginCmd represents the login subcommand for authenticating with AI providers.
// It handles OAuth flow for supported providers, opening a browser for authentication
// and securely storing the resulting credentials for future use.
var authLoginCmd = &cobra.Command{
	Use:   "login [provider]",
	Short: "Authenticate with an AI provider using OAuth",
	Long: `Authenticate with an AI provider using OAuth flow.

This will open your browser to complete the OAuth authentication process.
Your credentials will be securely stored and will take precedence over 
environment variables when making API calls.

Available providers:
  - anthropic: Anthropic Claude API (OAuth)

Example:
  mcphost auth login anthropic`,
	Args: cobra.ExactArgs(1),
	RunE: runAuthLogin,
}

// authLogoutCmd represents the logout subcommand for removing stored authentication credentials.
// This command removes stored API keys or OAuth tokens for specified providers,
// requiring the user to authenticate again or use environment variables.
var authLogoutCmd = &cobra.Command{
	Use:   "logout [provider]",
	Short: "Remove stored authentication credentials for a provider",
	Long: `Remove stored authentication credentials for an AI provider.

This will delete the stored API key for the specified provider. You will need
to use environment variables or command-line flags for authentication after logout.

Available providers:
  - anthropic: Anthropic Claude API

Example:
  mcphost auth logout anthropic`,
	Args: cobra.ExactArgs(1),
	RunE: runAuthLogout,
}

// authStatusCmd represents the status subcommand for checking authentication status.
// It displays which providers have stored credentials, their types (OAuth vs API key),
// creation dates, and expiration status without revealing the actual credentials.
var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status for all providers",
	Long: `Show the current authentication status for all supported AI providers.

This command displays which providers have stored credentials and when they were created.
It does not display the actual API keys for security reasons.

Example:
  mcphost auth status`,
	RunE: runAuthStatus,
}

func init() {
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	provider := strings.ToLower(args[0])

	switch provider {
	case "anthropic":
		return loginAnthropic()
	default:
		return fmt.Errorf("unsupported provider: %s. Available providers: anthropic", provider)
	}
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	provider := strings.ToLower(args[0])

	switch provider {
	case "anthropic":
		return logoutAnthropic()
	default:
		return fmt.Errorf("unsupported provider: %s. Available providers: anthropic", provider)
	}
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	cm, err := auth.NewCredentialManager()
	if err != nil {
		return fmt.Errorf("failed to initialize credential manager: %w", err)
	}

	fmt.Println("Authentication Status")
	fmt.Println("====================")
	fmt.Printf("Credentials file: %s\n\n", cm.GetCredentialsPath())

	// Check Anthropic credentials
	fmt.Print("Anthropic Claude: ")
	if hasAnthropicCreds, err := cm.HasAnthropicCredentials(); err != nil {
		fmt.Printf("Error checking credentials: %v\n", err)
	} else if hasAnthropicCreds {
		if creds, err := cm.GetAnthropicCredentials(); err != nil {
			fmt.Printf("Error reading credentials: %v\n", err)
		} else {
			authType := "API Key"
			status := "✓ Authenticated"

			if creds.Type == "oauth" {
				authType = "OAuth"
				if creds.IsExpired() {
					status = "⚠️  Token expired (will refresh automatically)"
				} else if creds.NeedsRefresh() {
					status = "⚠️  Token expires soon (will refresh automatically)"
				}
			}

			fmt.Printf("%s (%s, stored %s)\n", status, authType, creds.CreatedAt.Format("2006-01-02 15:04:05"))
		}
	} else {
		fmt.Println("✗ Not authenticated")
		// Check if environment variable is set
		if os.Getenv("ANTHROPIC_API_KEY") != "" {
			fmt.Println("  (ANTHROPIC_API_KEY environment variable is set)")
		}
	}

	fmt.Println("\nTo authenticate with a provider:")
	fmt.Println("  mcphost auth login anthropic")

	return nil
}

func loginAnthropic() error {
	cm, err := auth.NewCredentialManager()
	if err != nil {
		return fmt.Errorf("failed to initialize credential manager: %w", err)
	}

	// Check if already authenticated
	if hasAuth, err := cm.HasAnthropicCredentials(); err == nil && hasAuth {
		fmt.Print("You are already authenticated with Anthropic. Do you want to re-authenticate? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Authentication cancelled.")
			return nil
		}
	}

	// Create OAuth client
	client := auth.NewOAuthClient()

	// Generate authorization URL
	fmt.Println("🔐 Starting OAuth authentication with Anthropic...")
	authData, err := client.GetAuthorizationURL()
	if err != nil {
		return fmt.Errorf("failed to generate authorization URL: %w", err)
	}

	// Display URL and try to open browser
	fmt.Println("\n📱 Opening your browser for authentication...")
	fmt.Println("If the browser doesn't open automatically, please visit this URL:")
	fmt.Printf("\n%s\n\n", authData.URL)

	// Try to open browser
	auth.TryOpenBrowser(authData.URL)

	// Wait for user to complete OAuth flow
	fmt.Println("After authorizing the application, you'll receive an authorization code.")
	fmt.Print("Please enter the authorization code: ")

	reader := bufio.NewReader(os.Stdin)
	code, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read authorization code: %w", err)
	}
	code = strings.TrimSpace(code)

	if code == "" {
		return fmt.Errorf("authorization code cannot be empty")
	}

	// Exchange code for tokens
	fmt.Println("\n🔄 Exchanging authorization code for access token...")
	creds, err := client.ExchangeCode(code, authData.Verifier)
	if err != nil {
		return fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	// Store the credentials
	if err := cm.SetOAuthCredentials(creds); err != nil {
		return fmt.Errorf("failed to store credentials: %w", err)
	}

	fmt.Println("✅ Successfully authenticated with Anthropic!")
	fmt.Printf("📁 Credentials stored in: %s\n", cm.GetCredentialsPath())
	fmt.Println("\n🎉 Your OAuth credentials will now be used for Anthropic API calls.")
	fmt.Println("💡 You can check your authentication status with: mcphost auth status")

	return nil
}

func logoutAnthropic() error {
	cm, err := auth.NewCredentialManager()
	if err != nil {
		return fmt.Errorf("failed to initialize credential manager: %w", err)
	}

	// Check if authenticated
	hasAuth, err := cm.HasAnthropicCredentials()
	if err != nil {
		return fmt.Errorf("failed to check authentication status: %w", err)
	}

	if !hasAuth {
		fmt.Println("You are not currently authenticated with Anthropic.")
		return nil
	}

	// Confirm logout
	fmt.Print("Are you sure you want to remove your Anthropic credentials? (y/N): ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		fmt.Println("Logout cancelled.")
		return nil
	}

	// Remove credentials
	if err := cm.RemoveAnthropicCredentials(); err != nil {
		return fmt.Errorf("failed to remove credentials: %w", err)
	}

	fmt.Println("✓ Successfully logged out from Anthropic!")
	fmt.Println("You will need to use environment variables or command-line flags for authentication.")

	return nil
}
