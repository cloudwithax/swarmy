package app_test

import (
	"context"
	"os"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/cloudwithax/swarmy/internal/app"
	"github.com/cloudwithax/swarmy/internal/config"
	"github.com/cloudwithax/swarmy/internal/db"
	"github.com/cloudwithax/swarmy/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNvidiaNIMKimi25TokenAccountingLive(t *testing.T) {
	if os.Getenv("SWARMY_TEST_LIVE_NIM") == "" {
		t.Skip("set SWARMY_TEST_LIVE_NIM=1 to run live NVIDIA NIM token accounting test")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Minute)
	defer cancel()

	wd, err := os.Getwd()
	require.NoError(t, err)

	cfg, err := config.Init(wd, t.TempDir(), false)
	require.NoError(t, err)

	const providerID = "nvidia-nim"
	const modelID = "moonshotai/kimi-k2.5"

	providerCfg, ok := cfg.Providers.Get(providerID)
	require.True(t, ok, "provider %q must be configured", providerID)
	require.False(t, providerCfg.Disable, "provider %q is disabled", providerID)
	require.NotEmpty(t, providerCfg.APIKey, "provider %q has no resolved API key", providerID)

	model := cfg.GetModel(providerID, modelID)
	require.NotNil(t, model, "model %q not found in provider %q", modelID, providerID)

	temperature := 0.0
	cfg.Models[config.SelectedModelTypeLarge] = config.SelectedModel{
		Provider:    providerID,
		Model:       modelID,
		MaxTokens:   64,
		Temperature: &temperature,
	}
	cfg.Models[config.SelectedModelTypeSmall] = config.SelectedModel{
		Provider:    providerID,
		Model:       modelID,
		MaxTokens:   64,
		Temperature: &temperature,
	}

	conn, err := db.Connect(ctx, cfg.Options.DataDirectory)
	require.NoError(t, err)

	application, err := app.New(ctx, conn, cfg)
	require.NoError(t, err)
	defer application.Shutdown()

	sess, err := application.Sessions.Create(ctx, "NVIDIA NIM live token accounting smoke test")
	require.NoError(t, err)

	// Seed one message to skip title-generation usage so this assertion targets
	// only the call under test.
	_, err = application.Messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "seed"},
		},
	})
	require.NoError(t, err)

	before, err := application.Sessions.Get(ctx, sess.ID)
	require.NoError(t, err)

	result, err := application.AgentCoordinator.Run(
		ctx,
		sess.ID,
		"Reply with exactly PONG and nothing else.",
	)
	require.NoError(t, err)
	require.NotNil(t, result)

	after, err := application.Sessions.Get(ctx, sess.ID)
	require.NoError(t, err)

	usage := result.TotalUsage
	deltaPrompt := after.PromptTokens - before.PromptTokens
	deltaCompletion := after.CompletionTokens - before.CompletionTokens

	expectedPrompt := expectedPromptTokensForSessionUsage(usage)
	expectedCompletion := usage.OutputTokens

	assert.Equal(t, expectedPrompt, deltaPrompt, "prompt token accounting mismatch")
	assert.Equal(t, expectedCompletion, deltaCompletion, "completion token accounting mismatch")

	if usage.TotalTokens > 0 {
		assert.Equal(t, usage.TotalTokens, deltaPrompt+deltaCompletion, "total token accounting mismatch")
	}

	t.Logf(
		"usage={input=%d output=%d total=%d cache_read=%d cache_create=%d} session_delta={prompt=%d completion=%d}",
		usage.InputTokens,
		usage.OutputTokens,
		usage.TotalTokens,
		usage.CacheReadTokens,
		usage.CacheCreationTokens,
		deltaPrompt,
		deltaCompletion,
	)
}

func expectedPromptTokensForSessionUsage(usage fantasy.Usage) int64 {
	if usage.TotalTokens > 0 && usage.TotalTokens >= usage.OutputTokens {
		return usage.TotalTokens - usage.OutputTokens
	}
	return usage.InputTokens + usage.CacheCreationTokens + usage.CacheReadTokens
}
