package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/tree"
	"github.com/charmbracelet/swarmy/internal/agent"
	"github.com/charmbracelet/swarmy/internal/message"
	"github.com/charmbracelet/swarmy/internal/ui/anim"
	"github.com/charmbracelet/swarmy/internal/ui/styles"
)

// maxVisibleNestedTools is the maximum number of nested tools to display.
const maxVisibleNestedTools = 10

// SwarmToolMessageItem is a message item that represents a swarm tool call.
type SwarmToolMessageItem struct {
	*baseToolMessageItem

	nestedTools []ToolMessageItem

	// workerLabels maps a worker tool-call-ID prefix (e.g.
	// "{swarmID}-worker-2") to a human-readable label like "2".
	workerLabels map[string]string

	// toolWorkerMap maps a nested tool's ID to the worker ID prefix it
	// belongs to so we can group them visually.
	toolWorkerMap map[string]string
}

var (
	_ ToolMessageItem     = (*SwarmToolMessageItem)(nil)
	_ NestedToolContainer = (*SwarmToolMessageItem)(nil)
)

// NewSwarmToolMessageItem creates a new [SwarmToolMessageItem].
func NewSwarmToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) *SwarmToolMessageItem {
	t := &SwarmToolMessageItem{
		workerLabels:  make(map[string]string),
		toolWorkerMap: make(map[string]string),
	}
	t.baseToolMessageItem = newBaseToolMessageItem(sty, toolCall, result, &SwarmToolRenderContext{swarm: t}, canceled)
	t.spinningFunc = func(state SpinningState) bool {
		return !state.HasResult() && !state.IsCanceled()
	}
	return t
}

// SetWorkerLabel registers a worker tool-call-ID prefix and its label.
func (s *SwarmToolMessageItem) SetWorkerLabel(workerPrefix, label string) {
	s.workerLabels[workerPrefix] = label
	s.clearCache()
}

// TagToolWithWorker associates a nested tool ID with the worker that owns it.
func (s *SwarmToolMessageItem) TagToolWithWorker(toolID, workerPrefix string) {
	s.toolWorkerMap[toolID] = workerPrefix
	s.clearCache()
}

// Animate progresses the message animation if it should be spinning.
func (s *SwarmToolMessageItem) Animate(msg anim.StepMsg) tea.Cmd {
	if s.result != nil || s.Status() == ToolStatusCanceled {
		return nil
	}
	if msg.ID == s.ID() {
		return s.anim.Animate(msg)
	}
	for _, nestedTool := range s.nestedTools {
		if msg.ID != nestedTool.ID() {
			continue
		}
		if a, ok := nestedTool.(Animatable); ok {
			return a.Animate(msg)
		}
	}
	return nil
}

// NestedTools returns the nested tools.
func (s *SwarmToolMessageItem) NestedTools() []ToolMessageItem {
	return s.nestedTools
}

// SetNestedTools sets the nested tools.
func (s *SwarmToolMessageItem) SetNestedTools(tools []ToolMessageItem) {
	s.nestedTools = tools
	s.clearCache()
}

// AddNestedTool adds a nested tool.
func (s *SwarmToolMessageItem) AddNestedTool(tool ToolMessageItem) {
	if c, ok := tool.(Compactable); ok {
		c.SetCompact(true)
	}
	s.nestedTools = append(s.nestedTools, tool)
	s.clearCache()
}

// SwarmToolRenderContext renders swarm tool messages.
type SwarmToolRenderContext struct {
	swarm *SwarmToolMessageItem
}

// workerGroup holds the label and tools for a single swarm worker.
type workerGroup struct {
	label string
	tools []ToolMessageItem
}

