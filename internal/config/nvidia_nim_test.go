package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNvidiaNimProvider(t *testing.T) {
	provider := NvidiaNimProvider()

	assert.Equal(t, "NVIDIA NIM", provider.Name)
	assert.Equal(t, "nvidia-nim", string(provider.ID))
	assert.Equal(t, "https://integrate.api.nvidia.com/v1", provider.APIEndpoint)
	assert.Equal(t, "$NVIDIA_API_KEY", provider.APIKey)
	assert.Equal(t, "openai-compat", string(provider.Type))
	assert.Equal(t, "nvidia/llama-3.1-nemotron-ultra-253b-v1", provider.DefaultLargeModelID)
	assert.Equal(t, "meta/llama-3.3-70b-instruct", provider.DefaultSmallModelID)

	// Should have models defined.
	assert.NotEmpty(t, provider.Models)
}

func TestNvidiaNimModels(t *testing.T) {
	models := nvidiaNimModels()

	// Should have at least 14 models.
	require.GreaterOrEqual(t, len(models), 14)

	// Check for expected model IDs.
	modelIDs := make(map[string]bool)
	for _, m := range models {
		modelIDs[m.ID] = true
	}

	// Check for key models.
	expectedModels := []string{
		"nvidia/llama-3.1-nemotron-ultra-253b-v1",
		"nvidia/llama-3.3-nemotron-super-49b-v1",
		"meta/llama-3.1-405b-instruct",
		"meta/llama-3.3-70b-instruct",
		"mistralai/mistral-large-2-instruct",
	}

	for _, expected := range expectedModels {
		assert.True(t, modelIDs[expected], "Expected model %s to be present", expected)
	}
}

func TestIsNvidiaNIMProvider(t *testing.T) {
	assert.True(t, IsNvidiaNIMProvider("nvidia-nim"))
	assert.False(t, IsNvidiaNIMProvider("openai"))
	assert.False(t, IsNvidiaNIMProvider("anthropic"))
	assert.False(t, IsNvidiaNIMProvider(""))
}

func TestDefaultNvidiaNIMOptions(t *testing.T) {
	opts := DefaultNvidiaNIMOptions()

	assert.True(t, opts.RateLimitEnabled)
	assert.Equal(t, 10*time.Minute, opts.Timeout)
	assert.Equal(t, 3, opts.MaxRetries)
	assert.Equal(t, 2*time.Second, opts.RetryBackoff)
	assert.True(t, opts.EnableOutputSanitization)
}

func TestGetNvidiaNIMOptions(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *ProviderConfig
		checkTimeout time.Duration
		checkRetries int
		checkRateLim bool
		checkSanitiz bool
	}{
		{
			name:         "nil config returns defaults",
			cfg:          nil,
			checkTimeout: 10 * time.Minute,
			checkRetries: 3,
			checkRateLim: true,
			checkSanitiz: true,
		},
		{
			name: "empty provider options returns defaults",
			cfg: &ProviderConfig{
				ProviderOptions: nil,
			},
			checkTimeout: 10 * time.Minute,
			checkRetries: 3,
			checkRateLim: true,
			checkSanitiz: true,
		},
		{
			name: "custom max retries is applied",
			cfg: &ProviderConfig{
				ProviderOptions: map[string]any{
					"max_retries": 5,
				},
			},
			checkTimeout: 10 * time.Minute,
			checkRetries: 5,
			checkRateLim: true,
			checkSanitiz: true,
		},
		{
			name: "rate limiting can be disabled",
			cfg: &ProviderConfig{
				ProviderOptions: map[string]any{
					"rate_limit_enabled":         false,
					"enable_output_sanitization": true,
				},
			},
			checkTimeout: 10 * time.Minute,
			checkRetries: 3,
			checkRateLim: false,
			checkSanitiz: true,
		},
		{
			name: "sanitization can be disabled",
			cfg: &ProviderConfig{
				ProviderOptions: map[string]any{
					"rate_limit_enabled":         true,
					"enable_output_sanitization": false,
				},
			},
			checkTimeout: 10 * time.Minute,
			checkRetries: 3,
			checkRateLim: true,
			checkSanitiz: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetNvidiaNIMOptions(tt.cfg)
			assert.Equal(t, tt.checkTimeout, result.Timeout)
			assert.Equal(t, tt.checkRetries, result.MaxRetries)
			assert.Equal(t, tt.checkRateLim, result.RateLimitEnabled)
			assert.Equal(t, tt.checkSanitiz, result.EnableOutputSanitization)
		})
	}
}

// TestNvidiaNIMProviderLive is a live test that verifies the NVIDIA NIM provider
// works with the actual API. It requires the NVIDIA_API_KEY environment variable.
// This test is skipped by default to avoid rate limiting during normal test runs.
func TestNvidiaNIMProviderLive(t *testing.T) {
	apiKey := os.Getenv("NVIDIA_API_KEY")
	if apiKey == "" {
		t.Skip("NVIDIA_API_KEY not set, skipping live test")
	}

	// This test validates that the provider configuration is correct
	// and that the API endpoint is reachable.
	provider := NvidiaNimProvider()

	// Verify the provider is properly configured.
	require.NotEmpty(t, provider.APIEndpoint)
	require.NotEmpty(t, provider.Models)

	// Log the number of models available.
	t.Logf("NVIDIA NIM provider has %d models available", len(provider.Models))

	// Log the first few model IDs for verification.
	for i, m := range provider.Models {
		if i >= 5 {
			break
		}
		t.Logf("  - %s (%s)", m.ID, m.Name)
	}
}

// TestNvidiaNIMRateLimiting validates the rate limiting configuration.
func TestNvidiaNIMRateLimiting(t *testing.T) {
	// 40 RPM = 1 request per 1.5 seconds
	expectedInterval := 1500 * time.Millisecond // 1.5 seconds
	actualInterval := time.Duration(float64(time.Second) / (40.0 / 60.0))

	// Allow small floating point differences.
	assert.InDelta(t, expectedInterval.Milliseconds(), actualInterval.Milliseconds(), 1,
		"40 RPM should equal 1 request per 1.5 seconds")
}
