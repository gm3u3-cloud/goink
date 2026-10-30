package mcp_tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/sigpanic/goink/internal/chapter"
	"github.com/sigpanic/goink/internal/storage"
	"github.com/sigpanic/goink/internal/timeline"
)

// ── get_timeline ────────────────────────────────────────

// GetTimelineArgs 是 get_timeline 的参数。
type GetTimelineArgs struct {
	CurrentChapterID int64  `json:"current_chapter_id" jsonschema:"description=Stable ID of the current chapter. When provided, nearby entries are automatically collected by current reading order and anomalies are detected. Required when writing a new chapter." validate:"omitempty,min=1"`
	Category         string `json:"category" jsonschema:"description=Filter by category,enum=foreshadowing,enum=user_directive" validate:"omitempty,oneof=foreshadowing user_directive"`
	Status           string `json:"status" jsonschema:"description=Filter by status,enum=pending,enum=resolved,enum=abandoned" validate:"omitempty,oneof=pending resolved abandoned"`
	Search           string `json:"search" jsonschema:"description=Fuzzy search by title or content (only applies when current_chapter is not provided)"`
	PageArgs                // 嵌入分页参数（仅不传 current_chapter 时生效）
}

// GetTimelineTool 获取章节计划 + 伏笔/用户指令总览。
type GetTimelineTool struct{}

func (t *GetTimelineTool) Name() string { return "get_timeline" }
func (t *GetTimelineTool) Description() string {
	return "Get a story timeline overview: chapter plans (next/near/far) + foreshadowing and user directives. Two usage modes:\n" +
		"- With current_chapter_id: nearby entries are collected automatically (recent history + future + anomaly flags). Do NOT pass pagination/filter params\n" +
		"- Without current_chapter_id: paginate over entries (excluding plans), filterable by category/status. Pagination/filter params are required"
}
func (t *GetTimelineTool) Category() ToolCategory { return CategoryMemoryRetrieval }

func (t *GetTimelineTool) JSONSchema() json.RawMessage { return SchemaOf(GetTimelineArgs{}) }
func (t *GetTimelineTool) ExposeToLLM() bool           { return true }
func (t *GetTimelineTool) NewArgs() any                { return &GetTimelineArgs{} }

func (t *GetTimelineTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*GetTimelineArgs)
	a.NormalizePage()

	store := timeline.NewStore(tc.DB, tc.LoggerOrDefault())

	if a.CurrentChapterID > 0 {
		currentReadingNumber, err := chapter.NewStore(tc.DB, tc.LoggerOrDefault()).GetReadingNumberByID(ctx, nil, tc.NovelID, a.CurrentChapterID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return &ToolResult{Success: false, Error: fmt.Sprintf("Глава %d не существует или не принадлежит текущему роману", a.CurrentChapterID)}, nil
			}
			return nil, err
		}
		plans, err := store.GetPlans(ctx, tc.NovelID)
		if err != nil {
			return nil, fmt.Errorf("query plans: %w", err)
		}
		return t.executeContext(ctx, a, tc, store, plans, currentReadingNumber)
	}
	return t.executeFull(ctx, a, tc, store)
}

func (t *GetTimelineTool) executeContext(ctx context.Context, a *GetTimelineArgs, tc ToolContext, store *timeline.Store, plans []timeline.ChapterPlan, currentReadingNumber int) (*ToolResult, error) {
	// 近期历史（target_chapter < current，最近 10 条）
	history, err := store.ListBefore(ctx, tc.NovelID, currentReadingNumber, 10)
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}

	// 异常：前面的章还有 pending（该回收没回收）
	pendingBefore, err := store.ListPendingBefore(ctx, tc.NovelID, currentReadingNumber)
	if err != nil {
		return nil, fmt.Errorf("query pending before: %w", err)
	}
	anomalies := pendingBefore // target_chapter < current && status=pending

	// 未来条目
	future, err := store.ListAfter(ctx, tc.NovelID, currentReadingNumber)
	if err != nil {
		return nil, fmt.Errorf("query future: %w", err)
	}

	// 异常补充：未来条目中已 resolved 的（不应提前回收）
	for _, e := range future {
		if e.Status == "resolved" {
			anomalies = append(anomalies, e)
		}
	}

	formatted := formatTimelineContext(plans, history, anomalies, future, currentReadingNumber)

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"content": formatted},
	}, nil
}

