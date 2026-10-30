package mcp_tools

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

	"github.com/sigpanic/goink/internal/preference"
)

// ── upsert_preference ─────────────────────────────────

// UpsertPreferenceItem 是 upsert_preference 的单条参数。
type UpsertPreferenceItem struct {
	PreferenceID *int64 `json:"preference_id,omitempty" jsonschema:"description=existing preference ID; omit to create a new one"`
	Category     string `json:"category" jsonschema:"required,description=preference category (free-text label)" validate:"required"`
	Content      string `json:"content" jsonschema:"required,description=preference content" validate:"required"`
	IsGlobal     *bool  `json:"is_global,omitempty" jsonschema:"description=whether this is a global preference (applies to all novels); omit to default false on create, keep the original value on update"`
}

// UpsertPreferenceArgs 是 upsert_preference 的参数。
type UpsertPreferenceArgs struct {
	Preferences []UpsertPreferenceItem `json:"preferences" jsonschema:"required,description=list of preferences to upsert (1-5; pass preference_id to update, omit to create)" validate:"required,min=1,max=5,dive"`
}

// UpsertPreferenceTool 批量 upsert 创作偏好（创建或更新，单事务原子性）。
type UpsertPreferenceTool struct{}

func (t *UpsertPreferenceTool) Name() string { return "upsert_preference" }
func (t *UpsertPreferenceTool) Description() string {
	return "Bulk-create or update writing preferences (1-5, single-transaction atomicity, all rolled back on failure). " +
		"Pass preference_id to update that entry (PATCH semantics, only the provided fields are overwritten); omit to create a new one. " +
		"All preferences are injected into the context at the start (with a [preference_id:N | category] prefix); when updating, take the preference_id from the injection and pass it in. " +
		"Before adding a preference in a similar category, prefer updating an existing entry (merging into the original text) rather than creating a duplicate."
}
func (t *UpsertPreferenceTool) Category() ToolCategory { return CategoryWritingAssistant }

func (t *UpsertPreferenceTool) JSONSchema() json.RawMessage { return SchemaOf(UpsertPreferenceArgs{}) }
func (t *UpsertPreferenceTool) ExposeToLLM() bool           { return true }
func (t *UpsertPreferenceTool) NewArgs() any                { return &UpsertPreferenceArgs{} }

func (t *UpsertPreferenceTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*UpsertPreferenceArgs)

	type failure struct {
		index    int
		category string
		err      error
	}
	var failed *failure
	var ids []int64

	err := tc.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, item := range a.Preferences {
			id, err := upsertOnePreference(tx, tc.NovelID, item)
			if err != nil {
				failed = &failure{index: i, category: item.Category, err: err}
				return err
			}
			ids = append(ids, id)
		}
		return nil
	})
	if err != nil {
		if failed == nil {
			return nil, fmt.Errorf("upsert предпочтения: %w", err)
		}
		if !isBusinessError(failed.err) {
			return nil, fmt.Errorf("upsert предпочтения, запись %d [%s] (транзакция откатлена): %w", failed.index, failed.category, failed.err)
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("предпочтение %d [%s] не удалось применить: %s", failed.index, failed.category, failed.err),
			Data:    map[string]any{"failed_index": failed.index, "failed_category": failed.category},
		}, nil
	}

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"preference_ids": ids, "count": len(ids)},
	}, nil
}

// upsertOnePreference 在事务内 upsert 单条偏好，返回偏好 ID。
func upsertOnePreference(tx *gorm.DB, novelID int64, item UpsertPreferenceItem) (int64, error) {
	// 更新分支
	if item.PreferenceID != nil {
		var existing preference.PreferenceItem
		if err := tx.First(&existing, *item.PreferenceID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return 0, businessErrorf("предпочтение %d не существует", *item.PreferenceID)
			}
			return 0, fmt.Errorf("запрос предпочтения: %w", err)
		}
		// 归属校验：只能改全局偏好或当前小说的偏好
		if !existing.IsGlobal && existing.NovelID != novelID {
			return 0, businessErrorf("предпочтение %d не относится к текущему роману", *item.PreferenceID)
		}
		// PATCH 覆盖（指针字段判断"传没传"，值字段 required 总传值）
		if item.Category != "" {
			existing.Category = item.Category
		}
		if item.Content != "" {
			existing.Content = item.Content
		}
		if item.IsGlobal != nil {
			existing.IsGlobal = *item.IsGlobal
			// is_global 切换时 NovelID 联动调整（rawArgs patch 表达不了的联动，手动补）
			if *item.IsGlobal {
				existing.NovelID = 0
			} else {
				existing.NovelID = novelID
			}
		}
		if err := tx.Save(&existing).Error; err != nil {
			return 0, fmt.Errorf("сохранение предпочтения: %w", err)
		}
		return existing.ID, nil
	}

	// 创建分支
	pref := preference.PreferenceItem{
		Category: item.Category,
		Content:  item.Content,
		NovelID:  novelID,
	}
	if item.IsGlobal != nil && *item.IsGlobal {
		pref.IsGlobal = true
		pref.NovelID = 0
	}
	if err := tx.Create(&pref).Error; err != nil {
		return 0, fmt.Errorf("создание предпочтения: %w", err)
	}
	return pref.ID, nil
}

// ── 注册 ──────────────────────────────────────────────

// RegisterPreferenceTools 注册偏好工具。
func RegisterPreferenceTools(r *Registry) {
	r.Register(&UpsertPreferenceTool{})
}
