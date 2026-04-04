package agent

import (
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/assert"
)

func TestPromptTokensForSessionUsage_UsesTotalWhenAvailable(t *testing.T) {
	usage := fantasy.Usage{
		InputTokens:     12047,
		OutputTokens:    58,
		TotalTokens:     12105,
		CacheReadTokens: 11906,
	}

	// OpenAI-compatible providers report cached tokens as a detail of prompt
	// tokens, so totals should win to avoid double counting.
	assert.Equal(t, int64(12047), promptTokensForSessionUsage(usage))
}

func TestPromptTokensForSessionUsage_FallsBackToExplicitFields(t *testing.T) {
	usage := fantasy.Usage{
		InputTokens:         300,
		OutputTokens:        100,
		TotalTokens:         0,
		CacheCreationTokens: 50,
		CacheReadTokens:     200,
	}

	assert.Equal(t, int64(550), promptTokensForSessionUsage(usage))
}

func TestPromptTokensForSessionUsage_FallsBackWhenTotalIsInvalid(t *testing.T) {
	usage := fantasy.Usage{
		InputTokens:         20,
		OutputTokens:        25,
		TotalTokens:         10,
		CacheCreationTokens: 3,
		CacheReadTokens:     7,
	}

	assert.Equal(t, int64(30), promptTokensForSessionUsage(usage))
}
