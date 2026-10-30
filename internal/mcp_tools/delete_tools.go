package mcp_tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/sigpanic/goink/internal/character"
	"github.com/sigpanic/goink/internal/location"
	"github.com/sigpanic/goink/internal/preference"
	"github.com/sigpanic/goink/internal/reader"
	"github.com/sigpanic/goink/internal/setting"
	"github.com/sigpanic/goink/internal/storyarc"
	"github.com/sigpanic/goink/internal/timeline"
)

// ── delete_record ──────────────────────────────────────

const deleteRecordTableMapping = `
table allowed values and their corresponding tools:
  character                 — create_character / get_characters / update_character
  character_relation        — update_character_relationship / get_character_relations
  location                  — create_location / get_locations / update_location
  location_relation         — create_location_relation / update_location_relation
  timeline_entry            — create_timeline_entry / get_timeline_entries / update_timeline_entry
  story_arc                 — create_story_arc / get_story_arcs / update_story_arc
  arc_node                  — create_arc_node
  reader_perspective_entry  — create_reader_perspective_entry
  preference                — upsert_preference
  setting                   — upsert_setting`

// DeleteRecordArgs 是 delete_record 的参数。
//
//nolint:raw_id — 通用删除工具，多表共用一个 id 字段，裸 id 合理
type DeleteRecordArgs struct {
	Table string `json:"table" jsonschema:"required,description=table name to delete,enum=character,enum=character_relation,enum=location,enum=location_relation,enum=timeline_entry,enum=story_arc,enum=arc_node,enum=reader_perspective_entry,enum=preference,enum=setting" validate:"required,oneof=character character_relation location location_relation timeline_entry story_arc arc_node reader_perspective_entry preference setting"`
	ID    int64  `json:"id"    jsonschema:"required,description=primary key ID"                                                                validate:"required,min=1"`
}

// DeleteRecordTool 统一删除任意表的单条记录。
type DeleteRecordTool struct{}

func (t *DeleteRecordTool) Name() string { return "delete_record" }
func (t *DeleteRecordTool) Description() string {
	return "Delete a single record from the given table. Before deleting it automatically checks related data; if related data exists it refuses to delete and returns the impact list. " +
		"Only use this tool when you are sure the record needs to be deleted; if a record is deleted by mistake, tell the user to roll it back via the operation log. " +
		"\n\n" + deleteRecordTableMapping
}
func (t *DeleteRecordTool) Category() ToolCategory { return CategoryNovelManagement }

func (t *DeleteRecordTool) JSONSchema() json.RawMessage { return SchemaOf(DeleteRecordArgs{}) }
func (t *DeleteRecordTool) ExposeToLLM() bool           { return true }
func (t *DeleteRecordTool) NewArgs() any                { return &DeleteRecordArgs{} }

func (t *DeleteRecordTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*DeleteRecordArgs)

	switch a.Table {
	case "character":
		return t.deleteCharacter(ctx, a, tc)
	case "character_relation":
		return t.deleteCharacterRelation(ctx, a, tc)
	case "location":
		return t.deleteLocation(ctx, a, tc)
	case "location_relation":
		return t.deleteLocationRelation(ctx, a, tc)
	case "timeline_entry":
		return t.deleteTimelineEntry(ctx, a, tc)
	case "story_arc":
		return t.deleteStoryArc(ctx, a, tc)
	case "arc_node":
		return t.deleteArcNode(ctx, a, tc)
	case "reader_perspective_entry":
		return t.deleteReaderPerspectiveEntry(ctx, a, tc)
	case "preference":
		return t.deletePreference(ctx, a, tc)
	case "setting":
		return t.deleteSetting(ctx, a, tc)
	default:
		return &ToolResult{Success: false, Error: fmt.Sprintf("неподдерживаемая таблица: %s", a.Table)}, nil
	}
}

// ── helper ──────────────────────────────────────────────

