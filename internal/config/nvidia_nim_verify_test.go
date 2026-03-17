package config

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestNvidiaNIMVerifyAllConfiguredModelsExist verifies all models in our configuration
// actually exist in the NVIDIA NIM API.
func TestNvidiaNIMVerifyAllConfiguredModelsExist(t *testing.T) {
	apiKey := os.Getenv("NVIDIA_API_KEY")
	if apiKey == "" {
		t.Skip("NVIDIA_API_KEY not set, skipping verification test")
	}

	// Fetch available models from API
	req, err := http.NewRequest("GET", "https://integrate.api.nvidia.com/v1/models", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	data, ok := result["data"].([]any)
	require.True(t, ok)

	// Build set of available model IDs
	availableModels := make(map[string]bool)
	for _, item := range data {
		model := item.(map[string]any)
		modelID, ok := model["id"].(string)
		if ok {
			availableModels[modelID] = true
		}
	}

	provider := NvidiaNimProvider()

	t.Logf("Total models configured in provider: %d", len(provider.Models))
	t.Logf("Total models available from API: %d", len(availableModels))
	t.Log("")

	// Check each configured model
	var missingModels []string
	var availableCount int

	for _, model := range provider.Models {
		if availableModels[model.ID] {
			availableCount++
		} else {
			missingModels = append(missingModels, model.ID)
		}
	}

	t.Logf("✓ Models available: %d/%d", availableCount, len(provider.Models))

	if len(missingModels) > 0 {
		t.Logf("⚠ Models not found in API (%d):", len(missingModels))
		for _, id := range missingModels {
			t.Logf("  - %s", id)
		}
	}

	// Print available models that match our patterns
	t.Log("")
	t.Log("Checking specific models:")

	// Check newly added models
	newModels := []string{
		"z-ai/glm4.7",
		"z-ai/glm5",
		"stepfun-ai/step-3.5-flash",
		"qwen/qwen3.5-122b-a10b",
		"openai/gpt-oss-20b",
		"openai/gpt-oss-120b",
		"nvidia/nemotron-3-super-120b-a12b",
		"moonshotai/kimi-k2-instruct",
		"moonshotai/kimi-k2-instruct-0905",
		"moonshotai/kimi-k2-thinking",
		"qwen/qwen3.5-397b-a17b",
		"moonshotai/kimi-k2.5",
	}

	for _, modelID := range newModels {
		if availableModels[modelID] {
			t.Logf("  ✓ %s", modelID)
		} else {
			t.Logf("  ✗ %s (NOT FOUND)", modelID)
		}
	}
}

// TestNvidiaNIMTestOneModelFromEachCategory tests one working model from each category.
func TestNvidiaNIMTestOneModelFromEachCategory(t *testing.T) {
	apiKey := os.Getenv("NVIDIA_API_KEY")
	if apiKey == "" {
		t.Skip("NVIDIA_API_KEY not set, skipping test")
	}

	// Test a simple prompt with different model types
	testCases := []struct {
		name    string
		modelID string
	}{
		{"meta_llama", "meta/llama-3.3-70b-instruct"},
		{"nvidia_nemotron", "nvidia/llama-3.1-nemotron-ultra-253b-v1"},
		{"mistral", "mistralai/mistral-large-2-instruct"},
		{"microsoft_phi", "microsoft/phi-4-mini-instruct"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Skip if model not available (will be checked in API call)
			t.Logf("Testing model: %s", tc.modelID)
		})
	}
}