func (t *GetTimelineTool) executeFull(ctx context.Context, a *GetTimelineArgs, tc ToolContext, store *timeline.Store) (*ToolResult, error) {
	result, err := store.ListByNovel(ctx, tc.NovelID, timeline.ListByNovelOptions{
		PageParams: storage.PageParams{Page: a.Page, Size: a.Size},
		Search:     a.Search,
		Category:   a.Category,
		Status:     a.Status,
		Order:      "target_reading_number ASC, importance DESC",
	})
	if err != nil {
		return nil, fmt.Errorf("list timeline: %w", err)
	}

	formatted := formatTimelineFull(result.Items)

	data := PageMeta(result)
	data["content"] = formatted

	return &ToolResult{Success: true, Data: data}, nil
}

// ── create_timeline_entry ───────────────────────────────

// CreateTimelineEntryItem 是单条伏笔/用户指令的创建参数。
type CreateTimelineEntryItem struct {
	Category            string `json:"category" jsonschema:"required,description=Entry type,enum=foreshadowing,enum=user_directive" validate:"required,oneof=foreshadowing user_directive"`
	Title               string `json:"title" jsonschema:"required,description=Short title"                                   validate:"required"`
	Content             string `json:"content" jsonschema:"description=Detailed description"`
	DetailJSON          string `json:"detail_json" jsonschema:"description=JSON in string form, structured data"`
	TargetReadingNumber int    `json:"target_reading_number" jsonschema:"required,description=Expected reading number at which this is resolved. Used for the future plan position, not a chapter ID" validate:"required,min=1"`
	Importance          int    `json:"importance" jsonschema:"description=Importance 1-5,default=3,minimum=1,maximum=5"         validate:"omitempty,min=1,max=5"`
	SourceChapterID     *int64 `json:"source_chapter_id" jsonschema:"description=Stable chapter ID where this was created/planted" validate:"omitempty,min=1"`
	Source              string `json:"source" jsonschema:"description=Source,default=ai"`
}

// CreateTimelineEntryArgs 是 create_timeline_entry 的参数。
type CreateTimelineEntryArgs struct {
	Entries []CreateTimelineEntryItem `json:"entries" jsonschema:"required,description=List of entries to add (1-6)" validate:"required,min=1,max=6,dive"`
}

// CreateTimelineEntryTool 批量创建伏笔和用户指令。
type CreateTimelineEntryTool struct{}

func (t *CreateTimelineEntryTool) Name() string { return "create_timeline_entry" }
func (t *CreateTimelineEntryTool) Description() string {
	return "Batch-create foreshadowing entries or user directives (1-6). Guarantees atomicity; on failure returns the reason for the specific item." +
		"Call this after finishing a chapter when new foreshadowing or user directives are planted." +
		"Query existing entries before creating; if the content is similar, update the existing entry to avoid duplicates." +
		"category is foreshadowing (foreshadowing) or user_directive (user creative directive)."
}
func (t *CreateTimelineEntryTool) Category() ToolCategory { return CategoryWritingAssistant }

func (t *CreateTimelineEntryTool) JSONSchema() json.RawMessage {
	return SchemaOf(CreateTimelineEntryArgs{})
}
func (t *CreateTimelineEntryTool) ExposeToLLM() bool { return true }
func (t *CreateTimelineEntryTool) NewArgs() any      { return &CreateTimelineEntryArgs{} }

