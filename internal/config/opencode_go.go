package config

import (
	"charm.land/catwalk/pkg/catwalk"
)

// OpenCodeGoProvider returns the OpenCode Go provider configuration.
// OpenCode Go is a low-cost subscription plan ($5 first month, then $10/month)
// that provides reliable access to popular open coding models.
// Note: Go uses the same Zen API endpoint and key, but accesses different models.
func OpenCodeGoProvider() catwalk.Provider {
	return catwalk.Provider{
		Name:                "OpenCode Go",
		ID:                  "opencode-go",
		APIKey:              "$ZEN_API_KEY",
		APIEndpoint:         "https://opencode.ai/zen/go/v1",
		Type:                "openai-compat",
		DefaultLargeModelID: "kimi-k2.5",
		DefaultSmallModelID: "minimax-m2.5",
		Models:              openCodeGoModels(),
		DefaultHeaders:      map[string]string{},
	}
}

// openCodeGoModels returns the list of models supported by OpenCode Go.
// Go focuses on reliable access to popular open coding models.
func openCodeGoModels() []catwalk.Model {
	return []catwalk.Model{
		{
			ID:               "glm-5",
			Name:             "GLM-5",
			CostPer1MIn:      1.0,
			CostPer1MOut:     3.2,
			ContextWindow:    204800,
			DefaultMaxTokens: 65536,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "kimi-k2.5",
			Name:             "Kimi K2.5",
			CostPer1MIn:      0.5,
			CostPer1MOut:     2.0,
			ContextWindow:    256000,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "minimax-m2.5",
			Name:             "MiniMax M2.5",
			CostPer1MIn:      0.5,
			CostPer1MOut:     2.0,
			ContextWindow:    100000,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
	}
}