// requestDeleteApproval 发起删除审批。
// 返回 (injects, result, error)：injects 非 nil 表示审批通过且有用户反馈，
// result 非 nil 表示被拒绝或中断（调用方直接返回），error 为系统错误。
func requestDeleteApproval(ctx context.Context, tc ToolContext, payload map[string]any) ([]InjectMessage, *ToolResult, error) {
	if tc.Approver == nil {
		return nil, nil, nil
	}
	if tc.EmitApproval != nil {
		tc.EmitApproval(tc.ToolID, "delete", payload)
	}
	approval, err := tc.Approver.RequestApproval(ctx, tc.ToolID, payload)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, &ToolResult{Success: false, Error: "операция прервана"}, nil
		}
		return nil, nil, fmt.Errorf("одобрение: %w", err)
	}
	if !approval.Approved {
		info := "The user rejected the deletion"
		if approval.Feedback != "" {
			info += ". User feedback: " + approval.Feedback
		}
		return nil, &ToolResult{
			Success: false,
			Error:   "Удаление отклонено",
			Data:    map[string]any{"approved": false},
			Inject:  []InjectMessage{{Role: "user", Content: info}},
		}, nil
	}
	if approval.Feedback != "" {
		return []InjectMessage{{Role: "user", Content: "The user approved the deletion with feedback: " + approval.Feedback}}, nil, nil
	}
	return nil, nil, nil
}

// ── 各表删除方法 ────────────────────────────────────────

func (t *DeleteRecordTool) deleteCharacter(ctx context.Context, a *DeleteRecordArgs, tc ToolContext) (*ToolResult, error) {
	var rec character.Character
	if err := tc.DB.WithContext(ctx).Where("id = ? AND novel_id = ?", a.ID, tc.NovelID).First(&rec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("персонаж %d не существует или не относится к текущему роману", a.ID)}, nil
		}
		return nil, fmt.Errorf("запрос персонажа: %w", err)
	}

	// 检查关联的角色关系
	var relCount int64
	if err := tc.DB.WithContext(ctx).Model(&character.CharacterRelation{}).
		Where("(source_character_id = ? OR target_character_id = ?) AND novel_id = ?", a.ID, a.ID, tc.NovelID).
		Count(&relCount).Error; err != nil {
		return nil, fmt.Errorf("подсчёт связей: %w", err)
	}
	if relCount > 0 {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("у персонажа «%s» есть %d связанных отношений, сначала удалите эти связи", rec.Name, relCount),
			Data:    map[string]any{"impact": map[string]any{"character_relations": relCount}},
		}, nil
	}

	meta := map[string]any{"id": rec.ID, "name": rec.Name, "type": "character"}
	injects, result, err := requestDeleteApproval(ctx, tc, map[string]any{
		"table": a.Table, "id": a.ID, "deleted": meta,
	})
	if err != nil || result != nil {
		return result, err
	}

	if err := character.NewStore(tc.DB, tc.LoggerOrDefault()).DeleteCharacter(ctx, tc.NovelID, rec.ID); err != nil {
		return nil, fmt.Errorf("удаление персонажа: %w", err)
	}

	return &ToolResult{Success: true, Data: map[string]any{"deleted": meta}, Inject: injects}, nil
}

func (t *DeleteRecordTool) deleteCharacterRelation(ctx context.Context, a *DeleteRecordArgs, tc ToolContext) (*ToolResult, error) {
	var rec character.CharacterRelation
	if err := tc.DB.WithContext(ctx).Where("id = ? AND novel_id = ?", a.ID, tc.NovelID).First(&rec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("связь персонажей %d не существует или не относится к текущему роману", a.ID)}, nil
		}
		return nil, fmt.Errorf("запрос связи персонажей: %w", err)
	}

	// 解析关联角色名
	nameMap := make(map[int64]string)
	var chars []character.Character
	tc.DB.WithContext(ctx).Where("id IN ? AND novel_id = ?", []int64{rec.SourceCharacterID, rec.TargetCharacterID}, tc.NovelID).Find(&chars)
	for _, ch := range chars {
		nameMap[ch.ID] = ch.Name
	}

	meta := map[string]any{
		"id":       rec.ID,
		"source":   nameMap[rec.SourceCharacterID],
		"target":   nameMap[rec.TargetCharacterID],
		"relation": rec.RelationDescribe,
		"type":     "character_relation",
	}
	injects, result, err := requestDeleteApproval(ctx, tc, map[string]any{
		"table": a.Table, "id": a.ID, "deleted": meta,
	})
	if err != nil || result != nil {
		return result, err
	}

	if err := tc.DB.WithContext(ctx).Delete(&rec).Error; err != nil {
		return nil, fmt.Errorf("удаление связи персонажей: %w", err)
	}

	return &ToolResult{Success: true, Data: map[string]any{"deleted": meta}, Inject: injects}, nil
}

