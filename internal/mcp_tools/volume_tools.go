package mcp_tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sigpanic/goink/internal/git"
	"github.com/sigpanic/goink/internal/volume"
	"gorm.io/gorm"
)

func RegisterVolumeTools(r *Registry) {
	r.Register(&GetVolumesTool{})
	r.Register(&CreateVolumeTool{})
}

type GetVolumesArgs struct{}

type GetVolumesTool struct{}

func (t *GetVolumesTool) Name() string { return "get_volumes" }
func (t *GetVolumesTool) Description() string {
	return "查询当前小说的全部分卷，按阅读顺序返回卷名、卷 ID 和卷纲路径。卷纲内容通过 read 读取；尚未编写的卷纲文件可能不存在。"
}
func (t *GetVolumesTool) Category() ToolCategory      { return CategoryNovelManagement }
func (t *GetVolumesTool) JSONSchema() json.RawMessage { return SchemaOf(GetVolumesArgs{}) }
func (t *GetVolumesTool) ExposeToLLM() bool           { return false }
func (t *GetVolumesTool) NewArgs() any                { return &GetVolumesArgs{} }

func (t *GetVolumesTool) Execute(ctx context.Context, _ any, tc ToolContext) (*ToolResult, error) {
	volumes, err := volume.NewStore(tc.DB, tc.LoggerOrDefault()).ListByNovel(ctx, nil, tc.NovelID)
	if err != nil {
		return nil, fmt.Errorf("查询小说分卷: %w", err)
	}
	var content strings.Builder
	if len(volumes) == 0 {
		content.WriteString("当前小说暂无分卷。")
	} else {
		fmt.Fprintf(&content, "共 %d 卷：\n", len(volumes))
		for i, v := range volumes {
			fmt.Fprintf(&content, "\n- 第%d卷 %s [volume_id:%d]；卷纲：`%s`", i+1, v.Name, v.ID, v.OutlineFilePath)
		}
	}
	return &ToolResult{Success: true, Data: map[string]any{"content": content.String()}}, nil
}

type CreateVolumeItem struct {
	Name           string `json:"name" jsonschema:"required,description=卷名，同一小说内不可重复" validate:"required"`
	BeforeVolumeID *int64 `json:"before_volume_id,omitempty" jsonschema:"description=插入到此卷之前，省略则追加到小说末尾" validate:"omitempty,min=1"`
}

type CreateVolumeArgs struct {
	Volumes []CreateVolumeItem `json:"volumes" jsonschema:"required,minItems=1,maxItems=10,description=要创建的分卷（1-10个），按数组顺序创建" validate:"required,min=1,max=10,dive"`
}

type CreateVolumeTool struct{}

func (t *CreateVolumeTool) Name() string { return "create_volume" }
func (t *CreateVolumeTool) Description() string {
	return "批量创建当前小说的分卷（建卷，1-10个），失败时整批回滚。默认追加到末尾，也可通过 before_volume_id 插入到已有卷之前。只创建卷记录，不创建章节或卷纲文件；卷纲可随后用 edit 编写。"
}
func (t *CreateVolumeTool) Category() ToolCategory      { return CategoryNovelManagement }
func (t *CreateVolumeTool) JSONSchema() json.RawMessage { return SchemaOf(CreateVolumeArgs{}) }
func (t *CreateVolumeTool) ExposeToLLM() bool           { return false }
func (t *CreateVolumeTool) NewArgs() any                { return &CreateVolumeArgs{} }

func (t *CreateVolumeTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*CreateVolumeArgs)
	names := make(map[string]bool, len(a.Volumes))
	for i, item := range a.Volumes {
		if strings.TrimSpace(item.Name) == "" {
			return &ToolResult{Success: false, Error: fmt.Sprintf("第%d项卷名不能为空", i+1)}, nil
		}
		if names[item.Name] {
			return &ToolResult{Success: false, Error: fmt.Sprintf("第%d项卷名与本批次其他项重复", i+1)}, nil
		}
		names[item.Name] = true
	}
	store := volume.NewStore(tc.DB, tc.LoggerOrDefault())
	ids := make([]int64, 0, len(a.Volumes))
	err := tc.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, err := store.ListByNovel(ctx, tx, tc.NovelID)
		if err != nil {
			return err
		}
		existingNames := make(map[string]bool, len(existing))
		existingIDs := make(map[int64]bool, len(existing))
		for _, v := range existing {
			existingNames[v.Name] = true
			existingIDs[v.ID] = true
		}
		for i, item := range a.Volumes {
			if existingNames[item.Name] {
				return businessErrorf("第%d项卷名已存在", i+1)
			}
			if item.BeforeVolumeID != nil && !existingIDs[*item.BeforeVolumeID] {
				return businessErrorf("第%d项指定的前置卷不存在或不属于当前小说", i+1)
			}
		}
		for i, item := range a.Volumes {
			v, err := store.Place(ctx, tx, volume.PlaceInput{
				NovelID: tc.NovelID, Name: &item.Name, BeforeVolumeID: item.BeforeVolumeID,
			})
			if errors.Is(err, volume.ErrNameTaken) || errors.Is(err, volume.ErrNotFound) {
				return businessErrorf("第%d项创建失败：%s", i+1, err)
			}
			if err != nil {
				return fmt.Errorf("第%d项创建失败: %w", i+1, err)
			}
			ids = append(ids, v.ID)
		}
		return nil
	})
	if err != nil {
		if isBusinessError(err) {
			return &ToolResult{Success: false, Error: err.Error()}, nil
		}
		return nil, fmt.Errorf("创建分卷失败，事务已回滚: %w", err)
	}
	emitFileMetadataChanged(ctx, tc.NovelID, git.VolumePath(ids[0]))
	return &ToolResult{Success: true, Data: map[string]any{"ids": ids, "count": len(ids)}}, nil
}
