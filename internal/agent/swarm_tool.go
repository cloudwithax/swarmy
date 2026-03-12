package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/swarmy/internal/agent/prompt"
	"github.com/charmbracelet/swarmy/internal/agent/tools"
	"github.com/charmbracelet/swarmy/internal/config"
	"golang.org/x/sync/errgroup"
)

const SwarmToolName = "swarm"

const swarmToolDescription = "Plan file ownership for a coding task and execute one worker agent per file. Use this for multi-file edits when you want Swarmy to fan work out across a file-level swarm."

type SwarmParams struct {
	Prompt string `json:"prompt" description:"The coding task to distribute across the swarm"`
}

type swarmWorkerResult struct {
	filePath string
	content  string
	isError  bool
}

func (c *coordinator) swarmTool(ctx context.Context) (fantasy.AgentTool, error) {
	plannerCfg, ok := c.cfg.Agents[config.AgentTask]
	if !ok {
		return nil, fmt.Errorf("task agent not configured")
	}

	coderCfg, ok := c.cfg.Agents[config.AgentCoder]
	if !ok {
		return nil, errCoderAgentNotConfigured
	}

	plannerPrompt, err := taskPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
	if err != nil {
		return nil, err
	}
	plannerAgent, err := c.buildStaticAgent(ctx, plannerPrompt, plannerCfg, true, false)
	if err != nil {
		return nil, err
	}

	workerPrompt, err := coderPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
	if err != nil {
		return nil, err
	}
	workerCfg := coderCfg
	workerCfg.AllowedTools = removeTools(workerCfg.AllowedTools, SwarmToolName, AgentToolName, tools.AgenticFetchToolName)
	workerAgent, err := c.buildStaticAgent(ctx, workerPrompt, workerCfg, true, false)
	if err != nil {
		return nil, err
	}

	return fantasy.NewParallelAgentTool(
		SwarmToolName,
		swarmToolDescription,
		func(ctx context.Context, params SwarmParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if strings.TrimSpace(params.Prompt) == "" {
				return fantasy.NewTextErrorResponse("prompt is required"), nil
			}

			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session id missing from context")
			}

			agentMessageID := tools.GetMessageFromContext(ctx)
			if agentMessageID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("agent message id missing from context")
			}

			swarmCfg := c.cfg.SwarmOptions()
			maxFiles := swarmCfg.MaxPlannedFiles()

			planResponse, err := c.runSubAgent(ctx, subAgentParams{
				Agent:          plannerAgent,
				SessionID:      sessionID,
				AgentMessageID: agentMessageID,
				ToolCallID:     call.ID,
				Prompt:         buildSwarmPlannerPrompt(params.Prompt, maxFiles),
				SessionTitle:   "Swarm Planner",
			})
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("swarm planner failed: %v", err)), nil
			}
			if planResponse.IsError {
				return planResponse, nil
			}

			files := parseSwarmPlan(c.cfg.WorkingDir(), planResponse.Content, maxFiles)
			if len(files) == 0 {
				return fantasy.NewTextResponse("Swarm planner selected no files. Continue with direct tools if changes are still needed."), nil
			}

			results := make([]swarmWorkerResult, len(files))
			group, groupCtx := errgroup.WithContext(ctx)
			group.SetLimit(swarmCfg.MaxWorkers())

			for i, filePath := range files {
				i := i
				filePath := filePath
				group.Go(func() error {
					resp, err := c.runSubAgent(groupCtx, subAgentParams{
						Agent:          workerAgent,
						SessionID:      sessionID,
						AgentMessageID: agentMessageID,
						ToolCallID:     fmt.Sprintf("%s-worker-%d", call.ID, i+1),
						Prompt:         buildSwarmWorkerPrompt(params.Prompt, filePath),
						SessionTitle:   fmt.Sprintf("Swarm Worker: %s", filepath.Base(filePath)),
					})
					if err != nil {
						return err
					}
					results[i] = swarmWorkerResult{
						filePath: filePath,
						content:  strings.TrimSpace(resp.Content),
						isError:  resp.IsError,
					}
					return nil
				})
			}

			if err := group.Wait(); err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("swarm worker failed: %v", err)), nil
			}

			return fantasy.NewTextResponse(buildSwarmSummary(files, results)), nil
		},
	), nil
}

