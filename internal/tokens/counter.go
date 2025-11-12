package tokens

// EstimateTokens estimates the number of tokens in the given text string.
// It uses a rough approximation of 4 characters per token, which is a common
// heuristic for most language models. This function provides a quick estimation
// without requiring model-specific tokenizers.
//
// The estimation may not be accurate for all models or text types, particularly
// for texts with many special characters, non-English languages, or code snippets.
// For more accurate token counting, use model-specific tokenizers when available.
//
// Parameters:
//   - text: The input text string to estimate tokens for
//
// Returns:
//   - int: The estimated number of tokens in the text
//
// Example:
//
//	count := EstimateTokens("Hello, world!")  // Returns approximately 3
func EstimateTokens(text string) int {
	// Rough approximation: ~4 characters per token for most models
	return len(text) / 4
}
