package mcp_tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/sigpanic/goink/internal/chapter"
	"github.com/sigpanic/goink/internal/storage"
	"github.com/sigpanic/goink/internal/storyarc"
)

// ── get_story_arcs ─────────────────────────────────────

// GetStoryArcsArgs 是 get_story_arcs 的参数。
type GetStoryArcsArgs struct {
	CurrentChapterID int64  `json:"current_chapter_id" jsonschema:"description=Stable ID of the current chapter. When provided, the arcs are windowed by current reading order and anomalies are detected. Required when writing a new chapter." validate:"omitempty,min=1"`
	ArcType          string `json:"arc_type" jsonschema:"description=Filter by type,enum=main,enum=sub,enum=character,enum=background" validate:"omitempty,oneof=main sub character background"`
	Status           string `json:"status" jsonschema:"description=Filter by status,enum=active,enum=paused,enum=completed,enum=abandoned" validate:"omitempty,oneof=active paused completed abandoned"`
	PageArgs                // 嵌入分页参数（仅不传 current_chapter 时生效）
}

// GetStoryArcsTool 获取叙事弧线及节点链。
type GetStoryArcsTool struct{}

func (t *GetStoryArcsTool) Name() string { return "get_story_arcs" }
func (t *GetStoryArcsTool) Description() string {
	return "Get story arcs and their node chains. Two usage modes:\n" +
		"- With current_chapter_id: active arcs are windowed (recent/anomaly/future), paused arcs show breakpoint + reactivation condition, completed/abandoned arcs show metadata only. Do NOT pass pagination/filter params\n" +
		"- Without current_chapter_id: paginate over all arcs with their full node chains. Pagination/filter params are required"
}
func (t *GetStoryArcsTool) Category() ToolCategory { return CategoryMemoryRetrieval }

func (t *GetStoryArcsTool) JSONSchema() json.RawMessage { return SchemaOf(GetStoryArcsArgs{}) }
func (t *GetStoryArcsTool) ExposeToLLM() bool           { return true }
func (t *GetStoryArcsTool) NewArgs() any                { return &GetStoryArcsArgs{} }

func (t *GetStoryArcsTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*GetStoryArcsArgs)
	a.NormalizePage()

	store := storyarc.NewStore(tc.DB, tc.LoggerOrDefault())
	readingNumbers, err := chapter.NewStore(tc.DB, tc.LoggerOrDefault()).GetReadingNumbersByNovel(ctx, nil, tc.NovelID)
	if err != nil {
		return nil, err
	}

	if a.CurrentChapterID > 0 {
		currentReadingNumber, ok := readingNumbers[a.CurrentChapterID]
		if !ok {
			return &ToolResult{Success: false, Error: fmt.Sprintf("Глава %d не существует или не принадлежит текущему роману", a.CurrentChapterID)}, nil
		}
		return t.executeContext(ctx, tc, store, currentReadingNumber, readingNumbers)
	}
	return t.executeFull(ctx, a, tc, store, readingNumbers)
}

func (t *GetStoryArcsTool) executeContext(ctx context.Context, tc ToolContext, store *storyarc.Store, currentReadingNumber int, readingNumbers map[int64]int) (*ToolResult, error) {
	arcs, err := store.ListNonArchived(ctx, tc.NovelID)
	if err != nil {
		return nil, fmt.Errorf("query arcs: %w", err)
	}

	var activeArcs []storyarc.StoryArc
	var pausedArcs []storyarc.StoryArc
	var activeIDs []int64

	for _, arc := range arcs {
		switch arc.Status {
		case "active":
			activeArcs = append(activeArcs, arc)
			activeIDs = append(activeIDs, arc.ID)
		case "paused":
			pausedArcs = append(pausedArcs, arc)
		}
	}

	// 每条活跃弧线独占窗口
	beforeByArc, err := store.ListNodesBeforeByArc(ctx, activeIDs, currentReadingNumber, 10)
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}
	anomalyByArc, err := store.ListPendingNodesBeforeByArc(ctx, activeIDs, currentReadingNumber)
	if err != nil {
		return nil, fmt.Errorf("query anomalies: %w", err)
	}
	afterByArc, err := store.ListNodesAfterByArc(ctx, activeIDs, currentReadingNumber)
	if err != nil {
		return nil, fmt.Errorf("query future: %w", err)
	}

	// 暂停弧线：断点
	pausedBefore := make(map[int64][]storyarc.ArcNode)
	pausedPending := make(map[int64][]storyarc.ArcNode)
	for _, arc := range pausedArcs {
		before, pending, err := store.GetBreakpoint(ctx, arc.ID)
		if err != nil {
			return nil, fmt.Errorf("query breakpoint arc %d: %w", arc.ID, err)
		}
		if before != nil || pending != nil {
			pausedBefore[arc.ID] = before
			pausedPending[arc.ID] = pending
		}
	}

	// 已完成/废弃的弧线（仅元数据）
	var archivedArcs []storyarc.StoryArc
	tc.DB.WithContext(ctx).
		Where("novel_id = ? AND status IN ?", tc.NovelID, []string{"completed", "abandoned"}).
		Order("importance DESC").
		Find(&archivedArcs)

	formatted := formatArcsAll(activeArcs, beforeByArc, anomalyByArc, afterByArc,
		pausedArcs, pausedBefore, pausedPending,
		archivedArcs, currentReadingNumber, readingNumbers)

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"content": formatted},
	}, nil
}

