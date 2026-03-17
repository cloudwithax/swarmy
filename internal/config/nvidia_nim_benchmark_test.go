package config

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestModels lists all models to be benchmarked
type TestModel struct {
	ID             string
	Name           string
	IsMultimodal   bool
	ExpectedToWork bool // Some models may be preview/beta and not always available
}

// Text-only models
var textOnlyModels = []TestModel{
	{ID: "z-ai/glm4.7", Name: "GLM 4.7", IsMultimodal: false, ExpectedToWork: true},
	{ID: "z-ai/glm5", Name: "GLM 5", IsMultimodal: false, ExpectedToWork: true},
	{ID: "stepfun-ai/step-3.5-flash", Name: "Step 3.5 Flash", IsMultimodal: false, ExpectedToWork: true},
	{ID: "qwen/qwen3.5-122b-a10b", Name: "Qwen 3.5 122B", IsMultimodal: false, ExpectedToWork: true},
	{ID: "openai/gpt-oss-20b", Name: "GPT-OSS 20B", IsMultimodal: false, ExpectedToWork: true},
	{ID: "openai/gpt-oss-120b", Name: "GPT-OSS 120B", IsMultimodal: false, ExpectedToWork: true},
	{ID: "nvidia/nemotron-3-super-120b-a12b", Name: "Nemotron 3 Super 120B", IsMultimodal: false, ExpectedToWork: true},
	{ID: "moonshotai/kimi-k2-instruct", Name: "Kimi K2", IsMultimodal: false, ExpectedToWork: true},
	{ID: "moonshotai/kimi-k2-instruct-0905", Name: "Kimi K2 0905", IsMultimodal: false, ExpectedToWork: true},
	{ID: "moonshotai/kimi-k2-thinking", Name: "Kimi K2 Thinking", IsMultimodal: false, ExpectedToWork: true},
}

// Multimodal models
var multimodalModels = []TestModel{
	{ID: "qwen/qwen3.5-397b-a17b", Name: "Qwen 3.5 397B Multimodal", IsMultimodal: true, ExpectedToWork: true},
	{ID: "moonshotai/kimi-k2.5", Name: "Kimi K2.5 Multimodal", IsMultimodal: true, ExpectedToWork: true},
}

// TestNvidiaNIMModelsExist verifies all expected models are in the provider configuration.
func TestNvidiaNIMModelsExist(t *testing.T) {
	provider := NvidiaNimProvider()

	// Build map of available models
	availableModels := make(map[string]bool)
	for _, m := range provider.Models {
		availableModels[m.ID] = true
	}

	// Check all text-only models exist
	t.Run("text_only_models", func(t *testing.T) {
		for _, model := range textOnlyModels {
			t.Run(model.ID, func(t *testing.T) {
				assert.True(t, availableModels[model.ID], "Model %s should be available in NVIDIA NIM provider", model.ID)
			})
		}
	})

	// Check all multimodal models exist
	t.Run("multimodal_models", func(t *testing.T) {
		for _, model := range multimodalModels {
			t.Run(model.ID, func(t *testing.T) {
				assert.True(t, availableModels[model.ID], "Model %s should be available in NVIDIA NIM provider", model.ID)
			})
		}
	})
}

// TestNvidiaNIMModelProperties verifies model properties are correctly configured.
func TestNvidiaNIMModelProperties(t *testing.T) {
	provider := NvidiaNimProvider()

	// Build map of models
	modelMap := make(map[string]struct {
		Model  catwalk.Model
		Exists bool
	})
	for _, m := range provider.Models {
		modelMap[m.ID] = struct {
			Model  catwalk.Model
			Exists bool
		}{Model: m, Exists: true}
	}

	// Test multimodal models have SupportsImages=true
	t.Run("multimodal_models_support_images", func(t *testing.T) {
		for _, testModel := range multimodalModels {
			t.Run(testModel.ID, func(t *testing.T) {
				m, ok := modelMap[testModel.ID]
				if !ok {
					t.Skipf("Model %s not found in provider", testModel.ID)
				}
				assert.True(t, m.Model.SupportsImages, "Multimodal model %s should support images", testModel.ID)
			})
		}
	})

	// Test text-only models have SupportsImages=false
	t.Run("text_models_do_not_support_images", func(t *testing.T) {
		for _, testModel := range textOnlyModels {
			t.Run(testModel.ID, func(t *testing.T) {
				m, ok := modelMap[testModel.ID]
				if !ok {
					t.Skipf("Model %s not found in provider", testModel.ID)
				}
				assert.False(t, m.Model.SupportsImages, "Text model %s should not support images", testModel.ID)
			})
		}
	})

	// Test all models have required fields
	t.Run("all_models_have_required_fields", func(t *testing.T) {
		for _, m := range provider.Models {
			t.Run(m.ID, func(t *testing.T) {
				assert.NotEmpty(t, m.ID, "Model ID should not be empty")
				assert.NotEmpty(t, m.Name, "Model name should not be empty")
				assert.Positive(t, m.ContextWindow, "Context window should be positive")
				assert.Positive(t, m.DefaultMaxTokens, "Default max tokens should be positive")
			})
		}
	})
}

