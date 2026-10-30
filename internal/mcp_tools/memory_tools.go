//go:build cgo

package mcp_tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sigpanic/goink/internal/chapter"
	"github.com/sigpanic/goink/internal/rag"
)

// ── search_story_memory ──────────────────────────────────

// SearchStoryMemoryArgs 是 search_story_memory 的参数。
type SearchStoryMemoryArgs struct {
	Query        string   `json:"query" jsonschema:"required,description=semantic search query; describe what you want to find in natural language" validate:"required"`
	TopK         int      `json:"top_k" jsonschema:"description=number of results to return,default=5,minimum=1,maximum=20" validate:"omitempty,min=1,max=20"`
	MinRelevance float64  `json:"min_relevance" jsonschema:"description=relevance threshold 0-1,default=0.5" validate:"omitempty,min=0,max=1"`
	ChapterIDs   []int64  `json:"chapter_ids" jsonschema:"description=restrict to chapter IDs; empty means no restriction" validate:"omitempty,dive,min=1"`
	ChunkTypes   []string `json:"chunk_types" jsonschema:"description=restrict to chunk types: summary / chapter_brief / content; empty means all"`
}

// SearchStoryMemoryTool 语义检索小说记忆。
type SearchStoryMemoryTool struct{}

func (t *SearchStoryMemoryTool) Name() string           { return "search_story_memory" }
func (t *SearchStoryMemoryTool) Description() string    { return searchStoryMemoryDescription }
func (t *SearchStoryMemoryTool) Category() ToolCategory { return CategoryMemoryRetrieval }

func (t *SearchStoryMemoryTool) JSONSchema() json.RawMessage {
	return SchemaOf(SearchStoryMemoryArgs{})
}
func (t *SearchStoryMemoryTool) ExposeToLLM() bool { return true }
func (t *SearchStoryMemoryTool) NewArgs() any      { return &SearchStoryMemoryArgs{} }

func (t *SearchStoryMemoryTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*SearchStoryMemoryArgs)

	if a.TopK == 0 {
		a.TopK = 5
	}
	if a.MinRelevance == 0 {
		a.MinRelevance = 0.5
	}

	vs := rag.GetVectorStore()
	if vs == nil {
		return &ToolResult{Success: false, Error: "векторный поиск недоступен: модель ONNX не установлена, сначала загрузите модель"}, nil
	}

	// 1. 过量获取，提高召回
	fetchK := min(a.TopK*2, 40)

	var filter *rag.SearchFilter
	if len(a.ChapterIDs) > 0 || len(a.ChunkTypes) > 0 {
		filter = &rag.SearchFilter{
			ChapterIDs: a.ChapterIDs,
			ChunkTypes: a.ChunkTypes,
		}
	}

	results, err := vs.Search(ctx, tc.NovelID, a.Query, fetchK, filter)
	if err != nil {
		return nil, fmt.Errorf("ошибка векторного поиска: %w", err)
	}

	// 2. 相关度过滤
	filtered := make([]rag.SearchResult, 0, len(results))
	for _, r := range results {
		if r.Relevance >= a.MinRelevance {
			filtered = append(filtered, r)
		}
	}

	if len(filtered) == 0 {
		return &ToolResult{
			Success: true,
			Data: map[string]any{
				"query":   a.Query,
				"total":   0,
				"message": "No relevant memory found. Try different query terms or lower the relevance threshold.",
			},
		}, nil
	}

	// 3. MMR 重排序
	reranked := rag.MMRRerank(a.Query, filtered, a.TopK, 0.7)

	// 4. 查询章节元数据
	chapters, err := chapter.NewStore(tc.DB, tc.LoggerOrDefault()).ListAllByNovel(ctx, nil, tc.NovelID)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса метаданных глав: %w", err)
	}
	chapMap := make(map[int64]chapter.Chapter, len(chapters))
	for _, ch := range chapters {
		chapMap[ch.ID] = ch
	}

	content, maxRelevance := formatSearchStoryMemoryResults(a.Query, reranked, chapMap)

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"query":         a.Query,
			"total":         len(reranked),
			"max_relevance": fmt.Sprintf("%.2f", maxRelevance),
			"content":       content,
		},
	}, nil
}

// formatSearchStoryMemoryResults 格式化搜索结果；chapterID 用于关联章节，ReadingNumber 仅供展示。
func formatSearchStoryMemoryResults(query string, results []rag.SearchResult, chapters map[int64]chapter.Chapter) (string, float64) {
	var sb strings.Builder
	sb.WriteString("## Semantic Search Results\n\n")
	fmt.Fprintf(&sb, "**Query:** %s  \n", query)
	fmt.Fprintf(&sb, "**Results:** %d", len(results))

	maxRelevance := 0.0
	for i, r := range results {
		if r.Relevance > maxRelevance {
			maxRelevance = r.Relevance
		}

		ch, ok := chapters[r.ChapterID]
		sourceLabel := chunkTypeLabel(r.SourceType)
		fmt.Fprintf(&sb, "\n\n### %d. ", i+1)
		if ok && ch.ReadingNumber > 0 {
			fmt.Fprintf(&sb, "Chapter %d %s [chapter_id:%d]", ch.ReadingNumber, ch.Title, ch.ID)
		} else {
			sb.WriteString("Unknown chapter")
		}
		fmt.Fprintf(&sb, " — %s (relevance: %.2f)\n\n", sourceLabel, r.Relevance)
		sb.WriteString(r.Content)
	}

	fmt.Fprintf(&sb, "\n\n> Max relevance: %.2f | Query: %s", maxRelevance, query)
	return sb.String(), maxRelevance
}

// chunkTypeLabel 将块类型转为中文标签。
func chunkTypeLabel(t string) string {
	switch t {
	case "summary":
		return "chapter summary"
	case "chapter_brief":
		return "chapter brief"
	case "content":
		return "content"
	default:
		return t
	}
}

const searchStoryMemoryDescription = `Semantically search the novel's memory, finding the text passages most relevant to the query within indexed chapter content.

Supported chunk types (chunk_types filter):
- summary: chapter summary (AI-generated high-density plot summary)
- chapter_brief: chapter brief (title + summary + opening of the body text)
- content: body content chunk (a 420-token text window)

Returns the source chapter, relevance score, and content text for each result. The relevance score is 0-1; higher means a better match.
Use this tool to look up a specific plot, dialogue, scene, or detail, rather than reading chapter files one by one.`

// ── 注册 ──────────────────────────────────────────────────

// RegisterMemoryTools 注册记忆检索工具。
func RegisterMemoryTools(r *Registry) {
	r.Register(&SearchStoryMemoryTool{})
}
