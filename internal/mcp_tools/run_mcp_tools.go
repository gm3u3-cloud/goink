package mcp_tools

import (
	"context"
	"encoding/json"
	"fmt"
)

type RunMCPArgs struct {
	ToolName string                     `json:"tool_name" jsonschema:"required,description=要调用的拓展工具名称" validate:"required"`
	Args     map[string]json.RawMessage `json:"args" jsonschema:"required,description=目标工具的参数对象，无参数时传空对象" validate:"required"`
}

// RunMCPTool 执行未直接向模型展示的拓展工具，复用 Registry 的参数校验与执行上下文。
type RunMCPTool struct {
	registry *Registry
}

func (t *RunMCPTool) Name() string { return "run_mcp" }
func (t *RunMCPTool) Description() string {
	return "按名称调用拓展工具。tool_name 使用目标工具的原始名称，args 按目标工具的参数说明填写；无参数时传 {}。直接提供给模型的常用工具请直接调用。"
}
func (t *RunMCPTool) Category() ToolCategory      { return CategoryWritingAssistant }
func (t *RunMCPTool) JSONSchema() json.RawMessage { return SchemaOf(RunMCPArgs{}) }
func (t *RunMCPTool) ExposeToLLM() bool           { return true }
func (t *RunMCPTool) NewArgs() any                { return &RunMCPArgs{} }

func (t *RunMCPTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*RunMCPArgs)
	target, ok := t.registry.Get(a.ToolName)
	if !ok {
		return &ToolResult{Success: false, Error: "工具不存在: " + a.ToolName}, nil
	}
	if target.ExposeToLLM() {
		return &ToolResult{Success: false, Error: "该工具不支持按需调用，请直接调用: " + a.ToolName}, nil
	}
	raw, err := json.Marshal(a.Args)
	if err != nil {
		return nil, fmt.Errorf("编码按需工具参数: %w", err)
	}
	return t.registry.Execute(ctx, a.ToolName, raw, tc, nil), nil
}

func RegisterRunMCPTools(r *Registry) {
	r.Register(&RunMCPTool{registry: r})
}
