package mcp_tools

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"gorm.io/gorm"

	"github.com/sigpanic/goink/internal/chapter"
	"github.com/sigpanic/goink/internal/reader"
)

// ── get_reader_perspective ──────────────────────────────

// GetReaderPerspectiveArgs 无参——LLM 按需调用即可。
type GetReaderPerspectiveArgs struct{}

// GetReaderPerspectiveTool 返回读者当前认知状态的三段式摘要。
// known 兜底截断 60 条——完整认知上下文只需最近的关键事实。
type GetReaderPerspectiveTool struct{}

func (t *GetReaderPerspectiveTool) Name() string { return "get_reader_perspective" }
func (t *GetReaderPerspectiveTool) Description() string {
	return "Get the reader's current cognition state for this novel: known facts, active suspenses, and reader misconceptions. " +
		"The `[entry_id:X]` at the end of each entry is that entry's unique identifier; fill in entry_id when updating or resolving it. " +
		"Prefer merging similar information into existing entries to avoid creating duplicates. Only record entries the reader will definitely care about and that future writing must consider."
}
func (t *GetReaderPerspectiveTool) Category() ToolCategory { return CategoryMemoryRetrieval }

func (t *GetReaderPerspectiveTool) JSONSchema() json.RawMessage {
	return SchemaOf(GetReaderPerspectiveArgs{})
}

func (t *GetReaderPerspectiveTool) ExposeToLLM() bool { return true }
func (t *GetReaderPerspectiveTool) NewArgs() any      { return &GetReaderPerspectiveArgs{} }

func (t *GetReaderPerspectiveTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	rs := reader.NewStore(tc.DB, tc.LoggerOrDefault())

	// known：读取后按当前阅读顺序取最近 60 条。
	var knownItems []reader.ReaderPerspective
	if err := tc.DB.WithContext(ctx).
		Where("novel_id = ? AND type = ?", tc.NovelID, reader.TypeKnown).
		Find(&knownItems).Error; err != nil {
		return nil, fmt.Errorf("запрос известных фактов: %w", err)
	}

	// suspense + misconception：只取未回收的
	active, err := rs.ListActive(ctx, tc.NovelID)
	if err != nil {
		return nil, fmt.Errorf("запрос активных восприятий: %w", err)
	}

	var suspenses []reader.ReaderPerspective
	var misconceptions []reader.ReaderPerspective
	for _, e := range active {
		switch e.Type {
		case reader.TypeSuspense:
			suspenses = append(suspenses, e)
		case reader.TypeMisconception:
			misconceptions = append(misconceptions, e)
		}
	}

	readingNumbers, err := chapter.NewStore(tc.DB, tc.LoggerOrDefault()).GetReadingNumbersByNovel(ctx, nil, tc.NovelID)
	if err != nil {
		return nil, err
	}
	readingNumberFor := func(chapterID *int64) int {
		if chapterID == nil {
			return 0
		}
		return readingNumbers[*chapterID]
	}
	byLatestPlanting := func(a, b reader.ReaderPerspective) int {
		return readingNumberFor(b.PlantedChapterID) - readingNumberFor(a.PlantedChapterID)
	}
	slices.SortFunc(knownItems, byLatestPlanting)
	if len(knownItems) > 60 {
		knownItems = knownItems[:60]
	}
	slices.SortFunc(suspenses, byLatestPlanting)
	slices.SortFunc(misconceptions, byLatestPlanting)

	formatted := formatReaderPerspective(knownItems, suspenses, misconceptions, readingNumbers)

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"content": formatted,
			"counts": map[string]int{
				"known":         len(knownItems),
				"suspense":      len(suspenses),
				"misconception": len(misconceptions),
			},
		},
	}, nil
}

// ── 格式化 ──────────────────────────────────────────────

