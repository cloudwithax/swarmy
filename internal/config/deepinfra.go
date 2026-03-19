package config

import (
	"charm.land/catwalk/pkg/catwalk"
)

// DeepInfraProvider returns the DeepInfra provider configuration.
// DeepInfra provides cost-effective inference for open-source models.
func DeepInfraProvider() catwalk.Provider {
	return catwalk.Provider{
		Name:                "DeepInfra",
		ID:                  "deepinfra",
		APIKey:              "$DEEPINFRA_API_KEY",
		APIEndpoint:         "https://api.deepinfra.com/v1/openai",
		Type:                "openai-compat",
		DefaultLargeModelID: "meta-llama/Llama-3.3-70B-Instruct",
		DefaultSmallModelID: "meta-llama/Meta-Llama-3.1-8B-Instruct",
		Models:              deepinfraModels(),
		DefaultHeaders:      map[string]string{},
	}
}

// deepinfraModels returns the list of models supported by DeepInfra.
func deepinfraModels() []catwalk.Model {
	return []catwalk.Model{
		{
			ID:               "meta-llama/Llama-3.3-70B-Instruct",
			Name:             "Llama 3.3 70B Instruct",
			CostPer1MIn:      0.23,
			CostPer1MOut:     0.4,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "meta-llama/Meta-Llama-3.1-70B-Instruct",
			Name:             "Llama 3.1 70B Instruct",
			CostPer1MIn:      0.23,
			CostPer1MOut:     0.4,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "meta-llama/Meta-Llama-3.1-8B-Instruct",
			Name:             "Llama 3.1 8B Instruct",
			CostPer1MIn:      0.03,
			CostPer1MOut:     0.05,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "microsoft/WizardLM-2-8x22B",
			Name:             "WizardLM 2 8x22B",
			CostPer1MIn:      0.65,
			CostPer1MOut:     0.65,
			ContextWindow:    65536,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "deepseek-ai/DeepSeek-V3",
			Name:             "DeepSeek V3",
			CostPer1MIn:      0.85,
			CostPer1MOut:     0.85,
			ContextWindow:    64000,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "deepseek-ai/DeepSeek-R1",
			Name:             "DeepSeek R1",
			CostPer1MIn:      0.85,
			CostPer1MOut:     2.5,
			ContextWindow:    64000,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "Qwen/Qwen2.5-72B-Instruct",
			Name:             "Qwen 2.5 72B Instruct",
			CostPer1MIn:      0.35,
			CostPer1MOut:     0.4,
			ContextWindow:    32768,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "Qwen/Qwen2.5-Coder-32B-Instruct",
			Name:             "Qwen 2.5 Coder 32B Instruct",
			CostPer1MIn:      0.18,
			CostPer1MOut:     0.18,
			ContextWindow:    32768,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "mistralai/Mixtral-8x22B-Instruct-v0.1",
			Name:             "Mixtral 8x22B Instruct",
			CostPer1MIn:      0.65,
			CostPer1MOut:     0.65,
			ContextWindow:    65536,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "mistralai/Mistral-7B-Instruct-v0.3",
			Name:             "Mistral 7B Instruct v0.3",
			CostPer1MIn:      0.07,
			CostPer1MOut:     0.07,
			ContextWindow:    32768,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
	}
}