// RenderTool implements the [ToolRenderer] interface.
func (r *SwarmToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if !opts.ToolCall.Finished && !opts.IsCanceled() && len(r.swarm.nestedTools) == 0 {
		return pendingTool(sty, "Swarm", opts.Anim)
	}

	var params agent.SwarmParams
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

	prompt := params.Prompt
	prompt = strings.ReplaceAll(prompt, "\n", " ")

	header := toolHeader(sty, opts.Status, "Swarm", cappedWidth, opts.Compact)
	if opts.Compact {
		return header
	}

	// Build the task tag and prompt.
	taskTag := sty.Tool.AgentTaskTag.Render("Task")
	taskTagWidth := lipgloss.Width(taskTag)

	// Calculate remaining width for prompt.
	remainingWidth := min(cappedWidth-taskTagWidth-3, maxTextWidth-taskTagWidth-3)

	promptText := sty.Tool.AgentPrompt.Width(remainingWidth).Render(prompt)

	header = lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			taskTag,
			" ",
			promptText,
		),
	)

	// Build tree with nested tool calls grouped by worker.
	childTools := tree.Root(header)

	groups := r.groupByWorker()
	rendered := 0

	for _, g := range groups {
		if rendered >= maxVisibleNestedTools {
			break
		}

		workerHeader := fmt.Sprintf("Worker %s", g.label)
		workerBranch := tree.Root(sty.Tool.AgentTaskTag.Render(workerHeader))

		for _, nestedTool := range g.tools {
			if rendered >= maxVisibleNestedTools {
				break
			}
			childView := nestedTool.Render(remainingWidth)
			workerBranch.Child(childView)
			rendered++
		}

		childTools.Child(workerBranch)
	}

	// Render any untagged tools (planner or orphans) directly.
	untagged := r.untaggedTools()
	for _, nestedTool := range untagged {
		if rendered >= maxVisibleNestedTools {
			break
		}
		childView := nestedTool.Render(remainingWidth)
		childTools.Child(childView)
		rendered++
	}

	total := len(r.swarm.nestedTools)
	if total > maxVisibleNestedTools {
		overflow := fmt.Sprintf("… and %d more", total-maxVisibleNestedTools)
		childTools.Child(sty.Muted.Render(overflow))
	}

	// Build parts.
	var parts []string
	parts = append(parts, childTools.Enumerator(roundedEnumerator(2, taskTagWidth-5)).String())

	// Show animation if still running.
	if !opts.HasResult() && !opts.IsCanceled() {
		parts = append(parts, "", opts.Anim.Render())
	}

	result := lipgloss.JoinVertical(lipgloss.Left, parts...)

	// Add body content when completed.
	if opts.HasResult() && opts.Result.Content != "" {
		body := toolOutputMarkdownContent(sty, opts.Result.Content, cappedWidth-toolBodyLeftPaddingTotal, opts.ExpandedContent)
		return joinToolParts(result, body)
	}

	return result
}

// groupByWorker groups the nested tools by their worker label, preserving
// worker order by first appearance.
func (r *SwarmToolRenderContext) groupByWorker() []workerGroup {
	orderMap := make(map[string]int)
	groups := make([]workerGroup, 0, len(r.swarm.workerLabels))

	for _, t := range r.swarm.nestedTools {
		wp, ok := r.swarm.toolWorkerMap[t.ID()]
		if !ok {
			continue
		}
		idx, exists := orderMap[wp]
		if !exists {
			label := r.swarm.workerLabels[wp]
			if label == "" {
				label = wp
			}
			idx = len(groups)
			orderMap[wp] = idx
			groups = append(groups, workerGroup{label: label})
		}
		groups[idx].tools = append(groups[idx].tools, t)
	}

	return groups
}

// untaggedTools returns nested tools that are not associated with any worker.
func (r *SwarmToolRenderContext) untaggedTools() []ToolMessageItem {
	var out []ToolMessageItem
	for _, t := range r.swarm.nestedTools {
		if _, ok := r.swarm.toolWorkerMap[t.ID()]; !ok {
			out = append(out, t)
		}
	}
	return out
}