func (t *DeleteRecordTool) deleteLocation(ctx context.Context, a *DeleteRecordArgs, tc ToolContext) (*ToolResult, error) {
	var rec location.Location
	if err := tc.DB.WithContext(ctx).Where("id = ? AND novel_id = ?", a.ID, tc.NovelID).First(&rec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("локация %d не существует или не относится к текущему роману", a.ID)}, nil
		}
		return nil, fmt.Errorf("запрос локации: %w", err)
	}

	impact := map[string]any{}

	// 检查子地点
	var childCount int64
	if err := tc.DB.WithContext(ctx).Model(&location.Location{}).
		Where("parent_location_id = ? AND novel_id = ?", a.ID, tc.NovelID).
		Count(&childCount).Error; err != nil {
		return nil, fmt.Errorf("подсчёт дочерних локаций: %w", err)
	}
	if childCount > 0 {
		impact["child_locations"] = childCount
	}

	// 检查空间关系边
	var relCount int64
	if err := tc.DB.WithContext(ctx).Model(&location.LocationRelation{}).
		Where("(location_a = ? OR location_b = ?) AND novel_id = ?", a.ID, a.ID, tc.NovelID).
		Count(&relCount).Error; err != nil {
		return nil, fmt.Errorf("подсчёт связей локаций: %w", err)
	}
	if relCount > 0 {
		impact["location_relations"] = relCount
	}

	if len(impact) > 0 {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("у локации «%s» есть связанные данные, сначала обработайте их перед удалением", rec.Name),
			Data:    map[string]any{"impact": impact},
		}, nil
	}

	meta := map[string]any{"id": rec.ID, "name": rec.Name, "type": "location"}
	injects, result, err := requestDeleteApproval(ctx, tc, map[string]any{
		"table": a.Table, "id": a.ID, "deleted": meta,
	})
	if err != nil || result != nil {
		return result, err
	}

	if err := tc.DB.WithContext(ctx).Delete(&rec).Error; err != nil {
		return nil, fmt.Errorf("удаление локации: %w", err)
	}

	return &ToolResult{Success: true, Data: map[string]any{"deleted": meta}, Inject: injects}, nil
}

func (t *DeleteRecordTool) deleteLocationRelation(ctx context.Context, a *DeleteRecordArgs, tc ToolContext) (*ToolResult, error) {
	var rec location.LocationRelation
	if err := tc.DB.WithContext(ctx).Where("id = ? AND novel_id = ?", a.ID, tc.NovelID).First(&rec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("связь локаций %d не существует или не относится к текущему роману", a.ID)}, nil
		}
		return nil, fmt.Errorf("запрос связи локаций: %w", err)
	}

	// 解析关联地点名
	nameMap := make(map[int64]string)
	var locs []location.Location
	tc.DB.WithContext(ctx).Where("id IN ? AND novel_id = ?", []int64{rec.LocationA, rec.LocationB}, tc.NovelID).Find(&locs)
	for _, loc := range locs {
		nameMap[loc.ID] = loc.Name
	}

	meta := map[string]any{
		"id":         rec.ID,
		"location_a": nameMap[rec.LocationA],
		"location_b": nameMap[rec.LocationB],
		"relation":   rec.RelationType,
		"type":       "location_relation",
	}
	injects, result, err := requestDeleteApproval(ctx, tc, map[string]any{
		"table": a.Table, "id": a.ID, "deleted": meta,
	})
	if err != nil || result != nil {
		return result, err
	}

	if err := tc.DB.WithContext(ctx).Delete(&rec).Error; err != nil {
		return nil, fmt.Errorf("удаление связи локаций: %w", err)
	}

	return &ToolResult{Success: true, Data: map[string]any{"deleted": meta}, Inject: injects}, nil
}