func (t *CreateTimelineEntryTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*CreateTimelineEntryArgs)
	chapterIDs := make([]int64, 0, len(a.Entries))
	for _, item := range a.Entries {
		if item.SourceChapterID != nil {
			chapterIDs = append(chapterIDs, *item.SourceChapterID)
		}
	}
	if result, err := ensureChapterIDsInNovel(ctx, tc, chapterIDs); err != nil || result != nil {
		return result, err
	}

	var ids []int64
	var failedName string
	var failedErr error
	err := tc.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range a.Entries {
			source := item.Source
			if source == "" {
				source = "ai"
			}
			importance := item.Importance
			if importance == 0 {
				importance = 3
			}
			entry := timeline.TimelineEntry{
				NovelID:             tc.NovelID,
				Category:            item.Category,
				Title:               item.Title,
				Content:             item.Content,
				DetailJSON:          item.DetailJSON,
				TargetReadingNumber: item.TargetReadingNumber,
				Importance:          importance,
				SourceChapterID:     item.SourceChapterID,
				Source:              source,
				Status:              "pending",
			}
			if err := tx.Create(&entry).Error; err != nil {
				failedName = item.Title
				failedErr = err
				return err
			}
			ids = append(ids, entry.ID)
		}
		return nil
	})
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Не удалось создать запись таймлайна [%s]: %s", failedName, failedErr),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"ids": ids, "count": len(ids)},
	}, nil
}

// ── update_timeline_entry ───────────────────────────────

// UpdateTimelineEntryArgs 是 update_timeline_entry 的参数。
type UpdateTimelineEntryArgs struct {
	EntryID             int64  `json:"entry_id" jsonschema:"required,description=Entry ID"              validate:"required,min=1"`
	Title               string `json:"title" jsonschema:"description=New title"`
	Content             string `json:"content" jsonschema:"description=New description"`
	DetailJSON          string `json:"detail_json" jsonschema:"description=New structured data (completely replaces the old one)"`
	TargetReadingNumber int    `json:"target_reading_number" jsonschema:"description=New target reading number,minimum=1" validate:"omitempty,min=1"`
	Importance          int    `json:"importance" jsonschema:"description=New importance 1-5,minimum=1,maximum=5"`
	Status              string `json:"status" jsonschema:"description=New status,enum=pending,enum=resolved,enum=abandoned" validate:"omitempty,oneof=pending resolved abandoned"`
	ResolvedChapterID   *int64 `json:"resolved_chapter_id" jsonschema:"description=Stable chapter ID where this is resolved (fill in when marking resolved)" validate:"omitempty,min=1"`
}

// UpdateTimelineEntryTool 更新伏笔或用户指令。
type UpdateTimelineEntryTool struct{}

func (t *UpdateTimelineEntryTool) Name() string { return "update_timeline_entry" }
func (t *UpdateTimelineEntryTool) Description() string {
	return "Update an existing foreshadowing entry or user directive. Only pass the fields to change." +
		"Common uses: resolve a foreshadowing (status=resolved + resolved_chapter_id), adjust target_reading_number, edit content." +
		"category and source_chapter_id are immutable after creation."
}
func (t *UpdateTimelineEntryTool) Category() ToolCategory { return CategoryWritingAssistant }

func (t *UpdateTimelineEntryTool) JSONSchema() json.RawMessage {
	return SchemaOf(UpdateTimelineEntryArgs{})
}
func (t *UpdateTimelineEntryTool) ExposeToLLM() bool { return true }
func (t *UpdateTimelineEntryTool) NewArgs() any      { return &UpdateTimelineEntryArgs{} }

