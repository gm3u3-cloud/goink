package mcp_tools

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

	"github.com/sigpanic/goink/internal/setting"
)

// ── upsert_setting ──────────────────────────────────

// UpsertSettingItem 是 upsert_setting 的单条参数。
type UpsertSettingItem struct {
	SettingID *int64 `json:"setting_id,omitempty" jsonschema:"description=existing setting ID; omit to create a new one"`
	Category  string `json:"category" jsonschema:"required,description=setting category (free-text, e.g. world-building / power system / characters / geography / history / items)" validate:"required"`
	Content   string `json:"content" jsonschema:"required,description=setting content" validate:"required"`
}

// UpsertSettingArgs 是 upsert_setting 的参数。
type UpsertSettingArgs struct {
	Settings []UpsertSettingItem `json:"settings" jsonschema:"required,description=list of settings to upsert (1-5; pass setting_id to update, omit to create)" validate:"required,min=1,max=5,dive"`
}

// UpsertSettingTool 批量 upsert 世界观设定（创建或更新，单事务原子性）。
type UpsertSettingTool struct{}

func (t *UpsertSettingTool) Name() string { return "upsert_setting" }
func (t *UpsertSettingTool) Description() string {
	return "Bulk-create or update novel settings (1-5, single-transaction atomicity, all rolled back on failure). " +
		"Pass setting_id to update that entry (PATCH semantics, only the provided fields are overwritten); omit to create a new one. " +
		"All settings are injected into the context at the start (with a [setting_id:N | category] prefix); when updating, take the setting_id from the injection and pass it in. " +
		"Before adding a setting in a similar category, prefer updating an existing entry (merging into the original text) rather than creating a duplicate."
}
func (t *UpsertSettingTool) Category() ToolCategory { return CategoryWritingAssistant }

func (t *UpsertSettingTool) JSONSchema() json.RawMessage { return SchemaOf(UpsertSettingArgs{}) }
func (t *UpsertSettingTool) ExposeToLLM() bool           { return true }
func (t *UpsertSettingTool) NewArgs() any                { return &UpsertSettingArgs{} }

func (t *UpsertSettingTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*UpsertSettingArgs)

	type failure struct {
		index    int
		category string
		err      error
	}
	var failed *failure
	var ids []int64

	err := tc.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, item := range a.Settings {
			id, err := upsertOneSetting(tx, tc.NovelID, item)
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
			return nil, fmt.Errorf("upsert настройки: %w", err)
		}
		if !isBusinessError(failed.err) {
			return nil, fmt.Errorf("upsert настройки, запись %d [%s] (транзакция откатлена): %w", failed.index, failed.category, failed.err)
		}
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("настройка %d [%s] не удалось применить: %s", failed.index, failed.category, failed.err),
			Data:    map[string]any{"failed_index": failed.index, "failed_category": failed.category},
		}, nil
	}

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"setting_ids": ids, "count": len(ids)},
	}, nil
}

// upsertOneSetting 在事务内 upsert 单条设定，返回设定 ID。
func upsertOneSetting(tx *gorm.DB, novelID int64, item UpsertSettingItem) (int64, error) {
	// 更新分支
	if item.SettingID != nil {
		var existing setting.SettingItem
		if err := tx.First(&existing, *item.SettingID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return 0, businessErrorf("настройка %d не существует", *item.SettingID)
			}
			return 0, fmt.Errorf("запрос настройки: %w", err)
		}
		// 归属校验：只能改当前小说的设定
		if existing.NovelID != novelID {
			return 0, businessErrorf("настройка %d не относится к текущему роману", *item.SettingID)
		}
		// PATCH 覆盖（值字段 required 总传值，这里仍保留 if 判断以兼容"前端只传部分字段"的旧调用方）
		if item.Category != "" {
			existing.Category = item.Category
		}
		if item.Content != "" {
			existing.Content = item.Content
		}
		if err := tx.Save(&existing).Error; err != nil {
			return 0, fmt.Errorf("сохранение настройки: %w", err)
		}
		return existing.ID, nil
	}

	// 创建分支
	rec := setting.SettingItem{
		Category: item.Category,
		Content:  item.Content,
		NovelID:  novelID,
	}
	if err := tx.Create(&rec).Error; err != nil {
		return 0, fmt.Errorf("создание настройки: %w", err)
	}
	return rec.ID, nil
}

// ── 注册 ──────────────────────────────────────────────

// RegisterSettingTools 注册设定工具。
func RegisterSettingTools(r *Registry) {
	r.Register(&UpsertSettingTool{})
}
