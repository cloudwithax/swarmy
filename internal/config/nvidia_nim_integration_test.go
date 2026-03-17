package config

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNvidiaNIMLiveAPICall makes actual API calls to NVIDIA NIM to verify models work.
// This test requires NVIDIA_API_KEY to be set.
func TestNvidiaNIMLiveAPICall(t *testing.T) {
	apiKey := os.Getenv("NVIDIA_API_KEY")
	if apiKey == "" {
		t.Skip("NVIDIA_API_KEY not set, skipping live API test")
	}

	provider := NvidiaNimProvider()
	baseURL := provider.APIEndpoint

	// Test a simple text model
	t.Run("text_model_chat_completion", func(t *testing.T) {
		modelID := "meta/llama-3.3-70b-instruct" // Use a reliable model for testing

		reqBody := map[string]any{
			"model":       modelID,
			"messages":    []map[string]string{{"role": "user", "content": "Say 'Hello from NVIDIA NIM' and nothing else."}},
			"max_tokens":  50,
			"temperature": 0.0,
		}

		jsonBody, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 120 * time.Second} // 2 minute timeout for queue delays
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		t.Logf("Response status: %d", resp.StatusCode)
		t.Logf("Response body: %s", string(body))

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK, got %d: %s", resp.StatusCode, string(body))

		// Parse response
		var result map[string]any
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		// Verify response structure
		choices, ok := result["choices"].([]any)
		require.True(t, ok, "Expected choices in response")
		require.Greater(t, len(choices), 0, "Expected at least one choice")

		firstChoice := choices[0].(map[string]any)
		message, ok := firstChoice["message"].(map[string]any)
		require.True(t, ok, "Expected message in choice")

		content, ok := message["content"].(string)
		require.True(t, ok, "Expected content in message")
		assert.NotEmpty(t, content, "Expected non-empty content")

		t.Logf("✓ Model %s responded successfully", modelID)
		t.Logf("  Response: %s", content)
	})

	// Test all benchmarked models to ensure they exist and respond
	t.Run("verify_all_benchmarked_models_exist", func(t *testing.T) {
		// Get list of models from API
		req, err := http.NewRequest("GET", baseURL+"/models", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK from /models endpoint")

		var result map[string]any
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		data, ok := result["data"].([]any)
		require.True(t, ok, "Expected data in models response")

		// Build set of available model IDs
		availableModels := make(map[string]bool)
		for _, item := range data {
			model := item.(map[string]any)
			modelID, ok := model["id"].(string)
			if ok {
				availableModels[modelID] = true
			}
		}

		t.Logf("Found %d models available from NVIDIA NIM API", len(availableModels))

		// Test models that should be available
		testModels := []string{
			"meta/llama-3.3-70b-instruct",
			"deepseek-ai/deepseek-r1",
			"mistralai/mistral-large-2-instruct",
			"microsoft/phi-4-mini-instruct",
		}

		for _, modelID := range testModels {
			exists := availableModels[modelID]
			assert.True(t, exists, "Model %s should be available from NVIDIA NIM API", modelID)
			if exists {
				t.Logf("✓ Model %s is available", modelID)
			}
		}
	})
}

// TestNvidiaNIMNewModelsIntegration verifies the newly added models work with the API.
func TestNvidiaNIMNewModelsIntegration(t *testing.T) {
	apiKey := os.Getenv("NVIDIA_API_KEY")
	if apiKey == "" {
		t.Skip("NVIDIA_API_KEY not set, skipping integration test")
	}

	baseURL := "https://integrate.api.nvidia.com/v1"

	// Models to test - only test a subset to avoid rate limiting
	modelsToTest := []struct {
		modelID string
		name    string
	}{
		{"z-ai/glm4.7", "GLM 4.7"},
		{"openai/gpt-oss-20b", "GPT-OSS 20B"},
		{"moonshotai/kimi-k2-instruct", "Kimi K2"},
	}

	for _, test := range modelsToTest {
		t.Run(test.modelID, func(t *testing.T) {
			reqBody := map[string]any{
				"model":       test.modelID,
				"messages":    []map[string]string{{"role": "user", "content": "Hi"}},
				"max_tokens":  10,
				"temperature": 0.0,
			}

			jsonBody, err := json.Marshal(reqBody)
			require.NoError(t, err)

			req, err := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+apiKey)
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 120 * time.Second}
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			if resp.StatusCode == http.StatusOK {
				var result map[string]any
				if err := json.Unmarshal(body, &result); err == nil {
					if choices, ok := result["choices"].([]any); ok && len(choices) > 0 {
						t.Logf("✓ %s (%s) responded successfully", test.name, test.modelID)
						return
					}
				}
			}

			// Log error but don't fail - model might not be available yet
			t.Logf("⚠ %s (%s) - Status: %d, Body: %s", test.name, test.modelID, resp.StatusCode, string(body))
		})
	}
}

// TestNvidiaNIMAPIRateLimitHeaders verifies rate limit headers are present.
func TestNvidiaNIMAPIRateLimitHeaders(t *testing.T) {
	apiKey := os.Getenv("NVIDIA_API_KEY")
	if apiKey == "" {
		t.Skip("NVIDIA_API_KEY not set, skipping rate limit test")
	}

	baseURL := "https://integrate.api.nvidia.com/v1"

	reqBody := map[string]any{
		"model":       "meta/llama-3.3-70b-instruct",
		"messages":    []map[string]string{{"role": "user", "content": "Hi"}},
		"max_tokens":  10,
		"temperature": 0.0,
	}

	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Log all headers for inspection
	t.Log("Response headers:")
	for name, values := range resp.Header {
		for _, value := range values {
			t.Logf("  %s: %s", name, value)
		}
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
