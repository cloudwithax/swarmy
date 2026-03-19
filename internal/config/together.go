package config

import (
	"charm.land/catwalk/pkg/catwalk"
)

// TogetherProvider returns the Together AI provider configuration.
// Together AI provides access to open-source models with high-performance inference.
func TogetherProvider() catwalk.Provider {
	return catwalk.Provider{
		Name:                "Together AI",
		ID:                  "together",
		APIKey:              "$TOGETHER_API_KEY",
		APIEndpoint:         "https://api.together.xyz/v1",
		Type:                "openai-compat",
		DefaultLargeModelID: "meta-llama/Llama-3.3-70B-Instruct-Turbo",
		DefaultSmallModelID: "meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo",
		Models:              togetherModels(),
		DefaultHeaders:      map[string]string{},
	}
}

// togetherModels returns the list of models supported by Together AI.
func togetherModels() []catwalk.Model {
	return []catwalk.Model{
		{
			ID:               "meta-llama/Llama-3.3-70B-Instruct-Turbo",
			Name:             "Llama 3.3 70B Instruct Turbo",
			CostPer1MIn:      0.88,
			CostPer1MOut:     0.88,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo",
			Name:             "Llama 3.1 70B Instruct Turbo",
			CostPer1MIn:      0.88,
			CostPer1MOut:     0.88,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo",
			Name:             "Llama 3.1 8B Instruct Turbo",
			CostPer1MIn:      0.18,
			CostPer1MOut:     0.18,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "meta-llama/Llama-3.2-3B-Instruct-Turbo",
			Name:             "Llama 3.2 3B Instruct Turbo",
			CostPer1MIn:      0.06,
			CostPer1MOut:     0.06,
			ContextWindow:    8192,
			DefaultMaxTokens: 4096,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "deepseek-ai/DeepSeek-R1",
			Name:             "DeepSeek R1",
			CostPer1MIn:      7.0,
			CostPer1MOut:     7.0,
			ContextWindow:    160000,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "deepseek-ai/DeepSeek-V3",
			Name:             "DeepSeek V3",
			CostPer1MIn:      1.25,
			CostPer1MOut:     1.25,
			ContextWindow:    64000,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "Qwen/Qwen2.5-72B-Instruct-Turbo",
			Name:             "Qwen 2.5 72B Instruct Turbo",
			CostPer1MIn:      1.2,
			CostPer1MOut:     1.2,
			ContextWindow:    32768,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "Qwen/Qwen2.5-Coder-32B-Instruct",
			Name:             "Qwen 2.5 Coder 32B Instruct",
			CostPer1MIn:      0.8,
			CostPer1MOut:     0.8,
			ContextWindow:    32768,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "mistralai/Mixtral-8x22B-Instruct-v0.1",
			Name:             "Mixtral 8x22B Instruct",
			CostPer1MIn:      1.2,
			CostPer1MOut:     1.2,
			ContextWindow:    65536,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
	}
}
