// Package tokens provides token counting and estimation functionality for
// various language model providers. It includes utilities for estimating
// token counts in text, as well as provider-specific implementations for
// more accurate token counting.
//
// The package supports multiple approaches to token counting:
//   - Quick estimation using character-based heuristics
//   - Provider-specific tokenizers for accurate counts
//   - Initialization functions for setting up token counters
//
// Token counting is essential for:
//   - Managing API rate limits
//   - Calculating costs for API usage
//   - Ensuring prompts fit within model context windows
//   - Optimizing prompt engineering and response handling
package tokens
