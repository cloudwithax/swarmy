package config

import (
	"time"

	"charm.land/catwalk/pkg/catwalk"
)

// NvidiaNIMOptions contains provider-specific options for NVIDIA NIM.
type NvidiaNIMOptions struct {
	// RateLimitEnabled enables rate limiting for NVIDIA NIM (40 RPM).
	RateLimitEnabled bool `json:"rate_limit_enabled,omitempty" jsonschema:"description=Enable rate limiting for NVIDIA NIM (40 requests per minute),default=true"`
	// Timeout is the request timeout for NVIDIA NIM (default 10 minutes for queue delays).
	Timeout time.Duration `json:"timeout,omitempty" jsonschema:"description=Request timeout for NVIDIA NIM,default=600s,example=600s,example=300s"`
	// MaxRetries is the maximum number of retries for rate limit errors.
	MaxRetries int `json:"max_retries,omitempty" jsonschema:"description=Maximum number of retries for rate limit errors,default=3,example=3,example=5"`
	// RetryBackoff is the initial backoff duration for retries.
	RetryBackoff time.Duration `json:"retry_backoff,omitempty" jsonschema:"description=Initial backoff duration for retries,default=2s,example=2s,example=5s"`
	// EnableOutputSanitization cleans up non-standard model outputs.
	EnableOutputSanitization bool `json:"enable_output_sanitization,omitempty" jsonschema:"description=Enable output sanitization for non-standard model outputs,default=true"`
}

// DefaultNvidiaNIMOptions returns the default options for NVIDIA NIM.
func DefaultNvidiaNIMOptions() NvidiaNIMOptions {
	return NvidiaNIMOptions{
		RateLimitEnabled:         true,
		Timeout:                  10 * time.Minute,
		MaxRetries:               3,
		RetryBackoff:             2 * time.Second,
		EnableOutputSanitization: true,
	}
}

// NvidiaNimProvider returns the NVIDIA NIM provider configuration.
// NVIDIA NIM provides performance-optimized inference microservices for
// various AI models including LLMs, with an OpenAI-compatible API.
func NvidiaNimProvider() catwalk.Provider {
	return catwalk.Provider{
		Name:                "NVIDIA NIM",
		ID:                  "nvidia-nim",
		APIKey:              "$NVIDIA_API_KEY",
		APIEndpoint:         "https://integrate.api.nvidia.com/v1",
		Type:                "openai-compat",
		DefaultLargeModelID: "nvidia/llama-3.1-nemotron-ultra-253b-v1",
		DefaultSmallModelID: "meta/llama-3.3-70b-instruct",
		Models:              nvidiaNimModels(),
		DefaultHeaders:      map[string]string{},
	}
}

// IsNvidiaNIMProvider returns true if the provider ID is NVIDIA NIM.
func IsNvidiaNIMProvider(providerID string) bool {
	return providerID == "nvidia-nim"
}

// GetNvidiaNIMOptions extracts NVIDIA NIM options from provider config.
// Returns default options if no custom options are configured.
func GetNvidiaNIMOptions(cfg *ProviderConfig) NvidiaNIMOptions {
	opts := DefaultNvidiaNIMOptions()
	if cfg == nil || cfg.ProviderOptions == nil {
		return opts
	}

	// Apply custom options over defaults by checking the ProviderOptions map directly.
	// This allows us to distinguish between "not set" and "explicitly set to false".
	if v, ok := cfg.ProviderOptions["timeout"]; ok {
		if s, ok := v.(string); ok {
			if d, err := time.ParseDuration(s); err == nil && d > 0 {
				opts.Timeout = d
			}
		}
	}
	if v, ok := cfg.ProviderOptions["max_retries"]; ok {
		if n, ok := v.(int); ok && n > 0 {
			opts.MaxRetries = n
		}
		// Also handle float64 which JSON numbers decode to.
		if f, ok := v.(float64); ok && f > 0 {
			opts.MaxRetries = int(f)
		}
	}
	if v, ok := cfg.ProviderOptions["retry_backoff"]; ok {
		if s, ok := v.(string); ok {
			if d, err := time.ParseDuration(s); err == nil && d > 0 {
				opts.RetryBackoff = d
			}
		}
	}
	// Boolean fields: check if explicitly set in the map.
	if v, ok := cfg.ProviderOptions["rate_limit_enabled"]; ok {
		if b, ok := v.(bool); ok {
			opts.RateLimitEnabled = b
		}
	}
	if v, ok := cfg.ProviderOptions["enable_output_sanitization"]; ok {
		if b, ok := v.(bool); ok {
			opts.EnableOutputSanitization = b
		}
	}

	return opts
}