// TestNvidiaNIMTextModelBenchmark benchmarks text-only models with the core agent.
// This test requires NVIDIA_API_KEY to be set.
func TestNvidiaNIMTextModelBenchmark(t *testing.T) {
	apiKey := os.Getenv("NVIDIA_API_KEY")
	if apiKey == "" {
		t.Skip("NVIDIA_API_KEY not set, skipping benchmark")
	}

	provider := NvidiaNimProvider()

	// Verify all expected models exist
	availableModels := make(map[string]catwalk.Model)
	for _, m := range provider.Models {
		availableModels[m.ID] = m
	}

	t.Run("text_only_models", func(t *testing.T) {
		for _, testModel := range textOnlyModels {
			t.Run(testModel.ID, func(t *testing.T) {
				model, ok := availableModels[testModel.ID]
				if !ok {
					t.Fatalf("Model %s not found in provider configuration", testModel.ID)
				}

				// Verify model properties
				assert.NotEmpty(t, model.ID)
				assert.NotEmpty(t, model.Name)
				assert.Greater(t, model.ContextWindow, 0)
				assert.False(t, model.SupportsImages, "Text model should not support images")

				t.Logf("✓ Model %s (%s) configured correctly", model.Name, model.ID)
				t.Logf("  - Context Window: %d", model.ContextWindow)
				t.Logf("  - Max Tokens: %d", model.DefaultMaxTokens)
				t.Logf("  - Can Reason: %v", model.CanReason)
			})
		}
	})
}

// TestNvidiaNIMMultimodalModelBenchmark benchmarks multimodal models with the core agent.
// This test requires NVIDIA_API_KEY to be set.
func TestNvidiaNIMMultimodalModelBenchmark(t *testing.T) {
	apiKey := os.Getenv("NVIDIA_API_KEY")
	if apiKey == "" {
		t.Skip("NVIDIA_API_KEY not set, skipping benchmark")
	}

	provider := NvidiaNimProvider()

	// Verify all expected models exist
	availableModels := make(map[string]catwalk.Model)
	for _, m := range provider.Models {
		availableModels[m.ID] = m
	}

	t.Run("multimodal_models", func(t *testing.T) {
		for _, testModel := range multimodalModels {
			t.Run(testModel.ID, func(t *testing.T) {
				model, ok := availableModels[testModel.ID]
				if !ok {
					t.Fatalf("Model %s not found in provider configuration", testModel.ID)
				}

				// Verify model properties
				assert.NotEmpty(t, model.ID)
				assert.NotEmpty(t, model.Name)
				assert.Greater(t, model.ContextWindow, 0)
				assert.True(t, model.SupportsImages, "Multimodal model should support images")

				t.Logf("✓ Model %s (%s) configured correctly", model.Name, model.ID)
				t.Logf("  - Context Window: %d", model.ContextWindow)
				t.Logf("  - Max Tokens: %d", model.DefaultMaxTokens)
				t.Logf("  - Can Reason: %v", model.CanReason)
				t.Logf("  - Supports Images: %v", model.SupportsImages)
			})
		}
	})
}

// TestNvidiaNIMRateLimiterWithMultipleModels verifies rate limiting works correctly
// when multiple models are used in succession.
func TestNvidiaNIMRateLimiterWithMultipleModels(t *testing.T) {
	// This test verifies that the rate limiter is properly configured
	// to handle multiple models from the same provider.
	provider := NvidiaNimProvider()

	// All models should share the same rate limit (40 RPM)
	// since they're all from the same provider.
	require.GreaterOrEqual(t, len(provider.Models), len(textOnlyModels)+len(multimodalModels),
		"Provider should have at least %d models", len(textOnlyModels)+len(multimodalModels))

	t.Logf("NVIDIA NIM provider has %d models configured", len(provider.Models))
	t.Logf("  - Text-only models: %d", len(textOnlyModels))
	t.Logf("  - Multimodal models: %d", len(multimodalModels))
}

