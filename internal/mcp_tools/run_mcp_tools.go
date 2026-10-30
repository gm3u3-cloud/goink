package mcp_tools

import (
	"context"
	"encoding/json"
	"fmt"
)

type RunMCPArgs struct {
	ToolName string                     `json:"tool_name" jsonschema:"required,description=name of the extension tool to invoke" validate:"required"`
	Args     map[string]json.RawMessage `json:"args" jsonschema:"required,description=the argument object for the target tool; pass an empty object when there are no arguments" validate:"required"`
}

// RunMCPTool 执行未直接向模型展示的拓展工具，复用 Registry 的参数校验与执行上下文。
type RunMCPTool struct {
	registry *Registry
}

func (t *RunMCPTool) Name() string { return "run_mcp" }
func (t *RunMCPTool) Description() string {
	return "Invoke an extension tool by name. tool_name uses the target tool's original name, and args is filled in according to the target tool's parameter description; pass {} when there are no arguments. Common tools provided directly to the model should be called directly."
}
func (t *RunMCPTool) Category() ToolCategory      { return CategoryWritingAssistant }
func (t *RunMCPTool) JSONSchema() json.RawMessage { return SchemaOf(RunMCPArgs{}) }
func (t *RunMCPTool) ExposeToLLM() bool           { return true }
func (t *RunMCPTool) NewArgs() any                { return &RunMCPArgs{} }

func (t *RunMCPTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*RunMCPArgs)
	target, ok := t.registry.Get(a.ToolName)
	if !ok {
		return &ToolResult{Success: false, Error: "инструмент не найден: " + a.ToolName}, nil
	}
	if target.ExposeToLLM() {
		return &ToolResult{Success: false, Error: "этот инструмент не поддерживает вызов по требованию, вызовите его напрямую: " + a.ToolName}, nil
	}
	raw, err := json.Marshal(a.Args)
	if err != nil {
		return nil, fmt.Errorf("ошибка кодирования параметров инструмента по требованию: %w", err)
	}
	return t.registry.Execute(ctx, a.ToolName, raw, tc, nil), nil
}

func RegisterRunMCPTools(r *Registry) {
	r.Register(&RunMCPTool{registry: r})
}