func formatReaderPerspective(known, suspenses, misconceptions []reader.ReaderPerspective, readingNumbers map[int64]int) string {
	var sections []string

	ref := func(e reader.ReaderPerspective) string {
		return fmt.Sprintf(" `[entry_id:%d]`", e.ID)
	}
	chapterLabel := func(chapterID *int64) string {
		if chapterID == nil {
			return "chapter info missing"
		}
		if readingNumber, ok := readingNumbers[*chapterID]; ok {
			return fmt.Sprintf("Chapter %d [chapter_id:%d]", readingNumber, *chapterID)
		}
		return fmt.Sprintf("Chapter [chapter_id:%d]", *chapterID)
	}

	// 已知信息
	if len(known) > 0 {
		lines := []string{"### Known Facts"}
		for _, e := range known {
			lines = append(lines, fmt.Sprintf("- %s [from %s]%s", e.Content, chapterLabel(e.PlantedChapterID), ref(e)))
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	// 活跃悬念
	if len(suspenses) > 0 {
		lines := []string{"### Active Suspenses"}
		for _, e := range suspenses {
			lines = append(lines, fmt.Sprintf("- %s (planted in %s)%s", e.Content, chapterLabel(e.PlantedChapterID), ref(e)))
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	// 读者误知
	if len(misconceptions) > 0 {
		lines := []string{"### Reader Misconceptions"}
		for _, e := range misconceptions {
			truth := ""
			if e.RelatedTruth != "" {
				truth = fmt.Sprintf(" → Actual: %s", e.RelatedTruth)
			}
			lines = append(lines, fmt.Sprintf("- %s%s%s", e.Content, truth, ref(e)))
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	if len(sections) == 0 {
		return "No reader cognition data available."
	}
	return strings.Join(sections, "\n\n")
}

// ── create_reader_perspective_entry ──────────────────────

// CreateReaderPerspectiveEntryItem 是 create_reader_perspective_entry 的单条参数。
type CreateReaderPerspectiveEntryItem struct {
	Type             string `json:"type" jsonschema:"required,description=entry type,enum=known,enum=suspense,enum=misconception" validate:"required,oneof=known suspense misconception"`
	Content          string `json:"content" jsonschema:"required,description=content description"          validate:"required"`
	PlantedChapterID int64  `json:"planted_chapter_id" jsonschema:"required,description=stable chapter ID where this info was planted" validate:"required,min=1"`
	RelatedTruth     string `json:"related_truth" jsonschema:"description=misconception only: what the truth actually is"`
}

// CreateReaderPerspectiveEntryArgs 是 create_reader_perspective_entry 的参数。
type CreateReaderPerspectiveEntryArgs struct {
	Entries []CreateReaderPerspectiveEntryItem `json:"entries" jsonschema:"required,description=reader cognition entries to create (1-10)" validate:"required,min=1,max=10,dive"`
}

// CreateReaderPerspectiveEntryTool 创建一条读者认知条目。
type CreateReaderPerspectiveEntryTool struct{}

func (t *CreateReaderPerspectiveEntryTool) Name() string { return "create_reader_perspective_entry" }
func (t *CreateReaderPerspectiveEntryTool) Description() string {
	return "Bulk-add reader cognition entries (1-10). Atomic: on failure, returns the specific failing entry. Three types:\n" +
		"- known: what the reader learned after a certain chapter\n" +
		"- suspense: a suspense the reader is currently waiting to be resolved\n" +
		"- misconception: what the reader believes to be true (for future plot twists)\n" +
		"Proactively add entries after each chapter if new information is revealed or new suspenses are planted."
}
func (t *CreateReaderPerspectiveEntryTool) Category() ToolCategory { return CategoryWritingAssistant }

func (t *CreateReaderPerspectiveEntryTool) JSONSchema() json.RawMessage {
	return SchemaOf(CreateReaderPerspectiveEntryArgs{})
}

func (t *CreateReaderPerspectiveEntryTool) ExposeToLLM() bool { return true }
func (t *CreateReaderPerspectiveEntryTool) NewArgs() any      { return &CreateReaderPerspectiveEntryArgs{} }

func (t *CreateReaderPerspectiveEntryTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*CreateReaderPerspectiveEntryArgs)

	// 预校验：misconception 必须提供 related_truth
	for _, item := range a.Entries {
		if item.Type == reader.TypeMisconception && item.RelatedTruth == "" {
			return &ToolResult{Success: false, Error: "для типа misconception обязательно укажите related_truth (фактическая правда)"}, nil
		}
	}
	chapterIDs := make([]int64, 0, len(a.Entries))
	for _, item := range a.Entries {
		chapterIDs = append(chapterIDs, item.PlantedChapterID)
	}
	if result, err := ensureChapterIDsInNovel(ctx, tc, chapterIDs); err != nil || result != nil {
		return result, err
	}

	var ids []int64
	var failedName string
	var failedErr error
	err := tc.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range a.Entries {
			chapterID := item.PlantedChapterID
			rp := reader.ReaderPerspective{
				NovelID:          tc.NovelID,
				Type:             item.Type,
				Content:          item.Content,
				PlantedChapterID: &chapterID,
				RelatedTruth:     item.RelatedTruth,
			}
			if err := tx.Create(&rp).Error; err != nil {
				failedName = item.Content
				if len(failedName) > 20 {
					failedName = failedName[:20]
				}
				failedErr = err
				return err
			}
			ids = append(ids, rp.ID)
		}
		return nil
	})
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("не удалось создать запись о восприятии читателя [%s]: %s", failedName, failedErr),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"ids": ids, "count": len(ids)},
	}, nil
}

// ── update_reader_perspective_entry ──────────────────────

// UpdateReaderPerspectiveEntryArgs 是 update_reader_perspective_entry 的参数。
type UpdateReaderPerspectiveEntryArgs struct {
	EntryID           int    `json:"entry_id" jsonschema:"required,description=entry ID to update" validate:"required,min=1"`
	Content           string `json:"content" jsonschema:"description=full content description after update"`
	RevealedChapterID *int64 `json:"revealed_chapter_id" jsonschema:"description=stable chapter ID where it was revealed or resolved (once set, the entry no longer appears in the active list)" validate:"omitempty,min=1"`
	PlantedChapterID  *int64 `json:"planted_chapter_id" jsonschema:"description=stable chapter ID where it was planted" validate:"omitempty,min=1"`
	RelatedTruth      string `json:"related_truth" jsonschema:"description=the truth from the author's perspective (all types supported)"`
	Type              string `json:"type" jsonschema:"description=entry type,enum=known,enum=suspense,enum=misconception" validate:"omitempty,oneof=known suspense misconception"`
}

// UpdateReaderPerspectiveEntryTool 更新读者认知条目。
type UpdateReaderPerspectiveEntryTool struct{}

func (t *UpdateReaderPerspectiveEntryTool) Name() string { return "update_reader_perspective_entry" }
func (t *UpdateReaderPerspectiveEntryTool) Description() string {
	return "Update a single reader cognition entry. Common uses:\n" +
		"- Resolve a suspense: set revealed_chapter_id\n" +
		"- Reveal a misconception: set revealed_chapter_id"
}
func (t *UpdateReaderPerspectiveEntryTool) Category() ToolCategory { return CategoryWritingAssistant }

func (t *UpdateReaderPerspectiveEntryTool) JSONSchema() json.RawMessage {
	return SchemaOf(UpdateReaderPerspectiveEntryArgs{})
}

func (t *UpdateReaderPerspectiveEntryTool) ExposeToLLM() bool { return true }
func (t *UpdateReaderPerspectiveEntryTool) NewArgs() any      { return &UpdateReaderPerspectiveEntryArgs{} }

func (t *UpdateReaderPerspectiveEntryTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*UpdateReaderPerspectiveEntryArgs)

	if a.RevealedChapterID == nil && a.PlantedChapterID == nil && a.Content == "" && a.Type == "" && a.RelatedTruth == "" {
		return &ToolResult{Success: false, Error: "необходимо указать как минимум одно поле для изменения"}, nil
	}

	var entry reader.ReaderPerspective
	if err := tc.DB.WithContext(ctx).
		Where("id = ? AND novel_id = ?", a.EntryID, tc.NovelID).
		First(&entry).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("запись %d не существует", a.EntryID)}, nil
		}
		return nil, fmt.Errorf("запрос записи о восприятии: %w", err)
	}

	if err := json.Unmarshal(tc.RawArgs, &entry); err != nil {
		return &ToolResult{Success: false, Error: "некорректный формат параметров: " + err.Error()}, nil
	}
	chapterIDs := make([]int64, 0, 2)
	for _, chapterID := range []*int64{a.PlantedChapterID, a.RevealedChapterID} {
		if chapterID != nil {
			chapterIDs = append(chapterIDs, *chapterID)
		}
	}
	if result, err := ensureChapterIDsInNovel(ctx, tc, chapterIDs); err != nil || result != nil {
		return result, err
	}

	if err := tc.DB.WithContext(ctx).Save(&entry).Error; err != nil {
		return nil, fmt.Errorf("сохранение записи о восприятии: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"id": entry.ID, "revealed_chapter_id": entry.RevealedChapterID},
	}, nil
}

// ── 注册 ──────────────────────────────────────────────

// RegisterReaderPerspectiveTools 注册读者认知类工具。
func RegisterReaderPerspectiveTools(r *Registry) {
	r.Register(&GetReaderPerspectiveTool{})
	r.Register(&CreateReaderPerspectiveEntryTool{})
	r.Register(&UpdateReaderPerspectiveEntryTool{})
}
