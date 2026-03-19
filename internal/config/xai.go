package config

import (
	"charm.land/catwalk/pkg/catwalk"
)

// XAIProvider returns the xAI (Grok) provider configuration.
// xAI provides Grok models with high-speed inference and a 2M token context window.
func XAIProvider() catwalk.Provider {
	return catwalk.Provider{
		Name:                "xAI",
		ID:                  "xai",
		APIKey:              "$XAI_API_KEY",
		APIEndpoint:         "https://api.x.ai/v1",
		Type:                "openai-compat",
		DefaultLargeModelID: "grok-4",
		DefaultSmallModelID: "grok-3-mini",
		Models:              xaiModels(),
		DefaultHeaders:      map[string]string{},
	}
}

// xaiModels returns the list of models supported by xAI.
func xaiModels() []catwalk.Model {
	return []catwalk.Model{
		{
			ID:               "grok-4",
			Name:             "Grok 4",
			CostPer1MIn:      3.0,
			CostPer1MOut:     15.0,
			ContextWindow:    256000,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   true,
		},
		{
			ID:               "grok-4-fast",
			Name:             "Grok 4 Fast",
			CostPer1MIn:      0.2,
			CostPer1MOut:     0.5,
			ContextWindow:    2000000,
			DefaultMaxTokens: 30000,
			CanReason:        true,
			SupportsImages:   true,
		},
		{
			ID:               "grok-3",
			Name:             "Grok 3",
			CostPer1MIn:      3.0,
			CostPer1MOut:     15.0,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "grok-3-mini",
			Name:             "Grok 3 Mini",
			CostPer1MIn:      0.3,
			CostPer1MOut:     0.5,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
	}
}
