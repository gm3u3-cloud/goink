package mcp_tools

import (
	"context"
	"encoding/json"

	"github.com/sigpanic/goink/internal/web"
)

// WebFetchArgs 是 web_fetch 工具的参数。
type WebFetchArgs struct {
	URL string `json:"url" jsonschema:"required,description=the web page URL to fetch" validate:"required,url,min=1,max=2048"`
}

// WebFetchTool 抓取网页内容，清洗后返回 markdown 格式正文。
type WebFetchTool struct{}

func (t *WebFetchTool) Name() string           { return "web_fetch" }
func (t *WebFetchTool) Category() ToolCategory { return CategoryWritingAssistant }
func (t *WebFetchTool) ExposeToLLM() bool      { return true }

func (t *WebFetchTool) Description() string {
	return "Fetch the body content of a specified web page, returning cleaned markdown text. Use when you need to read the original source of a reference, explore details, or verify web_search results. Only one URL can be fetched at a time."
}

func (t *WebFetchTool) JSONSchema() json.RawMessage {
	return SchemaOf(WebFetchArgs{})
}

func (t *WebFetchTool) NewArgs() any { return &WebFetchArgs{} }

func (t *WebFetchTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*WebFetchArgs)

	result, err := web.Fetch(a.URL)
	if err != nil {
		return &ToolResult{Error: "ошибка получения: " + err.Error()}, nil
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"url":   result.URL,
			"title": result.Title,
			"text":  result.Text,
		},
	}, nil
}

// RegisterWebFetchTools 注册网页抓取工具。
func RegisterWebFetchTools(r *Registry) {
	r.Register(&WebFetchTool{})
}
