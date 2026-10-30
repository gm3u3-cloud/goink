package mcp_tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// WebSearchArgs 是 web_search 工具的参数。
type WebSearchArgs struct {
	Prompt string `json:"prompt" jsonschema:"required,description=instruction sent to the search AI; describe your question and context in natural language. This is not a traditional search keyword, but a specific search instruction" validate:"required,min=1,max=500"`
}

// WebSearchTool 通过 DeepSeek Anthropic 端点执行网络搜索，返回模型总结和来源链接。
type WebSearchTool struct{}

func (t *WebSearchTool) Name() string           { return "web_search" }
func (t *WebSearchTool) Category() ToolCategory { return CategoryWritingAssistant }
func (t *WebSearchTool) ExposeToLLM() bool      { return true }

func (t *WebSearchTool) Description() string {
	return "Search the web for real information, returning a synthesized answer and reference sources. Use when you need real-time data, news, technical documentation, or content beyond the model's knowledge. " +
		"Search results are already analyzed and synthesized by the AI; you can directly cite the returned summary; sources is a list of source URLs. To view the original detail of a specific source, use web_fetch."
}

func (t *WebSearchTool) JSONSchema() json.RawMessage {
	return SchemaOf(WebSearchArgs{})
}

func (t *WebSearchTool) NewArgs() any { return &WebSearchArgs{} }

func (t *WebSearchTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*WebSearchArgs)

	if tc.WebSearch == nil {
		return &ToolResult{Error: "веб-поиск не включён: сначала настройте DeepSeek в настройках LLM"}, nil
	}

	result, err := tc.WebSearch(ctx, a.Prompt)
	if err != nil {
		return nil, fmt.Errorf("ошибка поиска: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"queries": result.Queries,
			"summary": result.Summary,
			"sources": result.Sources,
		},
	}, nil
}

// RegisterWebSearchTools 注册网络搜索工具。
func RegisterWebSearchTools(r *Registry) {
	r.Register(&WebSearchTool{})
}
