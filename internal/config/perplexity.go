package config

import (
	"charm.land/catwalk/pkg/catwalk"
)

// PerplexityProvider returns the Perplexity provider configuration.
// Perplexity provides search-augmented LLM models with real-time web search capabilities.
func PerplexityProvider() catwalk.Provider {
	return catwalk.Provider{
		Name:                "Perplexity",
		ID:                  "perplexity",
		APIKey:              "$PERPLEXITY_API_KEY",
		APIEndpoint:         "https://api.perplexity.ai",
		Type:                "openai-compat",
		DefaultLargeModelID: "sonar-pro",
		DefaultSmallModelID: "sonar",
		Models:              perplexityModels(),
		DefaultHeaders:      map[string]string{},
	}
}

// perplexityModels returns the list of models supported by Perplexity.
func perplexityModels() []catwalk.Model {
	return []catwalk.Model{
		{
			ID:               "sonar-pro",
			Name:             "Sonar Pro",
			CostPer1MIn:      3.0,
			CostPer1MOut:     15.0,
			ContextWindow:    200000,
			DefaultMaxTokens: 8000,
			CanReason:        false,
			SupportsImages:   true,
		},
		{
			ID:               "sonar-reasoning-pro",
			Name:             "Sonar Reasoning Pro",
			CostPer1MIn:      2.0,
			CostPer1MOut:     8.0,
			ContextWindow:    128000,
			DefaultMaxTokens: 8000,
			CanReason:        true,
			SupportsImages:   true,
		},
		{
			ID:               "sonar-reasoning",
			Name:             "Sonar Reasoning",
			CostPer1MIn:      1.0,
			CostPer1MOut:     5.0,
			ContextWindow:    128000,
			DefaultMaxTokens: 8000,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "sonar",
			Name:             "Sonar",
			CostPer1MIn:      1.0,
			CostPer1MOut:     1.0,
			ContextWindow:    127072,
			DefaultMaxTokens: 8000,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "sonar-deep-research",
			Name:             "Sonar Deep Research",
			CostPer1MIn:      2.0,
			CostPer1MOut:     8.0,
			ContextWindow:    128000,
			DefaultMaxTokens: 8000,
			CanReason:        true,
			SupportsImages:   false,
		},
	}
}
