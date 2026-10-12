package mcp_tools

import (
	"context"
	"encoding/json"
	"strings"
)

type SearchMCPArgs struct {
	Queries []string `json:"queries" jsonschema:"required,minItems=1,maxItems=5,description=1至5条简短的能力与动作搜索意图，可提供近义表达；例如分卷查询和卷列表。不要包含具体小说名或卷名" validate:"min=1,max=5,dive,required"`
}

// SearchMCPTool 从当前注册表搜索未直接向模型展示的拓展工具。
type SearchMCPTool struct {
	registry *Registry
}

func (t *SearchMCPTool) Name() string { return "search_mcp" }
func (t *SearchMCPTool) Description() string {
	return "按能力与动作搜索拓展工具，返回最多5个候选的工具名称、简述及 schema 路径。用 read 读取选定工具的 schema，再通过 run_mcp 调用；没有结果时可换词搜索或 read mcp/index.md 浏览。"
}
func (t *SearchMCPTool) Category() ToolCategory      { return CategoryWritingAssistant }
func (t *SearchMCPTool) JSONSchema() json.RawMessage { return SchemaOf(SearchMCPArgs{}) }
func (t *SearchMCPTool) ExposeToLLM() bool           { return true }
func (t *SearchMCPTool) NewArgs() any                { return &SearchMCPArgs{} }

func (t *SearchMCPTool) Execute(_ context.Context, args any, _ ToolContext) (*ToolResult, error) {
	a := args.(*SearchMCPArgs)
	for _, query := range a.Queries {
		if strings.TrimSpace(query) == "" {
			return &ToolResult{Success: false, Error: "搜索意图不能为空"}, nil
		}
	}
	var output strings.Builder
	matches := t.registry.searchCatalog(a.Queries)
	if len(matches) == 0 {
		output.WriteString("未找到匹配的拓展工具。请换词搜索，或 read mcp/index.md 浏览。")
	} else {
		output.WriteString("# 候选拓展工具\n\n先 read 选定工具的 schema，再用 run_mcp 调用。")
		for _, tool := range matches {
			writeCatalogEntry(&output, tool)
		}
	}
	return &ToolResult{Success: true, Data: map[string]any{"content": output.String()}}, nil
}

func RegisterSearchMCPTools(r *Registry) {
	r.Register(&SearchMCPTool{registry: r})
}