func (t *DeleteRecordTool) deleteTimelineEntry(ctx context.Context, a *DeleteRecordArgs, tc ToolContext) (*ToolResult, error) {
	var rec timeline.TimelineEntry
	if err := tc.DB.WithContext(ctx).Where("id = ? AND novel_id = ?", a.ID, tc.NovelID).First(&rec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("запись таймлайна %d не существует или не относится к текущему роману", a.ID)}, nil
		}
		return nil, fmt.Errorf("запрос записи таймлайна: %w", err)
	}

	meta := map[string]any{"id": rec.ID, "title": rec.Title, "type": "timeline_entry"}
	injects, result, err := requestDeleteApproval(ctx, tc, map[string]any{
		"table": a.Table, "id": a.ID, "deleted": meta,
	})
	if err != nil || result != nil {
		return result, err
	}

	if err := tc.DB.WithContext(ctx).Delete(&rec).Error; err != nil {
		return nil, fmt.Errorf("удаление записи таймлайна: %w", err)
	}

	return &ToolResult{Success: true, Data: map[string]any{"deleted": meta}, Inject: injects}, nil
}

func (t *DeleteRecordTool) deleteStoryArc(ctx context.Context, a *DeleteRecordArgs, tc ToolContext) (*ToolResult, error) {
	var rec storyarc.StoryArc
	if err := tc.DB.WithContext(ctx).Where("id = ? AND novel_id = ?", a.ID, tc.NovelID).First(&rec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("сюжетная арка %d не существует или не относится к текущему роману", a.ID)}, nil
		}
		return nil, fmt.Errorf("запрос сюжетной арки: %w", err)
	}

	// 检查关联的弧节点
	var nodeCount int64
	if err := tc.DB.WithContext(ctx).Model(&storyarc.ArcNode{}).
		Where("story_arc_id = ? AND novel_id = ?", a.ID, tc.NovelID).
		Count(&nodeCount).Error; err != nil {
		return nil, fmt.Errorf("подсчёт узлов арки: %w", err)
	}
	if nodeCount > 0 {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("у сюжетной арки «%s» есть %d узлов, сначала удалите эти узлы", rec.Name, nodeCount),
			Data:    map[string]any{"impact": map[string]any{"arc_nodes": nodeCount}},
		}, nil
	}

	meta := map[string]any{"id": rec.ID, "name": rec.Name, "type": "story_arc"}
	injects, result, err := requestDeleteApproval(ctx, tc, map[string]any{
		"table": a.Table, "id": a.ID, "deleted": meta,
	})
	if err != nil || result != nil {
		return result, err
	}

	if err := tc.DB.WithContext(ctx).Delete(&rec).Error; err != nil {
		return nil, fmt.Errorf("удаление сюжетной арки: %w", err)
	}

	return &ToolResult{Success: true, Data: map[string]any{"deleted": meta}, Inject: injects}, nil
}

func (t *DeleteRecordTool) deleteArcNode(ctx context.Context, a *DeleteRecordArgs, tc ToolContext) (*ToolResult, error) {
	var rec storyarc.ArcNode
	if err := tc.DB.WithContext(ctx).Where("id = ? AND novel_id = ?", a.ID, tc.NovelID).First(&rec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("узел арки %d не существует или не относится к текущему роману", a.ID)}, nil
		}
		return nil, fmt.Errorf("запрос узла арки: %w", err)
	}

	// 解析所属故事弧名
	var arc storyarc.StoryArc
	arcName := ""
	if err := tc.DB.WithContext(ctx).Where("id = ? AND novel_id = ?", rec.StoryArcID, tc.NovelID).First(&arc).Error; err == nil {
		arcName = arc.Name
	}

	meta := map[string]any{
		"id":        rec.ID,
		"title":     rec.Title,
		"story_arc": arcName,
		"type":      "arc_node",
	}
	injects, result, err := requestDeleteApproval(ctx, tc, map[string]any{
		"table": a.Table, "id": a.ID, "deleted": meta,
	})
	if err != nil || result != nil {
		return result, err
	}

	if err := tc.DB.WithContext(ctx).Delete(&rec).Error; err != nil {
		return nil, fmt.Errorf("удаление узла арки: %w", err)
	}

	return &ToolResult{Success: true, Data: map[string]any{"deleted": meta}, Inject: injects}, nil
}