func (t *UpdateTimelineEntryTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*UpdateTimelineEntryArgs)

	if a.Title == "" && a.Content == "" && a.DetailJSON == "" && a.TargetReadingNumber == 0 && a.Importance == 0 && a.Status == "" && a.ResolvedChapterID == nil {
		return &ToolResult{Success: false, Error: "Необходимо указать хотя бы одно поле для изменения"}, nil
	}

	var entry timeline.TimelineEntry
	if err := tc.DB.WithContext(ctx).
		Where("id = ? AND novel_id = ?", a.EntryID, tc.NovelID).
		First(&entry).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("Запись %d не существует", a.EntryID)}, nil
		}
		return nil, fmt.Errorf("query timeline entry: %w", err)
	}
	chapterIDs := make([]int64, 0, 1)
	if a.ResolvedChapterID != nil {
		chapterIDs = append(chapterIDs, *a.ResolvedChapterID)
	}
	if result, err := ensureChapterIDsInNovel(ctx, tc, chapterIDs); err != nil || result != nil {
		return result, err
	}

	if err := json.Unmarshal(tc.RawArgs, &entry); err != nil {
		return &ToolResult{Success: false, Error: "Неверный формат параметров: " + err.Error()}, nil
	}

	if err := tc.DB.WithContext(ctx).Save(&entry).Error; err != nil {
		return nil, fmt.Errorf("save timeline entry: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"id": entry.ID},
	}, nil
}

// ── update_chapter_plan ─────────────────────────────────

// UpdateChapterPlanArgs 是 update_chapter_plan 的参数。
type UpdateChapterPlanArgs struct {
	Scope   string `json:"scope" jsonschema:"required,description=Plan scope,enum=next,enum=near,enum=far" validate:"required,oneof=next near far"`
	Content string `json:"content" jsonschema:"required,description=Plan content, a natural-language description"                   validate:"required"`
}

// UpdateChapterPlanTool 更新章节创作计划。
type UpdateChapterPlanTool struct{}

func (t *UpdateChapterPlanTool) Name() string { return "update_chapter_plan" }
func (t *UpdateChapterPlanTool) Description() string {
	return "Update the chapter writing plan. Three slots:\n" +
		"- next: the writing plan for the next chapter\n" +
		"- near: the writing direction for the upcoming 3-10 chapters\n" +
		"- far: the long-term writing direction\n" +
		"Calling the same scope again overwrites the previous value. Update the plan before writing a new chapter to reflect the latest progress."
}
func (t *UpdateChapterPlanTool) Category() ToolCategory { return CategoryWritingAssistant }

func (t *UpdateChapterPlanTool) JSONSchema() json.RawMessage {
	return SchemaOf(UpdateChapterPlanArgs{})
}
func (t *UpdateChapterPlanTool) ExposeToLLM() bool { return true }
func (t *UpdateChapterPlanTool) NewArgs() any      { return &UpdateChapterPlanArgs{} }

func (t *UpdateChapterPlanTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*UpdateChapterPlanArgs)

	plan := timeline.ChapterPlan{
		NovelID: tc.NovelID,
		Scope:   a.Scope,
		Content: a.Content,
	}

	if err := timeline.NewStore(tc.DB, tc.LoggerOrDefault()).SavePlan(ctx, &plan); err != nil {
		return nil, fmt.Errorf("save plan: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"scope": plan.Scope,
		},
	}, nil
}

// ── 格式化 ──────────────────────────────────────────────

