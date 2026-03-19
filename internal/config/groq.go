package config

import (
	"charm.land/catwalk/pkg/catwalk"
)

// GroqProvider returns the Groq provider configuration.
// Groq provides ultra-fast inference for open-source models using LPU technology.
func GroqProvider() catwalk.Provider {
	return catwalk.Provider{
		Name:                "Groq",
		ID:                  "groq",
		APIKey:              "$GROQ_API_KEY",
		APIEndpoint:         "https://api.groq.com/openai/v1",
		Type:                "openai-compat",
		DefaultLargeModelID: "llama-3.3-70b-versatile",
		DefaultSmallModelID: "llama-3.1-8b-instant",
		Models:              groqModels(),
		DefaultHeaders:      map[string]string{},
	}
}

// groqModels returns the list of models supported by Groq.
func groqModels() []catwalk.Model {
	return []catwalk.Model{
		{
			ID:               "llama-3.3-70b-versatile",
			Name:             "Llama 3.3 70B Versatile",
			CostPer1MIn:      0.59,
			CostPer1MOut:     0.79,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "llama-3.1-70b-versatile",
			Name:             "Llama 3.1 70B Versatile",
			CostPer1MIn:      0.59,
			CostPer1MOut:     0.79,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "llama-3.1-8b-instant",
			Name:             "Llama 3.1 8B Instant",
			CostPer1MIn:      0.05,
			CostPer1MOut:     0.08,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "gemma2-9b-it",
			Name:             "Gemma 2 9B IT",
			CostPer1MIn:      0.2,
			CostPer1MOut:     0.2,
			ContextWindow:    8192,
			DefaultMaxTokens: 4096,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "mixtral-8x7b-32768",
			Name:             "Mixtral 8x7B 32K",
			CostPer1MIn:      0.24,
			CostPer1MOut:     0.24,
			ContextWindow:    32768,
			DefaultMaxTokens: 4096,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "deepseek-r1-distill-llama-70b",
			Name:             "DeepSeek R1 Distill Llama 70B",
			CostPer1MIn:      0.75,
			CostPer1MOut:     0.99,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "deepseek-r1-distill-qwen-32b",
			Name:             "DeepSeek R1 Distill Qwen 32B",
			CostPer1MIn:      0.69,
			CostPer1MOut:     0.69,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "qwen-2.5-32b",
			Name:             "Qwen 2.5 32B",
			CostPer1MIn:      0.79,
			CostPer1MOut:     0.79,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "qwen-2.5-coder-32b",
			Name:             "Qwen 2.5 Coder 32B",
			CostPer1MIn:      0.79,
			CostPer1MOut:     0.79,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
	}
}
