package models

import (
	"net/http"
	"testing"
)

func TestCreateHTTPClientWithTLSConfig(t *testing.T) {
	tests := []struct {
		name         string
		skipVerify   bool
		wantInsecure bool
	}{
		{
			name:         "skip verify disabled",
			skipVerify:   false,
			wantInsecure: false,
		},
		{
			name:         "skip verify enabled",
			skipVerify:   true,
			wantInsecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createHTTPClientWithTLSConfig(tt.skipVerify)

			if client == nil {
				t.Fatal("expected non-nil client")
			}

			// Check if the client has a custom transport when skipVerify is true
			if tt.skipVerify {
				transport, ok := client.Transport.(*http.Transport)
				if !ok {
					t.Fatal("expected *http.Transport when skipVerify is true")
				}

				if transport.TLSClientConfig == nil {
					t.Fatal("expected non-nil TLSClientConfig when skipVerify is true")
				}

				if transport.TLSClientConfig.InsecureSkipVerify != tt.wantInsecure {
					t.Errorf("InsecureSkipVerify = %v, want %v",
						transport.TLSClientConfig.InsecureSkipVerify, tt.wantInsecure)
				}
			}
		})
	}
}

func TestCreateOAuthHTTPClient(t *testing.T) {
	tests := []struct {
		name         string
		accessToken  string
		skipVerify   bool
		wantInsecure bool
	}{
		{
			name:         "oauth with skip verify disabled",
			accessToken:  "test-token",
			skipVerify:   false,
			wantInsecure: false,
		},
		{
			name:         "oauth with skip verify enabled",
			accessToken:  "test-token",
			skipVerify:   true,
			wantInsecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createOAuthHTTPClient(tt.accessToken, tt.skipVerify)

			if client == nil {
				t.Fatal("expected non-nil client")
			}

			// Check that the transport is an oauthTransport
			oauthTransport, ok := client.Transport.(*oauthTransport)
			if !ok {
				t.Fatal("expected *oauthTransport")
			}

			if oauthTransport.accessToken != tt.accessToken {
				t.Errorf("accessToken = %v, want %v", oauthTransport.accessToken, tt.accessToken)
			}

			// Check the base transport when skipVerify is true
			if tt.skipVerify {
				baseTransport, ok := oauthTransport.base.(*http.Transport)
				if !ok {
					t.Fatal("expected base transport to be *http.Transport when skipVerify is true")
				}

				if baseTransport.TLSClientConfig == nil {
					t.Fatal("expected non-nil TLSClientConfig when skipVerify is true")
				}

				if baseTransport.TLSClientConfig.InsecureSkipVerify != tt.wantInsecure {
					t.Errorf("InsecureSkipVerify = %v, want %v",
						baseTransport.TLSClientConfig.InsecureSkipVerify, tt.wantInsecure)
				}
			}
		})
	}
}

func TestProviderConfigTLSSkipVerify(t *testing.T) {
	// Test that ProviderConfig properly stores TLSSkipVerify
	config := &ProviderConfig{
		ModelString:   "test:model",
		TLSSkipVerify: true,
	}

	if !config.TLSSkipVerify {
		t.Error("expected TLSSkipVerify to be true")
	}
}

func TestCreateHuggingFaceProvider(t *testing.T) {
	// Test case 1: API key provided via flag
	t.Run("API key from flag", func(t *testing.T) {
		config := &ProviderConfig{
			ModelString:    "huggingface:test-model",
			ProviderAPIKey: "test-api-key",
		}
		provider, err := createHuggingFaceProvider(nil, config, "test-model")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if provider == nil {
			t.Error("expected provider to be not nil")
		}
	})

	// Test case 2: API key from environment variable
	t.Run("API key from env", func(t *testing.T) {
		t.Setenv("HUGGINGFACE_API_KEY", "test-env-key")
		config := &ProviderConfig{
			ModelString: "huggingface:test-model",
		}
		provider, err := createHuggingFaceProvider(nil, config, "test-model")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if provider == nil {
			t.Error("expected provider to be not nil")
		}
	})

	// Test case 3: No API key provided
	t.Run("No API key", func(t *testing.T) {
		config := &ProviderConfig{
			ModelString: "huggingface:test-model",
		}
		_, err := createHuggingFaceProvider(nil, config, "test-model")
		if err == nil {
			t.Error("expected an error, but got nil")
		}
	})
}

func TestCreateOpenRouterProvider(t *testing.T) {
	// Test case 1: API key provided via flag
	t.Run("API key from flag", func(t *testing.T) {
		config := &ProviderConfig{
			ModelString:    "openrouter:test-model",
			ProviderAPIKey: "test-api-key",
		}
		provider, err := createOpenRouterProvider(nil, config, "test-model")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if provider == nil {
			t.Error("expected provider to be not nil")
		}
	})

	// Test case 2: API key from environment variable
	t.Run("API key from env", func(t *testing.T) {
		t.Setenv("OPENROUTER_API_KEY", "test-env-key")
		config := &ProviderConfig{
			ModelString: "openrouter:test-model",
		}
		provider, err := createOpenRouterProvider(nil, config, "test-model")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if provider == nil {
			t.Error("expected provider to be not nil")
		}
	})

	// Test case 3: No API key provided
	t.Run("No API key", func(t *testing.T) {
		config := &ProviderConfig{
			ModelString: "openrouter:test-model",
		}
		_, err := createOpenRouterProvider(nil, config, "test-model")
		if err == nil {
			t.Error("expected an error, but got nil")
		}
	})
}