func formatTimelineContext(plans []timeline.ChapterPlan, history, anomalies, future []timeline.TimelineEntry, currentChapter int) string {
	var parts []string

	// 章节计划
	parts = append(parts, "### Chapter Plans")
	planMap := map[string]string{"next": "None", "near": "None", "far": "None"}
	for _, p := range plans {
		if p.Content != "" {
			planMap[p.Scope] = p.Content
		}
	}
	parts = append(parts, fmt.Sprintf("- **next**: %s", planMap["next"]))
	parts = append(parts, fmt.Sprintf("- **near**: %s", planMap["near"]))
	parts = append(parts, fmt.Sprintf("- **far**: %s", planMap["far"]))

	// 近期历史（排除已在异常中的条目，避免重复）
	anomalyIDs := make(map[int64]bool, len(anomalies))
	for _, e := range anomalies {
		anomalyIDs[e.ID] = true
	}
	if len(history) > 0 {
		lines := []string{fmt.Sprintf("\n### Recent History (last %d entries, up to chapter %d)", len(history), currentChapter)}
		for _, e := range history {
			if anomalyIDs[e.ID] {
				continue
			}
			cat := catLabel(e.Category)
			st := statusLabel(e.Status)
			line := fmt.Sprintf("- %s %s [entry_id:%d] — target chapter %d — %s", cat, e.Title, e.ID, e.TargetReadingNumber, st)
			lines = append(lines, line)
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}

	// 异常
	if len(anomalies) > 0 {
		lines := []string{"\n### ⚠️ Status Anomalies"}
		lines = append(lines, "The following entries need attention and correction:")
		for _, e := range anomalies {
			cat := catLabel(e.Category)
			if e.Status == "pending" && e.TargetReadingNumber < currentChapter {
				lines = append(lines, fmt.Sprintf("- %s %s [entry_id:%d] — target chapter %d but still pending; should be resolved before chapter %d",
					cat, e.Title, e.ID, e.TargetReadingNumber, currentChapter))
			} else if e.Status == "resolved" && e.TargetReadingNumber >= currentChapter {
				lines = append(lines, fmt.Sprintf("- %s %s [entry_id:%d] — target chapter %d but already marked resolved; may have been resolved early",
					cat, e.Title, e.ID, e.TargetReadingNumber))
			}
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}

	// 未来条目
	if len(future) > 0 {
		lines := []string{fmt.Sprintf("\n### Future Entries (%d entries, sorted by target chapter)", len(future))}
		for _, e := range future {
			if e.Status == "resolved" {
				continue // 已在异常区展示
			}
			line := fmt.Sprintf("- %s %s [entry_id:%d] chapter %d", catLabel(e.Category), e.Title, e.ID, e.TargetReadingNumber)
			if e.Importance > 0 {
				line += fmt.Sprintf(" [importance:%d]", e.Importance)
			}
			if e.Status == "abandoned" {
				line += " — abandoned"
			}
			lines = append(lines, line)
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}

	if len(history) == 0 && len(anomalies) == 0 && len(future) == 0 {
		parts = append(parts, "\nNo foreshadowing or user directives yet.")
	}

	return strings.Join(parts, "\n")
}

func formatTimelineFull(entries []timeline.TimelineEntry) string {
	var parts []string

	if len(entries) > 0 {
		lines := []string{fmt.Sprintf("### Foreshadowing & User Directives (%d entries)", len(entries))}
		for _, e := range entries {
			line := fmt.Sprintf("- %s %s [entry_id:%d] chapter %d — %s",
				catLabel(e.Category), e.Title, e.ID, e.TargetReadingNumber, statusLabel(e.Status))
			if e.Importance > 0 {
				line += fmt.Sprintf(" [importance:%d]", e.Importance)
			}
			lines = append(lines, line)
		}
		parts = append(parts, strings.Join(lines, "\n"))
	} else {
		parts = append(parts, "No foreshadowing or user directives yet.")
	}

	return strings.Join(parts, "\n")
}

func catLabel(category string) string {
	switch category {
	case "foreshadowing":
		return "[foreshadowing]"
	case "user_directive":
		return "[user_directive]"
	}
	return "[" + category + "]"
}

func statusLabel(status string) string {
	switch status {
	case "pending":
		return "pending"
	case "resolved":
		return "resolved ✓"
	case "abandoned":
		return "abandoned ✗"
	}
	return status
}

// ── 注册 ──────────────────────────────────────────────

// RegisterTimelineTools 注册时间线管理类工具。
func RegisterTimelineTools(r *Registry) {
	r.Register(&GetTimelineTool{})
	r.Register(&CreateTimelineEntryTool{})
	r.Register(&UpdateTimelineEntryTool{})
	r.Register(&UpdateChapterPlanTool{})
}
