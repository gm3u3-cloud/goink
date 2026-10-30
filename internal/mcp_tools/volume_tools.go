package mcp_tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sigpanic/goink/internal/git"
	"github.com/sigpanic/goink/internal/volume"
)

func RegisterVolumeTools(r *Registry) {
	r.Register(&GetVolumesTool{})
	r.Register(&PlaceVolumeTool{})
	r.Register(&UpdateVolumeTool{})
}

type GetVolumesArgs struct{}

type GetVolumesTool struct{}

func (t *GetVolumesTool) Name() string { return "get_volumes" }
func (t *GetVolumesTool) Description() string {
	return "Query all volumes of the current novel, returning volume name, volume ID, and volume outline path in reading order. Read outline content via read; outline files not yet written may not exist."
}
func (t *GetVolumesTool) Category() ToolCategory      { return CategoryNovelManagement }
func (t *GetVolumesTool) JSONSchema() json.RawMessage { return SchemaOf(GetVolumesArgs{}) }
func (t *GetVolumesTool) ExposeToLLM() bool           { return false }
func (t *GetVolumesTool) NewArgs() any                { return &GetVolumesArgs{} }

func (t *GetVolumesTool) Execute(ctx context.Context, _ any, tc ToolContext) (*ToolResult, error) {
	volumes, err := volume.NewStore(tc.DB, tc.LoggerOrDefault()).ListByNovel(ctx, nil, tc.NovelID)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса томов романа: %w", err)
	}
	var content strings.Builder
	if len(volumes) == 0 {
		content.WriteString("The current novel has no volumes.")
	} else {
		fmt.Fprintf(&content, "A total of %d volumes:\n", len(volumes))
		for i, v := range volumes {
			fmt.Fprintf(&content, "\n- Volume %d %s [volume_id:%d]; outline: `%s`", i+1, v.Name, v.ID, v.OutlineFilePath)
		}
	}
	return &ToolResult{Success: true, Data: map[string]any{"content": content.String()}}, nil
}

type PlaceVolumeArgs struct {
	SourceVolumeID *int64  `json:"source_volume_id,omitempty" jsonschema:"description=volume ID to move; omit when creating, mutually exclusive with name" validate:"required_without=Name,excluded_with=Name,omitempty,min=1"`
	Name           *string `json:"name,omitempty" jsonschema:"description=name of the new volume; omit when moving, mutually exclusive with source_volume_id" validate:"required_without=SourceVolumeID,excluded_with=SourceVolumeID,omitempty"`
	BeforeVolumeID *int64  `json:"before_volume_id,omitempty" jsonschema:"description=place before this volume; omit or null to place at the end of the novel" validate:"omitempty,min=1"`
}

type PlaceVolumeTool struct{}

func (t *PlaceVolumeTool) Name() string { return "place_volume" }
func (t *PlaceVolumeTool) Description() string {
	return "Create or move a single volume of the current novel. Pass name to create, or source_volume_id to move; the two are mutually exclusive. before_volume_id specifies which existing volume to place before; omit or null to place at the end. Creating does not generate chapter or volume outline files; write the outline with edit, and rename with update_volume."
}
func (t *PlaceVolumeTool) Category() ToolCategory      { return CategoryNovelManagement }
func (t *PlaceVolumeTool) JSONSchema() json.RawMessage { return SchemaOf(PlaceVolumeArgs{}) }
func (t *PlaceVolumeTool) ExposeToLLM() bool           { return false }
func (t *PlaceVolumeTool) NewArgs() any                { return &PlaceVolumeArgs{} }

func (t *PlaceVolumeTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*PlaceVolumeArgs)
	if a.Name != nil && strings.TrimSpace(*a.Name) == "" {
		return &ToolResult{Success: false, Error: "Название тома не может быть пустым"}, nil
	}
	v, err := volume.NewStore(tc.DB, tc.LoggerOrDefault()).Place(ctx, nil, volume.PlaceInput{
		NovelID: tc.NovelID, SourceVolumeID: a.SourceVolumeID,
		Name: a.Name, BeforeVolumeID: a.BeforeVolumeID,
	})
	if err != nil {
		if errors.Is(err, volume.ErrNameTaken) || errors.Is(err, volume.ErrNotFound) {
			return &ToolResult{Success: false, Error: err.Error()}, nil
		}
		return nil, fmt.Errorf("ошибка размещения тома: %w", err)
	}
	emitFileMetadataChanged(ctx, tc.NovelID, v.OutlineFilePath)
	return &ToolResult{Success: true, Data: map[string]any{"volume_id": v.ID}}, nil
}

type UpdateVolumeArgs struct {
	VolumeID int64  `json:"volume_id" jsonschema:"required,description=volume ID to rename" validate:"required,min=1"`
	Name     string `json:"name" jsonschema:"required,description=new volume name, must be unique within the novel" validate:"required"`
}

type UpdateVolumeTool struct{}

func (t *UpdateVolumeTool) Name() string { return "update_volume" }
func (t *UpdateVolumeTool) Description() string {
	return "Rename a single volume of the current novel, without changing volume order, chapter assignment, or the outline path. Use place_volume to adjust volume order."
}
func (t *UpdateVolumeTool) Category() ToolCategory      { return CategoryNovelManagement }
func (t *UpdateVolumeTool) JSONSchema() json.RawMessage { return SchemaOf(UpdateVolumeArgs{}) }
func (t *UpdateVolumeTool) ExposeToLLM() bool           { return false }
func (t *UpdateVolumeTool) NewArgs() any                { return &UpdateVolumeArgs{} }

func (t *UpdateVolumeTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*UpdateVolumeArgs)
	if strings.TrimSpace(a.Name) == "" {
		return &ToolResult{Success: false, Error: "Название тома не может быть пустым"}, nil
	}
	if err := volume.NewStore(tc.DB, tc.LoggerOrDefault()).Update(ctx, nil, tc.NovelID, a.VolumeID, a.Name); err != nil {
		if errors.Is(err, volume.ErrNameTaken) || errors.Is(err, volume.ErrNotFound) {
			return &ToolResult{Success: false, Error: err.Error()}, nil
		}
		return nil, fmt.Errorf("ошибка переименования тома: %w", err)
	}
	emitFileMetadataChanged(ctx, tc.NovelID, git.VolumePath(a.VolumeID))
	return &ToolResult{Success: true, Data: map[string]any{"message": "Volume name updated"}}, nil
}
