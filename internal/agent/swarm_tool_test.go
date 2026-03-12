package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSwarmPlan(t *testing.T) {
	t.Run("deduplicates normalizes and caps files", func(t *testing.T) {
		files := parseSwarmPlan("/repo", "```\n- internal/agent/coordinator.go\ninternal/agent/coordinator.go\n/home/user/outside.go\n1. internal/config/config.go\n../hack.go\n```", 2)
		require.Equal(t, []string{"internal/agent/coordinator.go", "internal/config/config.go"}, files)
	})

	t.Run("ignores none", func(t *testing.T) {
		files := parseSwarmPlan("/repo", "NONE", 8)
		require.Empty(t, files)
	})
}

func TestBuildSwarmSummary(t *testing.T) {
	summary := buildSwarmSummary([]string{"a.go", "b.go"}, []swarmWorkerResult{
		{filePath: "a.go", content: "updated imports", isError: false},
		{filePath: "b.go", content: "no changes applied", isError: true},
	})

	require.Contains(t, summary, "Planned files:")
	require.Contains(t, summary, "- a.go")
	require.Contains(t, summary, "[ok] a.go: updated imports")
	require.Contains(t, summary, "[error] b.go: no changes applied")
}