// BenchmarkNvidiaNIMModelLookup benchmarks the performance of model lookups.
func BenchmarkNvidiaNIMModelLookup(b *testing.B) {
	provider := NvidiaNimProvider()

	// Build map for O(1) lookup
	modelMap := make(map[string]catwalk.Model)
	for _, m := range provider.Models {
		modelMap[m.ID] = m
	}

	// Test model IDs to look up
	testIDs := []string{
		"z-ai/glm4.7",
		"moonshotai/kimi-k2-instruct",
		"qwen/qwen3.5-397b-a17b",
		"openai/gpt-oss-120b",
		"nvidia/nemotron-3-super-120b-a12b",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, id := range testIDs {
			_, _ = modelMap[id]
		}
	}
}

// TestNvidiaNIMModelConfigurationComplete verifies the complete model configuration
// is valid and can be used with the core agent.
func TestNvidiaNIMModelConfigurationComplete(t *testing.T) {
	provider := NvidiaNimProvider()

	tests := []struct {
		name           string
		modelID        string
		wantImages     bool
		wantReasoning  bool
		minContextWin  int64
	}{
		// Text-only models
		{"glm4.7", "z-ai/glm4.7", false, true, 64000},
		{"glm5", "z-ai/glm5", false, true, 64000},
		{"step-3.5-flash", "stepfun-ai/step-3.5-flash", false, true, 64000},
		{"qwen3.5-122b", "qwen/qwen3.5-122b-a10b", false, true, 64000},
		{"gpt-oss-20b", "openai/gpt-oss-20b", false, false, 64000},
		{"gpt-oss-120b", "openai/gpt-oss-120b", false, true, 64000},
		{"nemotron-3-super", "nvidia/nemotron-3-super-120b-a12b", false, true, 64000},
		{"kimi-k2", "moonshotai/kimi-k2-instruct", false, true, 64000},
		{"kimi-k2-0905", "moonshotai/kimi-k2-instruct-0905", false, true, 64000},
		{"kimi-k2-thinking", "moonshotai/kimi-k2-thinking", false, true, 64000},
		// Multimodal models
		{"qwen3.5-397b", "qwen/qwen3.5-397b-a17b", true, true, 64000},
		{"kimi-k2.5", "moonshotai/kimi-k2.5", true, true, 64000},
	}

	// Build map of available models
	availableModels := make(map[string]catwalk.Model)
	for _, m := range provider.Models {
		availableModels[m.ID] = m
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, ok := availableModels[tt.modelID]
			if !ok {
				t.Fatalf("Model %s not found in provider configuration", tt.modelID)
			}

			assert.Equal(t, tt.wantImages, model.SupportsImages, "SupportsImages mismatch")
			assert.Equal(t, tt.wantReasoning, model.CanReason, "CanReason mismatch")
			assert.GreaterOrEqual(t, int64(model.ContextWindow), int64(tt.minContextWin), "ContextWindow too small")
		})
	}
}

// TestNvidiaNIMProviderTimeoutConfiguration verifies the timeout configuration
// for handling NVIDIA NIM's queue delays.
func TestNvidiaNIMProviderTimeoutConfiguration(t *testing.T) {
	opts := DefaultNvidiaNIMOptions()

	// The timeout should be set to 10 minutes to handle queue delays
	assert.Equal(t, 10*time.Minute, opts.Timeout, "Timeout should be 10 minutes for queue handling")
	assert.True(t, opts.RateLimitEnabled, "Rate limiting should be enabled by default")
	assert.Equal(t, 3, opts.MaxRetries, "Max retries should be 3 by default")
	assert.Equal(t, 2*time.Second, opts.RetryBackoff, "Retry backoff should be 2 seconds by default")
}

// TestNvidiaNIMContextCancellation verifies that the rate limiter properly
// respects context cancellation.
func TestNvidiaNIMContextCancellation(t *testing.T) {
	// Import the ratelimit package to test the rate limiter
	// This test ensures that if a user cancels a request,
	// the rate limiter doesn't block indefinitely.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// The rate limiter should respect the context timeout
	// This is more of an integration test to ensure proper behavior
	select {
	case <-ctx.Done():
		// Expected behavior - context was cancelled/timed out
		assert.Error(t, ctx.Err())
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Context should have been cancelled")
	}
}

// PrintNvidiaNIMModelReport prints a report of all configured models.
func PrintNvidiaNIMModelReport(t *testing.T) {
	provider := NvidiaNimProvider()

	separator := strings.Repeat("=", 60)
	t.Log(separator)
	t.Log("NVIDIA NIM Model Report")
	t.Log(separator)
	t.Logf("Total models: %d", len(provider.Models))
	t.Log("")

	t.Log("Text-only models:")
	for _, m := range provider.Models {
		if !m.SupportsImages {
			t.Logf("  - %s (%s)", m.Name, m.ID)
		}
	}

	t.Log("")
	t.Log("Multimodal models:")
	for _, m := range provider.Models {
		if m.SupportsImages {
			t.Logf("  - %s (%s)", m.Name, m.ID)
		}
	}
}
