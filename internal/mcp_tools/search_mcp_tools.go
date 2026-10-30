package mcp_tools

import (
	"context"
	"encoding/json"
	"strings"
)

type SearchMCPArgs struct {
	Queries []string `json:"queries" jsonschema:"required,minItems=1,maxItems=5,description=1 to 5 short capability-and-action search intents; synonymous expressions may be provided; e.g. volume query and volume list. Do not include specific novel names or volume names" validate:"min=1,max=5,dive,required"`
}

// SearchMCPTool 从当前注册表搜索未直接向模型展示的拓展工具。
type SearchMCPTool struct {
	registry *Registry
}

func (t *SearchMCPTool) Name() string { return "search_mcp" }
func (t *SearchMCPTool) Description() string {
	return "Search extension tools by capability and action, returning up to 5 candidates with their names, brief descriptions, and schema paths. Use read to load the chosen tool's schema, then call it via run_mcp; if there are no results, search with different terms or read mcp/index.md to browse."
}
func (t *SearchMCPTool) Category() ToolCategory      { return CategoryWritingAssistant }
func (t *SearchMCPTool) JSONSchema() json.RawMessage { return SchemaOf(SearchMCPArgs{}) }
func (t *SearchMCPTool) ExposeToLLM() bool           { return true }
func (t *SearchMCPTool) NewArgs() any                { return &SearchMCPArgs{} }

func (t *SearchMCPTool) Execute(_ context.Context, args any, _ ToolContext) (*ToolResult, error) {
	a := args.(*SearchMCPArgs)
	for _, query := range a.Queries {
		if strings.TrimSpace(query) == "" {
			return &ToolResult{Success: false, Error: "поисковый запрос не может быть пустым"}, nil
		}
	}
	var output strings.Builder
	matches := t.registry.searchCatalog(a.Queries)
	if len(matches) == 0 {
		output.WriteString("No matching extension tools found. Try searching with different terms, or read mcp/index.md to browse.")
	} else {
		output.WriteString("# Candidate Extension Tools\n\nFirst read the chosen tool's schema, then call it via run_mcp.")
		for _, tool := range matches {
			writeCatalogEntry(&output, tool)
		}
	}
	return &ToolResult{Success: true, Data: map[string]any{"content": output.String()}}, nil
}

func RegisterSearchMCPTools(r *Registry) {
	r.Register(&SearchMCPTool{registry: r})
}