func (t *GetStoryArcsTool) executeFull(ctx context.Context, a *GetStoryArcsArgs, tc ToolContext, store *storyarc.Store, readingNumbers map[int64]int) (*ToolResult, error) {
	result, err := store.ListByNovel(ctx, tc.NovelID, storyarc.ListByNovelOptions{
		PageParams: storage.PageParams{Page: a.Page, Size: a.Size},
		ArcType:    a.ArcType,
		Status:     a.Status,
		Order:      "importance DESC, created_at ASC", // 按重要性排序，与 executeContext 一致
	})
	if err != nil {
		return nil, fmt.Errorf("list arcs: %w", err)
	}

	var arcIDs []int64
	for _, arc := range result.Items {
		arcIDs = append(arcIDs, arc.ID)
	}
	nodes, _ := store.ListByArcs(ctx, arcIDs)

	formatted := formatArcsFull(result.Items, nodes, readingNumbers)

	data := PageMeta(result)
	data["content"] = formatted

	return &ToolResult{Success: true, Data: data}, nil
}

// ── create_story_arc ───────────────────────────────────

// CreateStoryArcItem 是 create_story_arc 的单条参数。
type CreateStoryArcItem struct {
	Name        string `json:"name" jsonschema:"required,description=Arc name, e.g. 'Path of Revenge'"                          validate:"required"`
	ArcType     string `json:"arc_type" jsonschema:"required,description=Arc type,enum=main,enum=sub,enum=character,enum=background" validate:"required,oneof=main sub character background"`
	Description string `json:"description" jsonschema:"description=Overall description of the arc"`
	Importance  int    `json:"importance" jsonschema:"description=Importance 1-5,default=1,minimum=1,maximum=5"          validate:"omitempty,min=1,max=5"`
}

// CreateStoryArcArgs 是 create_story_arc 的参数。
type CreateStoryArcArgs struct {
	StoryArcs []CreateStoryArcItem `json:"story_arcs" jsonschema:"required,description=List of arcs to create (1-5)" validate:"required,min=1,max=5,dive"`
}

// CreateStoryArcTool 创建新叙事弧线。
type CreateStoryArcTool struct{}

func (t *CreateStoryArcTool) Name() string { return "create_story_arc" }
func (t *CreateStoryArcTool) Description() string {
	return "Batch-create story arcs (1-5). Guarantees atomicity; on failure returns the reason for the specific item." +
		"Arc types: main (main storyline) / sub (sub-plot) / character (character arc) / background (background arc)." +
		"An arc is a storyline container spanning multiple chapters; its nodes are added via create_arc_node."
}
func (t *CreateStoryArcTool) Category() ToolCategory { return CategoryWritingAssistant }

func (t *CreateStoryArcTool) JSONSchema() json.RawMessage { return SchemaOf(CreateStoryArcArgs{}) }
func (t *CreateStoryArcTool) ExposeToLLM() bool           { return true }
func (t *CreateStoryArcTool) NewArgs() any                { return &CreateStoryArcArgs{} }

