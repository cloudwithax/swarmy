package config

import (
	"charm.land/catwalk/pkg/catwalk"
)

// CerebrasProvider returns the Cerebras provider configuration.
// Cerebras provides ultra-fast inference on specialized AI hardware (wafer-scale chips).
func CerebrasProvider() catwalk.Provider {
	return catwalk.Provider{
		Name:                "Cerebras",
		ID:                  "cerebras",
		APIKey:              "$CEREBRAS_API_KEY",
		APIEndpoint:         "https://api.cerebras.ai/v1",
		Type:                "openai-compat",
		DefaultLargeModelID: "llama-3.3-70b",
		DefaultSmallModelID: "llama-3.1-8b",
		Models:              cerebrasModels(),
		DefaultHeaders:      map[string]string{},
	}
}

// cerebrasModels returns the list of models supported by Cerebras.
func cerebrasModels() []catwalk.Model {
	return []catwalk.Model{
		{
			ID:               "llama-3.3-70b",
			Name:             "Llama 3.3 70B",
			CostPer1MIn:      0.85,
			CostPer1MOut:     1.2,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "llama-3.1-70b",
			Name:             "Llama 3.1 70B",
			CostPer1MIn:      0.85,
			CostPer1MOut:     1.2,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "llama-3.1-8b",
			Name:             "Llama 3.1 8B",
			CostPer1MIn:      0.1,
			CostPer1MOut:     0.1,
			ContextWindow:    8192,
			DefaultMaxTokens: 4096,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "deepseek-r1-distill-llama-70b",
			Name:             "DeepSeek R1 Distill Llama 70B",
			CostPer1MIn:      0.85,
			CostPer1MOut:     1.2,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
	}
}
