package mcp_tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sigpanic/goink/internal/chapter"
	"github.com/sigpanic/goink/internal/storage"
	"github.com/sigpanic/goink/internal/volume"
)

// ── get_chapter_list ─────────────────────────────────

// GetChapterListArgs 是 get_chapter_list 的参数。
type GetChapterListArgs struct {
	PageArgs // 嵌入分页参数
}

// GetChapterListTool 获取章节目录，按阅读顺序降序。
type GetChapterListTool struct{}

func (t *GetChapterListTool) Name() string { return "get_chapter_list" }
func (t *GetChapterListTool) Description() string {
	return "Get the novel's chapter directory, with pagination. Sorted by reading order in descending order (newest first), grouped by volume, showing each chapter's stable ID, live reading number, title, and word count."
}
func (t *GetChapterListTool) Category() ToolCategory { return CategoryNovelManagement }

func (t *GetChapterListTool) JSONSchema() json.RawMessage {
	return SchemaOf(GetChapterListArgs{})
}

func (t *GetChapterListTool) ExposeToLLM() bool { return true }
func (t *GetChapterListTool) NewArgs() any      { return &GetChapterListArgs{} }

func (t *GetChapterListTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*GetChapterListArgs)
	a.NormalizePage()

	chStore := chapter.NewStore(tc.DB, tc.LoggerOrDefault())
	result, err := chStore.ListByNovel(ctx, nil, tc.NovelID, chapter.ListByNovelOptions{
		PageParams: storage.PageParams{Page: a.Page, Size: a.Size},
		Order:      "desc",
	})
	if err != nil {
		return nil, fmt.Errorf("ошибка перечисления глав: %w", err)
	}

	volumes, err := volume.NewStore(tc.DB, tc.LoggerOrDefault()).ListByNovel(ctx, nil, tc.NovelID)
	if err != nil {
		return nil, fmt.Errorf("ошибка перечисления томов: %w", err)
	}
	volumeNames := make(map[int64]string, len(volumes))
	for _, v := range volumes {
		volumeNames[v.ID] = v.Name
	}

	data := PageMeta(result)
	data["content"] = formatChapterList(result.Items, volumeNames)

	return &ToolResult{
		Success: true,
		Data:    data,
	}, nil
}

func formatChapterList(chapters []chapter.Chapter, volumeNames map[int64]string) string {
	if len(chapters) == 0 {
		return "No chapters available."
	}

	var sb strings.Builder
	var previousVolumeID *int64
	for i, ch := range chapters {
		if i == 0 || !sameVolumeID(previousVolumeID, ch.VolumeID) {
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			if ch.VolumeID == nil {
				sb.WriteString("## Unassigned Volume\n")
			} else {
				name, ok := volumeNames[*ch.VolumeID]
				if !ok {
					name = "Unknown Volume"
				}
				fmt.Fprintf(&sb, "## %s [volume_id:%d]\n", name, *ch.VolumeID)
			}
			previousVolumeID = ch.VolumeID
		}
		fmt.Fprintf(&sb, "- Chapter %d \"%s\" [chapter_id:%d] · %d words\n", ch.ReadingNumber, ch.Title, ch.ID, ch.WordCount)
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

func sameVolumeID(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// ── 注册 ──────────────────────────────────────────────

// RegisterNovelTools 注册小说管理类工具。
func RegisterNovelTools(r *Registry) {
	r.Register(&GetChapterListTool{})
}