func (t *CreateStoryArcTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*CreateStoryArcArgs)

	var ids []int64
	var failedName string
	var failedErr error
	err := tc.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range a.StoryArcs {
			importance := item.Importance
			if importance == 0 {
				importance = 1
			}
			arc := storyarc.StoryArc{
				NovelID:     tc.NovelID,
				Name:        item.Name,
				ArcType:     item.ArcType,
				Description: item.Description,
				Importance:  importance,
				Status:      "active",
			}
			if err := tx.Create(&arc).Error; err != nil {
				failedName = item.Name
				failedErr = err
				return err
			}
			ids = append(ids, arc.ID)
		}
		return nil
	})
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Не удалось создать дугу [%s]: %s", failedName, failedErr),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"ids": ids, "count": len(ids)},
	}, nil
}

// ── update_story_arc ───────────────────────────────────

// UpdateStoryArcArgs 是 update_story_arc 的参数。
type UpdateStoryArcArgs struct {
	ArcID        int64  `json:"arc_id" jsonschema:"required,description=Arc ID"                         validate:"required,min=1"`
	Name         string `json:"name" jsonschema:"description=New arc name"`
	Description  string `json:"description" jsonschema:"description=New description"`
	ArcType      string `json:"arc_type" jsonschema:"description=New arc type,enum=main,enum=sub,enum=character,enum=background" validate:"omitempty,oneof=main sub character background"`
	Importance   int    `json:"importance" jsonschema:"description=New importance 1-5,minimum=1,maximum=5"`
	Status       string `json:"status" jsonschema:"description=New status,enum=active,enum=paused,enum=completed,enum=abandoned" validate:"omitempty,oneof=active paused completed abandoned"`
	ReactivateAt string `json:"reactivate_at" jsonschema:"description=Reactivation condition for a paused arc, in natural language. Fill in when setting status to paused"`
}

// UpdateStoryArcTool 更新叙事弧线元数据（PATCH 语义）。
type UpdateStoryArcTool struct{}

func (t *UpdateStoryArcTool) Name() string { return "update_story_arc" }
func (t *UpdateStoryArcTool) Description() string {
	return "Update the metadata of a story arc. Only pass the fields to change." +
		"Common: pause an arc (status=paused + reactivate_at), complete an arc (status=completed), abandon an arc (status=abandoned)."
}
func (t *UpdateStoryArcTool) Category() ToolCategory { return CategoryWritingAssistant }

func (t *UpdateStoryArcTool) JSONSchema() json.RawMessage { return SchemaOf(UpdateStoryArcArgs{}) }
func (t *UpdateStoryArcTool) ExposeToLLM() bool           { return true }
func (t *UpdateStoryArcTool) NewArgs() any                { return &UpdateStoryArcArgs{} }

func (t *UpdateStoryArcTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*UpdateStoryArcArgs)

	if a.Name == "" && a.Description == "" && a.ArcType == "" && a.Importance == 0 && a.Status == "" && a.ReactivateAt == "" {
		return &ToolResult{Success: false, Error: "Необходимо указать хотя бы одно поле для изменения"}, nil
	}

	var arc storyarc.StoryArc
	if err := tc.DB.WithContext(ctx).
		Where("id = ? AND novel_id = ?", a.ArcID, tc.NovelID).
		First(&arc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("Дуга %d не существует", a.ArcID)}, nil
		}
		return nil, fmt.Errorf("query arc: %w", err)
	}

	if err := json.Unmarshal(tc.RawArgs, &arc); err != nil {
		return &ToolResult{Success: false, Error: "Неверный формат параметров: " + err.Error()}, nil
	}

	if err := tc.DB.WithContext(ctx).Save(&arc).Error; err != nil {
		return nil, fmt.Errorf("save arc: %w", err)
	}

	return &ToolResult{Success: true, Data: map[string]any{"id": arc.ID}}, nil
}

// ── create_arc_node ────────────────────────────────────

// CreateArcNodeItem 是 create_arc_node 的单条参数。
type CreateArcNodeItem struct {
	StoryArcID          int64  `json:"arc_id" jsonschema:"required,description=ID of the parent arc"                  validate:"required,min=1"`
	Title               string `json:"title" jsonschema:"required,description=Node title, e.g. 'Discovering the enemy's identity'"              validate:"required"`
	Description         string `json:"description" jsonschema:"description=Node details"`
	TargetReadingNumber int    `json:"target_reading_number" jsonschema:"required,description=Expected reading number at which this happens. Used for the future plan position, not a chapter ID" validate:"required,min=1"`
}

