package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Variable substitution patterns
var (
	envVarPattern     = regexp.MustCompile(`\$\{env://([A-Za-z_][A-Za-z0-9_]*)(:-([^}]*))?\}`)
	scriptArgsPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:-([^}]*))?\}`)
)

// parseVariableWithDefault extracts variable name and default value
// Works for both ${var:-default} and ${env://var:-default} patterns
func parseVariableWithDefault(varPart string) (varName, defaultValue string, hasDefault bool) {
	// Handle the case where varPart is like "VAR:-default" or just "VAR"
	if strings.Contains(varPart, ":-") {
		parts := strings.SplitN(varPart, ":-", 2)
		return parts[0], parts[1], true
	}
	return varPart, "", false
}

// EnvSubstituter handles environment variable substitution
type EnvSubstituter struct{}

// SubstituteEnvVars replaces ${env://VAR} and ${env://VAR:-default} patterns with environment variables
func (e *EnvSubstituter) SubstituteEnvVars(content string) (string, error) {
	var errors []string

	result := envVarPattern.ReplaceAllStringFunc(content, func(match string) string {
		// Extract the variable part from ${env://VAR:-default}
		// Remove ${env:// prefix and } suffix
		varPart := strings.TrimPrefix(strings.TrimSuffix(match, "}"), "${env://")

		varName, defaultValue, hasDefault := parseVariableWithDefault(varPart)

		if envValue := os.Getenv(varName); envValue != "" {
			return envValue
		}

		if hasDefault {
			return defaultValue
		}

		errors = append(errors, fmt.Sprintf("required environment variable %s not set in %s", varName, match))
		return match // Keep original if error
	})

	if len(errors) > 0 {
		return "", fmt.Errorf("environment variable substitution failed: %s", strings.Join(errors, ", "))
	}

	return result, nil
}

// ArgsSubstituter handles script argument substitution
type ArgsSubstituter struct {
	args map[string]string
}

// NewArgsSubstituter creates a new args substituter with the given arguments
func NewArgsSubstituter(args map[string]string) *ArgsSubstituter {
	return &ArgsSubstituter{args: args}
}

// SubstituteArgs replaces ${VAR} and ${VAR:-default} patterns with script arguments
func (a *ArgsSubstituter) SubstituteArgs(content string) (string, error) {
	var errors []string

	result := scriptArgsPattern.ReplaceAllStringFunc(content, func(match string) string {
		// Extract the variable part from ${VAR:-default}
		// Remove ${ prefix and } suffix
		varPart := strings.TrimPrefix(strings.TrimSuffix(match, "}"), "${")

		varName, defaultValue, hasDefault := parseVariableWithDefault(varPart)

		if argValue, exists := a.args[varName]; exists {
			return argValue
		}

		if hasDefault {
			return defaultValue
		}

		errors = append(errors, fmt.Sprintf("required script argument '%s' not set in %s", varName, match))
		return match // Keep original if error
	})

	if len(errors) > 0 {
		return "", fmt.Errorf("script argument substitution failed: %s", strings.Join(errors, ", "))
	}

	return result, nil
}

// HasEnvVars checks if content contains environment variable patterns
func HasEnvVars(content string) bool {
	return envVarPattern.MatchString(content)
}

// HasScriptArgs checks if content contains script argument patterns
func HasScriptArgs(content string) bool {
	return scriptArgsPattern.MatchString(content)
}