// nvidiaNimModels returns the list of models available through NVIDIA NIM.
// Models include Llama, Nemotron, DeepSeek, and other popular models.
func nvidiaNimModels() []catwalk.Model {
	return []catwalk.Model{
		// NVIDIA Nemotron Models
		{
			ID:               "nvidia/llama-3.1-nemotron-ultra-253b-v1",
			Name:             "Nemotron Ultra 253B",
			CostPer1MIn:      0.50,
			CostPer1MOut:     1.50,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "nvidia/llama-3.3-nemotron-super-49b-v1",
			Name:             "Nemotron Super 49B",
			CostPer1MIn:      0.30,
			CostPer1MOut:     0.90,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "nvidia/llama-3.1-nemotron-nano-8b-v1",
			Name:             "Nemotron Nano 8B",
			CostPer1MIn:      0.10,
			CostPer1MOut:     0.30,
			ContextWindow:    131072,
			DefaultMaxTokens: 4096,
			CanReason:        false,
			SupportsImages:   false,
		},
		// Meta Llama Models
		{
			ID:               "meta/llama-3.1-405b-instruct",
			Name:             "Llama 3.1 405B",
			CostPer1MIn:      1.00,
			CostPer1MOut:     3.00,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "meta/llama-3.3-70b-instruct",
			Name:             "Llama 3.3 70B",
			CostPer1MIn:      0.20,
			CostPer1MOut:     0.60,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "meta/llama-3.1-70b-instruct",
			Name:             "Llama 3.1 70B",
			CostPer1MIn:      0.20,
			CostPer1MOut:     0.60,
			ContextWindow:    131072,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "meta/llama-3.1-8b-instruct",
			Name:             "Llama 3.1 8B",
			CostPer1MIn:      0.05,
			CostPer1MOut:     0.15,
			ContextWindow:    131072,
			DefaultMaxTokens: 4096,
			CanReason:        false,
			SupportsImages:   false,
		},
		// DeepSeek Models
		{
			ID:               "deepseek-ai/deepseek-v3.1",
			Name:             "DeepSeek V3.1",
			CostPer1MIn:      0.30,
			CostPer1MOut:     0.90,
			ContextWindow:    64000,
			DefaultMaxTokens: 4096,
			CanReason:        true,
			SupportsImages:   false,
		},
		// Mistral Models
		{
			ID:               "mistralai/mistral-large-2-instruct",
			Name:             "Mistral Large 2",
			CostPer1MIn:      0.50,
			CostPer1MOut:     1.50,
			ContextWindow:    128000,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		// Microsoft Phi Models
		{
			ID:               "microsoft/phi-4-mini-instruct",
			Name:             "Phi-4 Mini",
			CostPer1MIn:      0.05,
			CostPer1MOut:     0.15,
			ContextWindow:    131072,
			DefaultMaxTokens: 4096,
			CanReason:        false,
			SupportsImages:   false,
		},
		// Google Models
		{
			ID:               "google/gemma-2-27b-it",
			Name:             "Gemma 2 27B",
			CostPer1MIn:      0.15,
			CostPer1MOut:     0.45,
			ContextWindow:    8192,
			DefaultMaxTokens: 4096,
			CanReason:        false,
			SupportsImages:   false,
		},
		// Qwen Models
		{
			ID:               "qwen/qwen2.5-coder-32b-instruct",
			Name:             "Qwen2.5 Coder 32B",
			CostPer1MIn:      0.20,
			CostPer1MOut:     0.60,
			ContextWindow:    128000,
			DefaultMaxTokens: 4096,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "qwen/qwen3.5-122b-a10b",
			Name:             "Qwen 3.5 122B A10B",
			CostPer1MIn:      0.30,
			CostPer1MOut:     0.90,
			ContextWindow:    128000,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		// Z-AI GLM Models
		{
			ID:               "z-ai/glm4.7",
			Name:             "GLM 4.7",
			CostPer1MIn:      0.20,
			CostPer1MOut:     0.60,
			ContextWindow:    128000,
			DefaultMaxTokens: 4096,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "z-ai/glm5",
			Name:             "GLM 5",
			CostPer1MIn:      0.25,
			CostPer1MOut:     0.75,
			ContextWindow:    128000,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		// StepFun Models
		{
			ID:               "stepfun-ai/step-3.5-flash",
			Name:             "Step 3.5 Flash",
			CostPer1MIn:      0.15,
			CostPer1MOut:     0.45,
			ContextWindow:    128000,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		// OpenAI OSS Models
		{
			ID:               "openai/gpt-oss-20b",
			Name:             "GPT-OSS 20B",
			CostPer1MIn:      0.10,
			CostPer1MOut:     0.30,
			ContextWindow:    128000,
			DefaultMaxTokens: 4096,
			CanReason:        false,
			SupportsImages:   false,
		},
		{
			ID:               "openai/gpt-oss-120b",
			Name:             "GPT-OSS 120B",
			CostPer1MIn:      0.40,
			CostPer1MOut:     1.20,
			ContextWindow:    128000,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		// Additional NVIDIA Nemotron Models
		{
			ID:               "nvidia/nemotron-3-super-120b-a12b",
			Name:             "Nemotron 3 Super 120B A12B",
			CostPer1MIn:      0.35,
			CostPer1MOut:     1.05,
			ContextWindow:    128000,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		// MoonshotAI Models
		{
			ID:               "moonshotai/kimi-k2-instruct",
			Name:             "Kimi K2 Instruct",
			CostPer1MIn:      0.50,
			CostPer1MOut:     1.50,
			ContextWindow:    256000,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "moonshotai/kimi-k2-instruct-0905",
			Name:             "Kimi K2 Instruct 0905",
			CostPer1MIn:      0.50,
			CostPer1MOut:     1.50,
			ContextWindow:    256000,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		{
			ID:               "moonshotai/kimi-k2-thinking",
			Name:             "Kimi K2 Thinking",
			CostPer1MIn:      0.50,
			CostPer1MOut:     1.50,
			ContextWindow:    256000,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   false,
		},
		// Multimodal Models
		{
			ID:               "qwen/qwen3.5-397b-a17b",
			Name:             "Qwen 3.5 397B A17B Multimodal",
			CostPer1MIn:      0.80,
			CostPer1MOut:     2.40,
			ContextWindow:    128000,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   true,
		},
		{
			ID:               "moonshotai/kimi-k2.5",
			Name:             "Kimi K2.5 Multimodal",
			CostPer1MIn:      0.60,
			CostPer1MOut:     1.80,
			ContextWindow:    256000,
			DefaultMaxTokens: 8192,
			CanReason:        true,
			SupportsImages:   true,
		},
	}
}