// CreateArcNodeArgs 是 create_arc_node 的参数。
type CreateArcNodeArgs struct {
	ArcNodes []CreateArcNodeItem `json:"arc_nodes" jsonschema:"required,description=List of nodes to create (1-10)" validate:"required,min=1,max=10,dive"`
}

// CreateArcNodeTool 新建弧线节点。
type CreateArcNodeTool struct{}

func (t *CreateArcNodeTool) Name() string { return "create_arc_node" }
func (t *CreateArcNodeTool) Description() string {
	return "Batch-add nodes to an arc (1-10). Guarantees atomicity; on failure returns the reason for the specific item." +
		"target_reading_number is the expected reading number at which this happens (inexact is fine; it can be adjusted later via update_arc_node)." +
		"Nodes are sorted by target_reading_number to form the arc's evolution chain."
}
func (t *CreateArcNodeTool) Category() ToolCategory { return CategoryWritingAssistant }

func (t *CreateArcNodeTool) JSONSchema() json.RawMessage { return SchemaOf(CreateArcNodeArgs{}) }
func (t *CreateArcNodeTool) ExposeToLLM() bool           { return true }
func (t *CreateArcNodeTool) NewArgs() any                { return &CreateArcNodeArgs{} }

func (t *CreateArcNodeTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*CreateArcNodeArgs)

	// 预校验：批量验证所有弧线存在且属于当前小说
	arcIDSet := make(map[int64]bool)
	for _, item := range a.ArcNodes {
		arcIDSet[item.StoryArcID] = true
	}
	arcIDs := make([]int64, 0, len(arcIDSet))
	for id := range arcIDSet {
		arcIDs = append(arcIDs, id)
	}
	var count int64
	if err := tc.DB.WithContext(ctx).Model(&storyarc.StoryArc{}).
		Where("id IN ? AND novel_id = ?", arcIDs, tc.NovelID).
		Count(&count).Error; err != nil {
		return nil, fmt.Errorf("verify arcs: %w", err)
	}
	if int(count) != len(arcIDs) {
		return &ToolResult{Success: false, Error: "Некоторые дуги не существуют или не принадлежат текущему роману"}, nil
	}

	var ids []int64
	var failedName string
	var failedErr error
	err := tc.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range a.ArcNodes {
			node := storyarc.ArcNode{
				NovelID:             tc.NovelID,
				StoryArcID:          item.StoryArcID,
				Title:               item.Title,
				Description:         item.Description,
				TargetReadingNumber: item.TargetReadingNumber,
				Status:              "pending",
			}
			if err := tx.Create(&node).Error; err != nil {
				failedName = item.Title
				failedErr = err
				return err
			}
			ids = append(ids, node.ID)
		}
		return nil
	})
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Не удалось создать узел [%s]: %s", failedName, failedErr),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"ids": ids, "count": len(ids)},
	}, nil
}

// ── update_arc_node ────────────────────────────────────

// UpdateArcNodeArgs 是 update_arc_node 的参数。
type UpdateArcNodeArgs struct {
	NodeID              int64  `json:"node_id" jsonschema:"required,description=Node ID"              validate:"required,min=1"`
	Title               string `json:"title" jsonschema:"description=New title"`
	Description         string `json:"description" jsonschema:"description=New description"`
	TargetReadingNumber int    `json:"target_reading_number" jsonschema:"description=New target reading number,minimum=1" validate:"omitempty,min=1"`
	ActualChapterID     *int64 `json:"actual_chapter_id" jsonschema:"description=Stable chapter ID where it actually happened (fill in when marking completed)" validate:"omitempty,min=1"`
	Status              string `json:"status" jsonschema:"description=New status,enum=pending,enum=completed,enum=abandoned" validate:"omitempty,oneof=pending completed abandoned"`
}

// UpdateArcNodeTool 更新弧线节点（PATCH 语义）。
type UpdateArcNodeTool struct{}

func (t *UpdateArcNodeTool) Name() string { return "update_arc_node" }
func (t *UpdateArcNodeTool) Description() string {
	return "Update an existing arc node. Only pass the fields to change." +
		"When marking as completed, fill in actual_chapter_id + status=completed. Adjusting target_reading_number changes the node's position in the arc chain."
}
func (t *UpdateArcNodeTool) Category() ToolCategory { return CategoryWritingAssistant }