func buildSwarmPlannerPrompt(userPrompt string, maxFiles int) string {
	return fmt.Sprintf("Plan a file-level coding swarm for the following request. Use read-only tools to inspect the codebase and identify the smallest set of files that should be modified. Return only relative file paths, one per line, with no bullets, no code fences, and no prose. Return NONE if no file changes are required. Limit the plan to at most %d files.\n\nUser request:\n%s", maxFiles, userPrompt)
}

func buildSwarmWorkerPrompt(userPrompt, filePath string) string {
	return fmt.Sprintf("User request:\n%s\n\nYou are the worker responsible for exactly one file: %s\n\nRequirements:\n- Read any other files you need for context.\n- Only modify %s.\n- Complete the file-local changes needed for the user request.\n- If changes are needed outside %s, mention that briefly in your final response instead of editing other files.\n- After making changes, summarize what you changed in %s.", userPrompt, filePath, filePath, filePath, filePath)
}

func parseSwarmPlan(workingDir, content string, maxFiles int) []string {
	lines := strings.Split(content, "\n")
	seen := make(map[string]struct{})
	files := make([]string, 0, maxFiles)

	for _, raw := range lines {
		line := sanitizeSwarmPlanLine(raw)
		if line == "" || strings.EqualFold(line, "none") {
			continue
		}

		normalized, ok := normalizeSwarmPlanPath(workingDir, line)
		if !ok {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}

		seen[normalized] = struct{}{}
		files = append(files, normalized)
		if len(files) >= maxFiles {
			break
		}
	}

	return files
}

func sanitizeSwarmPlanLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.Trim(line, "`")
	line = strings.TrimPrefix(line, "- ")
	line = strings.TrimPrefix(line, "* ")
	line = strings.TrimSpace(line)

	for i := 1; i <= 9; i++ {
		prefix := fmt.Sprintf("%d. ", i)
		if strings.HasPrefix(line, prefix) {
			line = strings.TrimSpace(strings.TrimPrefix(line, prefix))
			break
		}
	}

	if strings.HasPrefix(line, "```") {
		return ""
	}

	return line
}

func normalizeSwarmPlanPath(workingDir, candidate string) (string, bool) {
	cleaned := filepath.Clean(candidate)
	if cleaned == "." || cleaned == "" {
		return "", false
	}

	if filepath.IsAbs(cleaned) {
		rel, err := filepath.Rel(workingDir, cleaned)
		if err != nil {
			return "", false
		}
		cleaned = rel
	}

	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", false
	}

	return filepath.ToSlash(cleaned), true
}

func buildSwarmSummary(files []string, results []swarmWorkerResult) string {
	var builder strings.Builder
	builder.WriteString("Swarm execution complete.\n")
	builder.WriteString("Planned files:\n")
	for _, filePath := range files {
		builder.WriteString("- ")
		builder.WriteString(filePath)
		builder.WriteString("\n")
	}

	builder.WriteString("\nWorker results:\n")
	for _, result := range results {
		status := "ok"
		if result.isError {
			status = "error"
		}
		builder.WriteString("[")
		builder.WriteString(status)
		builder.WriteString("] ")
		builder.WriteString(result.filePath)
		if result.content != "" {
			builder.WriteString(": ")
			builder.WriteString(result.content)
		}
		builder.WriteString("\n")
	}

	return strings.TrimSpace(builder.String())
}

func removeTools(tools []string, names ...string) []string {
	filtered := make([]string, 0, len(tools))
	for _, tool := range tools {
		if containsString(names, tool) {
			continue
		}
		filtered = append(filtered, tool)
	}
	return filtered
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