func (t *DeleteRecordTool) deleteReaderPerspectiveEntry(ctx context.Context, a *DeleteRecordArgs, tc ToolContext) (*ToolResult, error) {
	var rec reader.ReaderPerspective
	if err := tc.DB.WithContext(ctx).Where("id = ? AND novel_id = ?", a.ID, tc.NovelID).First(&rec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("запись о восприятии читателя %d не существует или не относится к текущему роману", a.ID)}, nil
		}
		return nil, fmt.Errorf("запрос восприятия читателя: %w", err)
	}

	meta := map[string]any{
		"id":                 rec.ID,
		"entry_type":         rec.Type,
		"planted_chapter_id": rec.PlantedChapterID,
		"type":               "reader_perspective_entry",
	}
	injects, result, err := requestDeleteApproval(ctx, tc, map[string]any{
		"table": a.Table, "id": a.ID, "deleted": meta,
	})
	if err != nil || result != nil {
		return result, err
	}

	if err := tc.DB.WithContext(ctx).Delete(&rec).Error; err != nil {
		return nil, fmt.Errorf("удаление восприятия читателя: %w", err)
	}

	return &ToolResult{Success: true, Data: map[string]any{"deleted": meta}, Inject: injects}, nil
}

func (t *DeleteRecordTool) deletePreference(ctx context.Context, a *DeleteRecordArgs, tc ToolContext) (*ToolResult, error) {
	var rec preference.PreferenceItem
	if err := tc.DB.WithContext(ctx).Where("id = ? AND ((novel_id = ? AND is_global = false) OR is_global = true)", a.ID, tc.NovelID).First(&rec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("предпочтение %d не существует", a.ID)}, nil
		}
		return nil, fmt.Errorf("запрос предпочтения: %w", err)
	}

	meta := map[string]any{"id": rec.ID, "category": rec.Category, "type": "preference"}
	injects, result, err := requestDeleteApproval(ctx, tc, map[string]any{
		"table": a.Table, "id": a.ID, "deleted": meta,
	})
	if err != nil || result != nil {
		return result, err
	}

	if err := tc.DB.WithContext(ctx).Delete(&rec).Error; err != nil {
		return nil, fmt.Errorf("удаление предпочтения: %w", err)
	}

	return &ToolResult{Success: true, Data: map[string]any{"deleted": meta}, Inject: injects}, nil
}

func (t *DeleteRecordTool) deleteSetting(ctx context.Context, a *DeleteRecordArgs, tc ToolContext) (*ToolResult, error) {
	var rec setting.SettingItem
	// v2 取消 is_global 后，设定全部归属当前小说，只校验 id + novel_id
	if err := tc.DB.WithContext(ctx).Where("id = ? AND novel_id = ?", a.ID, tc.NovelID).First(&rec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("настройка %d не существует или не относится к текущему роману", a.ID)}, nil
		}
		return nil, fmt.Errorf("запрос настройки: %w", err)
	}

	meta := map[string]any{"id": rec.ID, "category": rec.Category, "type": "setting"}
	injects, result, err := requestDeleteApproval(ctx, tc, map[string]any{
		"table": a.Table, "id": a.ID, "deleted": meta,
	})
	if err != nil || result != nil {
		return result, err
	}

	if err := tc.DB.WithContext(ctx).Delete(&rec).Error; err != nil {
		return nil, fmt.Errorf("удаление настройки: %w", err)
	}

	return &ToolResult{Success: true, Data: map[string]any{"deleted": meta}, Inject: injects}, nil
}

// ── 注册 ────────────────────────────────────────────────

// RegisterDeleteTools 注册删除工具。
func RegisterDeleteTools(r *Registry) {
	r.Register(&DeleteRecordTool{})
}