func (t *UpdateArcNodeTool) JSONSchema() json.RawMessage { return SchemaOf(UpdateArcNodeArgs{}) }
func (t *UpdateArcNodeTool) ExposeToLLM() bool           { return true }
func (t *UpdateArcNodeTool) NewArgs() any                { return &UpdateArcNodeArgs{} }

func (t *UpdateArcNodeTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*UpdateArcNodeArgs)

	if a.Title == "" && a.Description == "" && a.TargetReadingNumber == 0 && a.ActualChapterID == nil && a.Status == "" {
		return &ToolResult{Success: false, Error: "Необходимо указать хотя бы одно поле для изменения"}, nil
	}

	var node storyarc.ArcNode
	if err := tc.DB.WithContext(ctx).
		Where("id = ? AND novel_id = ?", a.NodeID, tc.NovelID).
		First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("Узел %d не существует", a.NodeID)}, nil
		}
		return nil, fmt.Errorf("query node: %w", err)
	}
	chapterIDs := make([]int64, 0, 1)
	if a.ActualChapterID != nil {
		chapterIDs = append(chapterIDs, *a.ActualChapterID)
	}
	if result, err := ensureChapterIDsInNovel(ctx, tc, chapterIDs); err != nil || result != nil {
		return result, err
	}

	if err := json.Unmarshal(tc.RawArgs, &node); err != nil {
		return &ToolResult{Success: false, Error: "Неверный формат параметров: " + err.Error()}, nil
	}

	if err := tc.DB.WithContext(ctx).Save(&node).Error; err != nil {
		return nil, fmt.Errorf("save node: %w", err)
	}

	return &ToolResult{Success: true, Data: map[string]any{"id": node.ID}}, nil
}

// ── 格式化 ──────────────────────────────────────────────

func formatArcsAll(activeArcs []storyarc.StoryArc, beforeByArc, anomalyByArc, afterByArc map[int64][]storyarc.ArcNode, pausedArcs []storyarc.StoryArc, pausedBefore, pausedPending map[int64][]storyarc.ArcNode, archivedArcs []storyarc.StoryArc, currentChapter int, readingNumbers map[int64]int) string {
	if len(activeArcs) == 0 && len(pausedArcs) == 0 && len(archivedArcs) == 0 {
		return "No story arcs yet."
	}

	var parts []string
	parts = append(parts, "### Story Arcs")

	for _, arc := range activeArcs {
		parts = append(parts, formatActiveArc(arc,
			beforeByArc[arc.ID], anomalyByArc[arc.ID], afterByArc[arc.ID], currentChapter, readingNumbers))
	}

	for _, arc := range pausedArcs {
		parts = append(parts, formatPausedArc(arc, pausedBefore[arc.ID], pausedPending[arc.ID], readingNumbers))
	}

	for _, arc := range archivedArcs {
		parts = append(parts, formatArchivedArc(arc))
	}

	return strings.Join(parts, "\n")
}

func formatActiveArc(arc storyarc.StoryArc, before, anomalies, after []storyarc.ArcNode, currentChapter int, readingNumbers map[int64]int) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("\n#### %s [arc_id:%d] (%s) — %s %s",
		arc.Name, arc.ID, arc.ArcType, arc.Status, importanceStars(arc.Importance)))
	if arc.Description != "" {
		parts = append(parts, arc.Description)
	}

	if len(before) == 0 && len(anomalies) == 0 && len(after) == 0 {
		parts = append(parts, "(No nodes yet)")
		return strings.Join(parts, "\n")
	}

	if len(before) > 0 {
		lines := []string{fmt.Sprintf("\n##### Recent (last %d nodes, up to chapter %d)", len(before), currentChapter)}
		for _, n := range before {
			lines = append(lines, formatNode(n, false, readingNumbers))
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}

	if len(anomalies) > 0 {
		lines := []string{"\n##### ⚠️ Anomalies", "The following nodes need attention and correction:"}
		for _, n := range anomalies {
			lines = append(lines, fmt.Sprintf("- %s [node_id:%d] — target chapter %d but still pending; should be completed before chapter %d",
				n.Title, n.ID, n.TargetReadingNumber, currentChapter))
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}

	if len(after) > 0 {
		lines := []string{fmt.Sprintf("\n##### Future (%d nodes)", len(after))}
		for _, n := range after {
			lines = append(lines, formatNode(n, false, readingNumbers))
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}

	return strings.Join(parts, "\n")
}

func formatPausedArc(arc storyarc.StoryArc, before, pending []storyarc.ArcNode, readingNumbers map[int64]int) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("\n#### %s [arc_id:%d] (%s) — paused %s",
		arc.Name, arc.ID, arc.ArcType, importanceStars(arc.Importance)))
	if arc.ReactivateAt != "" {
		parts = append(parts, fmt.Sprintf("Reactivation condition: %s", arc.ReactivateAt))
	}

	if len(before) == 0 && len(pending) == 0 {
		return strings.Join(parts, "\n")
	}

	parts = append(parts, "")
	for _, n := range before {
		parts = append(parts, formatNode(n, false, readingNumbers))
	}
	for i, n := range pending {
		if i == 0 {
			parts = append(parts, fmt.Sprintf("- ▶ %s [node_id:%d] — target chapter %d", n.Title, n.ID, n.TargetReadingNumber))
		} else {
			parts = append(parts, fmt.Sprintf("- %s [node_id:%d] — target chapter %d", n.Title, n.ID, n.TargetReadingNumber))
		}
	}

	return strings.Join(parts, "\n")
}

