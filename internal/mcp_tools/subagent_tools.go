package mcp_tools

import (
	"context"
	"encoding/json"
)

// ── run_subagent ─────────────────────────────────────────

// RunSubagentArgs 是 run_subagent 的参数。
type RunSubagentArgs struct {
	AgentType   string `json:"agent_type" jsonschema:"required,enum=memory,enum=review,description=subagent type. memory: memory retrieval analyst that explores story data and compiles a report; review: chapter reviewer that performs a full quality review" validate:"required,oneof=memory review"`
	Instruction string `json:"instruction" jsonschema:"required,description=task instruction for the subagent, describing the specific work to complete" validate:"required"`
}

// RunSubagentTool 启动子 Agent 执行专项任务，返回最终报告。
type RunSubagentTool struct{}

func (t *RunSubagentTool) Name() string           { return "run_subagent" }
func (t *RunSubagentTool) Description() string    { return runSubagentDescription }
func (t *RunSubagentTool) Category() ToolCategory { return CategoryWritingAssistant }

func (t *RunSubagentTool) JSONSchema() json.RawMessage { return SchemaOf(RunSubagentArgs{}) }
func (t *RunSubagentTool) ExposeToLLM() bool           { return true }
func (t *RunSubagentTool) NewArgs() any                { return &RunSubagentArgs{} }

func (t *RunSubagentTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*RunSubagentArgs)

	if tc.RunSubAgent == nil {
		return &ToolResult{
			Success: false,
			Error:   "раннер суб-Agent не настроен",
		}, nil
	}

	report, err := tc.RunSubAgent(ctx, SubAgentRequest{
		AgentType:   a.AgentType,
		NovelID:     tc.NovelID,
		Instruction: a.Instruction,
		ToolID:      tc.ToolID,
	})
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"agent_type": a.AgentType,
			"report":     report,
		},
	}, nil
}

const runSubagentDescription = `Launch a specialized subagent to perform a task; the subagent runs multiple rounds of thinking independently and then returns a report.

Agent types:
- memory: memory retrieval analyst. Can search story memory and query all data such as characters / timeline / arcs / locations / reader cognition, integrating scattered information into a coherent report. Use for scenarios requiring multi-dimensional retrieval and deep analysis.
- review: chapter reviewer. Can read chapter content and query reference data such as character settings / foreshadowing / arcs, checking character consistency, plot logic, foreshadowing management, reader cognition, and arc progression item by item, and outputting review comments.

instruction should clearly describe the task goal and expected output.`

// ── 注册 ──────────────────────────────────────────────────

// RegisterSubagentTools 注册子 Agent 工具。
func RegisterSubagentTools(r *Registry) {
	r.Register(&RunSubagentTool{})
}
