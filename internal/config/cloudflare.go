package config

import (
	"charm.land/catwalk/pkg/catwalk"
)

// CloudflareProvider returns the Cloudflare Workers AI provider configuration.
// Cloudflare Workers AI provides serverless GPU inference for open-source models.
func CloudflareProvider() catwalk.Provider {
	return catwalk.Provider{
		Name:                "Cloudflare Workers AI",
		ID:                  "cloudflare",
		APIKey:              "$CLOUDFLARE_API_TOKEN",
		APIEndpoint:         "https://api.cloudflare.com/client/v4/accounts/$CLOUDFLARE_ACCOUNT_ID/ai/v1",
		Type:                "openai-compat",
		DefaultLargeModelID: "@cf/meta/llama-3.3-70b-instruct-fp16-fast",
		DefaultSmallModelID: "@cf/meta/llama-3.1-8b-instruct",
		Models:              cloudflareModels(),
		DefaultHeaders:      map[string]string{},
	}
}

// cloudflareModels returns the list of models supported by Cloudflare Workers AI.
func cloudflareModels() []catwalk.Model {
	return []catwalk.Model{
		{
			ID:               "@cf/meta/llama-3.3-70b-instruct-fp16-fast",
			Name:             "Llama 3.3 70B Instruct (Fast)",
			CostPer1MIn:      0.0,
			CostPer1MOut:     0.0,
			ContextWindow:    32768,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "@cf/meta/llama-3.1-70b-instruct",
			Name:             "Llama 3.1 70B Instruct",
			CostPer1MIn:      0.0,
			CostPer1MOut:     0.0,
			ContextWindow:    32768,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "@cf/meta/llama-3.1-8b-instruct",
			Name:             "Llama 3.1 8B Instruct",
			CostPer1MIn:      0.0,
			CostPer1MOut:     0.0,
			ContextWindow:    32768,
			DefaultMaxTokens: 8192,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "@cf/meta/llama-3.1-8b-instruct-fast",
			Name:             "Llama 3.1 8B Instruct (Fast)",
			CostPer1MIn:      0.0,
			CostPer1MOut:     0.0,
			ContextWindow:    8192,
			DefaultMaxTokens: 4096,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "@cf/mistral/mistral-7b-instruct-v0.1",
			Name:             "Mistral 7B Instruct",
			CostPer1MIn:      0.0,
			CostPer1MOut:     0.0,
			ContextWindow:    8192,
			DefaultMaxTokens: 4096,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "@cf/deepseek/deepseek-r1-distill-qwen-32b",
			Name:             "DeepSeek R1 Distill Qwen 32B",
			CostPer1MIn:      0.0,
			CostPer1MOut:     0.0,
			ContextWindow:    16384,
			DefaultMaxTokens: 4096,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "@cf/qwen/qwen1.5-14b-chat-awq",
			Name:             "Qwen 1.5 14B Chat (AWQ)",
			CostPer1MIn:      0.0,
			CostPer1MOut:     0.0,
			ContextWindow:    8192,
			DefaultMaxTokens: 4096,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "@hf/nousresearch/hermes-2-pro-mistral-7b",
			Name:             "Hermes 2 Pro Mistral 7B",
			CostPer1MIn:      0.0,
			CostPer1MOut:     0.0,
			ContextWindow:    8192,
			DefaultMaxTokens: 4096,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "@cf/google/gemma-2b-it-lora",
			Name:             "Gemma 2B IT (LoRA)",
			CostPer1MIn:      0.0,
			CostPer1MOut:     0.0,
			ContextWindow:    4096,
			DefaultMaxTokens: 2048,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "@cf/google/gemma-7b-it-lora",
			Name:             "Gemma 7B IT (LoRA)",
			CostPer1MIn:      0.0,
			CostPer1MOut:     0.0,
			ContextWindow:    8192,
			DefaultMaxTokens: 4096,
			CanReason:        false,
			SupportsImages:   false,
		},
	}
}