func formatArchivedArc(arc storyarc.StoryArc) string {
	return fmt.Sprintf("\n#### %s [arc_id:%d] (%s) — %s %s",
		arc.Name, arc.ID, arc.ArcType, arc.Status, importanceStars(arc.Importance))
}

func formatArcsFull(arcs []storyarc.StoryArc, nodes []storyarc.ArcNode, readingNumbers map[int64]int) string {
	if len(arcs) == 0 {
		return "No story arcs yet."
	}

	var parts []string
	parts = append(parts, "### Story Arcs")

	nodesByArc := groupNodesByArc(nodes)

	for _, arc := range arcs {
		parts = append(parts, fmt.Sprintf("\n#### %s [arc_id:%d] (%s) — %s %s",
			arc.Name, arc.ID, arc.ArcType, arc.Status, importanceStars(arc.Importance)))
		if arc.Description != "" {
			parts = append(parts, arc.Description)
		}
		if arc.Status == "paused" && arc.ReactivateAt != "" {
			parts = append(parts, fmt.Sprintf("Reactivation condition: %s", arc.ReactivateAt))
		}

		arcNodes := nodesByArc[arc.ID]
		if len(arcNodes) == 0 {
			parts = append(parts, "(No nodes yet)")
			continue
		}
		for _, n := range arcNodes {
			parts = append(parts, formatNode(n, true, readingNumbers))
		}
	}

	return strings.Join(parts, "\n")
}

func formatNode(n storyarc.ArcNode, showStatus bool, readingNumbers map[int64]int) string {
	statusStr := ""
	if n.ActualChapterID != nil {
		if readingNumber, ok := readingNumbers[*n.ActualChapterID]; ok {
			statusStr = fmt.Sprintf(" — chapter %d ✓ [chapter_id:%d]", readingNumber, *n.ActualChapterID)
		} else {
			statusStr = fmt.Sprintf(" — occurred [chapter_id:%d]", *n.ActualChapterID)
		}
	} else if n.TargetReadingNumber > 0 {
		statusStr = fmt.Sprintf(" — target chapter %d", n.TargetReadingNumber)
	}
	if showStatus && n.Status == "abandoned" {
		statusStr += " — abandoned"
	}

	return fmt.Sprintf("- %s [node_id:%d]%s", n.Title, n.ID, statusStr)
}

func groupNodesByArc(nodes []storyarc.ArcNode) map[int64][]storyarc.ArcNode {
	m := make(map[int64][]storyarc.ArcNode)
	for _, n := range nodes {
		m[n.StoryArcID] = append(m[n.StoryArcID], n)
	}
	return m
}

func importanceStars(v int) string {
	if v <= 0 {
		return ""
	}
	s := "⭐"
	for i := 1; i < v && i < 5; i++ {
		s += "⭐"
	}
	return s
}

// ── 注册 ──────────────────────────────────────────────

// RegisterStoryArcTools 注册叙事弧线管理类工具。
func RegisterStoryArcTools(r *Registry) {
	r.Register(&GetStoryArcsTool{})
	r.Register(&CreateStoryArcTool{})
	r.Register(&UpdateStoryArcTool{})
	r.Register(&CreateArcNodeTool{})
	r.Register(&UpdateArcNodeTool{})
}
