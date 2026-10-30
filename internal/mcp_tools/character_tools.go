package mcp_tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/sigpanic/goink/internal/activity"
	"github.com/sigpanic/goink/internal/character"
	"github.com/sigpanic/goink/internal/storage"
)

// ── get_characters ────────────────────────────────────

// GetCharactersArgs 是 get_characters 的参数。
type GetCharactersArgs struct {
	Mode     string `json:"mode" jsonschema:"description=characters (default) queries characters and their groups; groups queries group names, descriptions, and member counts,enum=characters,enum=groups" validate:"omitempty,oneof=characters groups"`
	GroupID  *int64 `json:"group_id,omitempty" jsonschema:"description=characters mode only: omit to query all, 0 for ungrouped, positive integer for a specific group,minimum=0" validate:"omitempty,min=0"`
	Search   string `json:"search" jsonschema:"description=In characters mode, search by character name; in groups mode, search by group name or description"`
	PageArgs        // 嵌入分页参数
}

// GetCharactersTool 获取角色列表，截断 50 条，冷角色靠搜索。
type GetCharactersTool struct{}

func (t *GetCharactersTool) Name() string { return "get_characters" }
func (t *GetCharactersTool) Description() string {
	return "Get the character list for the current novel. Sorted by most recent update in descending order, truncated to 50 entries — understand the full character lineup before writing. " +
		"Uncommon characters (not updated for a long time) may not appear in the list; use search to find them by name. " +
		"Character results include their groups; you can filter with group_id, where 0 means ungrouped. " +
		"When mode=groups, pages through group names, purpose descriptions, member counts, and group_id (including empty groups); do not pass group_id in this case. " +
		"To understand relationships between characters, use get_character_relations with the character IDs to fetch the subgraph."
}
func (t *GetCharactersTool) Category() ToolCategory { return CategoryNovelManagement }

func (t *GetCharactersTool) JSONSchema() json.RawMessage { return SchemaOf(GetCharactersArgs{}) }
func (t *GetCharactersTool) ExposeToLLM() bool           { return true }
func (t *GetCharactersTool) NewArgs() any                { return &GetCharactersArgs{} }

func (t *GetCharactersTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*GetCharactersArgs)
	if a.Mode == "groups" && a.GroupID != nil {
		return &ToolResult{Success: false, Error: "Нельзя передавать group_id при mode=groups; для запроса персонажей внутри группы используйте mode=characters"}, nil
	}
	a.NormalizePage()

	store := character.NewStore(tc.DB, tc.LoggerOrDefault())
	if a.Mode == "groups" {
		return t.getGroups(ctx, a, tc, store)
	}
	result, err := store.ListByNovel(ctx, tc.NovelID, character.ListByNovelOptions{
		PageParams: storage.PageParams{Page: a.Page, Size: a.Size},
		GroupID:    a.GroupID,
		Search:     a.Search,
		Order:      "updated_at DESC", // MCP 按最近编辑排序，截断时保留活跃角色
	})
	if err != nil {
		return nil, fmt.Errorf("list characters: %w", err)
	}

	characterIDs := make([]int64, len(result.Items))
	for i, ch := range result.Items {
		characterIDs[i] = ch.ID
	}
	groups, err := store.GetGroupsByCharacterIDs(ctx, tc.NovelID, characterIDs)
	if err != nil {
		return nil, fmt.Errorf("get character groups: %w", err)
	}

	items := make([]map[string]any, len(result.Items))
	for i, ch := range result.Items {
		memberships := make([]string, 0, len(groups[ch.ID]))
		for _, group := range groups[ch.ID] {
			memberships = append(memberships, fmt.Sprintf("%s [group_id:%d]", group.Name, group.ID))
		}
		items[i] = map[string]any{
			"id":          ch.ID,
			"name":        ch.Name,
			"description": ch.Description,
			"personality": parseJSONField(ch.Personality),
			"abilities":   parseJSONField(ch.Abilities),
			"groups":      memberships,
		}
	}

	data := PageMeta(result)
	data["characters"] = items

	return &ToolResult{Success: true, Data: data}, nil
}

func (t *GetCharactersTool) getGroups(ctx context.Context, a *GetCharactersArgs, tc ToolContext, store *character.Store) (*ToolResult, error) {
	result, err := store.ListGroups(ctx, tc.NovelID, character.ListGroupsOptions{
		PageParams: storage.PageParams{Page: a.Page, Size: a.Size},
		Search:     a.Search,
		Order:      "name ASC, id ASC",
	})
	if err != nil {
		return nil, fmt.Errorf("list character groups: %w", err)
	}
	lines := make([]string, 0, len(result.Items))
	for _, group := range result.Items {
		line := fmt.Sprintf("- %s [group_id:%d] (%d characters)", group.Name, group.ID, group.MemberCount)
		if group.Description != "" {
			line += ": " + group.Description
		}
		lines = append(lines, line)
	}
	content := strings.Join(lines, "\n")
	if len(lines) == 0 {
		content = "No matching character groups on this page."
	}
	data := PageMeta(result)
	data["content"] = content
	return &ToolResult{Success: true, Data: data}, nil
}

// ── get_character_relations ───────────────────────────

// GetCharacterRelationsArgs 是 get_character_relations 的参数。
type GetCharacterRelationsArgs struct {
	CharacterIDs []int64 `json:"character_ids" jsonschema:"required,description=List of character IDs; only returns relation edges between these characters" validate:"required,min=1"`
}

// GetCharacterRelationsTool 返回给定角色集合内部的子图边。
type GetCharacterRelationsTool struct{}

func (t *GetCharacterRelationsTool) Name() string { return "get_character_relations" }
func (t *GetCharacterRelationsTool) Description() string {
	return "Get the relation edges (subgraph) among the specified characters. Only returns relations where both endpoints are in character_ids, in any direction. " +
		"Typically, first get the character list via get_characters, then pass the character IDs you care about to query their relations."
}
func (t *GetCharacterRelationsTool) Category() ToolCategory { return CategoryMemoryRetrieval }

func (t *GetCharacterRelationsTool) JSONSchema() json.RawMessage {
	return SchemaOf(GetCharacterRelationsArgs{})
}
func (t *GetCharacterRelationsTool) ExposeToLLM() bool { return true }
func (t *GetCharacterRelationsTool) NewArgs() any      { return &GetCharacterRelationsArgs{} }

func (t *GetCharacterRelationsTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*GetCharacterRelationsArgs)

	store := character.NewStore(tc.DB, tc.LoggerOrDefault())

	rels, err := store.ListBetweenCharacters(ctx, a.CharacterIDs)
	if err != nil {
		return nil, fmt.Errorf("get character relations: %w", err)
	}

	// 解析角色名
	chars, err := store.GetByIDs(ctx, a.CharacterIDs)
	if err != nil {
		return nil, fmt.Errorf("get character names: %w", err)
	}
	nameMap := make(map[int64]string, len(chars))
	for _, ch := range chars {
		nameMap[ch.ID] = ch.Name
	}

	formatted := formatRelationEdges(rels, nameMap)

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"content": formatted},
	}, nil
}

func formatRelationEdges(rels []character.CharacterRelation, nameMap map[int64]string) string {
	if len(rels) == 0 {
		return "No relation data yet."
	}

	// 按源角色分组为邻接表
	groups := make(map[int64][]character.CharacterRelation)
	var order []int64
	for _, rel := range rels {
		if _, ok := groups[rel.SourceCharacterID]; !ok {
			order = append(order, rel.SourceCharacterID)
		}
		groups[rel.SourceCharacterID] = append(groups[rel.SourceCharacterID], rel)
	}

	lines := []string{"### Character Relations"}
	for _, srcID := range order {
		srcName := nameMap[srcID]
		var edges []string
		for _, rel := range groups[srcID] {
			edge := fmt.Sprintf("→ %s: %s [relation_id:%d]",
				nameMap[rel.TargetCharacterID], rel.RelationDescribe, rel.ID)
			if rel.Description != "" {
				edge += fmt.Sprintf(" (%s)", rel.Description)
			}
			edges = append(edges, edge)
		}
		lines = append(lines, fmt.Sprintf("- %s %s", srcName, strings.Join(edges, ", ")))
	}
	return strings.Join(lines, "\n")
}

// ── create_character ──────────────────────────────────

// CreateCharacterItem 是 create_character 的单条参数。
type CreateCharacterItem struct {
	Name        string `json:"name"        jsonschema:"required,description=Character name"               validate:"required"`
	Description string `json:"description"  jsonschema:"description=Natural language description of the character, e.g. appearance, identity, background story"`
	Personality string `json:"personality" jsonschema:"description=Free-form JSON object describing the character's personality/role/background, e.g. {\"traits\":[\"brave\"],\"brief\":\"passionate youth\"}"`
	Abilities   string `json:"abilities"   jsonschema:"description=JSON array of the character's abilities/skills, e.g. [\"swordsmanship\",\"stealth\"]"`
}

// CreateCharacterArgs 是 create_character 的参数。
type CreateCharacterArgs struct {
	Characters []CreateCharacterItem `json:"characters" jsonschema:"required,description=List of characters to create (1-10)" validate:"required,min=1,max=10,dive"`
}

// CreateCharacterTool 创建新角色。
type CreateCharacterTool struct{}

func (t *CreateCharacterTool) Name() string { return "create_character" }
func (t *CreateCharacterTool) Description() string {
	return "Batch-create characters (1-10). Guarantees atomicity; on failure, returns the specific entry reason. " +
		"name is required; personality is free-form JSON and should include role/traits/background/motivation; " +
		"abilities is a JSON array. After creation, use get_characters to view them."
}
func (t *CreateCharacterTool) Category() ToolCategory { return CategoryNovelManagement }

func (t *CreateCharacterTool) JSONSchema() json.RawMessage { return SchemaOf(CreateCharacterArgs{}) }
func (t *CreateCharacterTool) ExposeToLLM() bool           { return true }
func (t *CreateCharacterTool) NewArgs() any                { return &CreateCharacterArgs{} }

func (t *CreateCharacterTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*CreateCharacterArgs)

	var ids []int64
	var failedName string
	var failedErr error
	err := tc.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range a.Characters {
			ch := character.Character{
				NovelID:     tc.NovelID,
				Name:        item.Name,
				Description: item.Description,
				Personality: item.Personality,
				Abilities:   item.Abilities,
			}
			if err := tx.Create(&ch).Error; err != nil {
				failedName = item.Name
				failedErr = err
				return err
			}
			ids = append(ids, ch.ID)
		}
		return nil
	})
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Не удалось создать персонажа [%s]: %s", failedName, failedErr),
		}, nil
	}

	activity.NewStore(tc.DB, tc.LoggerOrDefault()).AddActivity(ctx, activity.ActivityDelta{CharactersCreated: int64(len(ids))})

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"ids": ids, "count": len(ids)},
	}, nil
}

// ── update_character ──────────────────────────────────

// UpdateCharacterArgs 是 update_character 的参数。
type UpdateCharacterArgs struct {
	CharacterID int64  `json:"character_id" jsonschema:"required,description=Character ID"     validate:"required,min=1"`
	Name        string `json:"name"         jsonschema:"description=New name"`
	Description string `json:"description"  jsonschema:"description=New natural language description (fully replaces the old one)"`
	Personality string `json:"personality"  jsonschema:"description=New personality/settings, JSON in string form (fully replaces the old one)"`
	Abilities   string `json:"abilities"    jsonschema:"description=New abilities list, JSON in string form (fully replaces the old one)"`
}

// UpdateCharacterTool 更新角色字段。
type UpdateCharacterTool struct{}

func (t *UpdateCharacterTool) Name() string { return "update_character" }
func (t *UpdateCharacterTool) Description() string {
	return "Update the settings of an existing character. Only pass the fields you want to change; unpassed fields stay unchanged. " +
		"personality and abilities fully replace the old values; they are not merged."
}
func (t *UpdateCharacterTool) Category() ToolCategory { return CategoryNovelManagement }

func (t *UpdateCharacterTool) JSONSchema() json.RawMessage { return SchemaOf(UpdateCharacterArgs{}) }
func (t *UpdateCharacterTool) ExposeToLLM() bool           { return true }
func (t *UpdateCharacterTool) NewArgs() any                { return &UpdateCharacterArgs{} }

func (t *UpdateCharacterTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*UpdateCharacterArgs)

	if a.Name == "" && a.Description == "" && a.Personality == "" && a.Abilities == "" {
		return &ToolResult{Success: false, Error: "Требуется указать хотя бы одно поле для изменения"}, nil
	}

	var ch character.Character
	if err := tc.DB.WithContext(ctx).Where("id = ? AND novel_id = ?", a.CharacterID, tc.NovelID).First(&ch).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("Персонаж %d не найден", a.CharacterID)}, nil
		}
		return nil, fmt.Errorf("query character: %w", err)
	}

	if err := json.Unmarshal(tc.RawArgs, &ch); err != nil {
		return &ToolResult{Success: false, Error: "Неверный формат параметров: " + err.Error()}, nil
	}

	if err := tc.DB.WithContext(ctx).Save(&ch).Error; err != nil {
		return nil, fmt.Errorf("save character: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"id": ch.ID},
	}, nil
}

// ── update_character_relationship ─────────────────────

// UpdateCharacterRelationshipArgs 是 update_character_relationship 的参数。
type UpdateCharacterRelationshipArgs struct {
	RelationID        int64  `json:"relation_id"         jsonschema:"description=Provide this ID when editing an existing relation to modify the description directly"`
	SourceCharacterID int64  `json:"source_character_id" jsonschema:"description=Source character ID when creating a new relation. The old relation automatically becomes historical"`
	TargetCharacterID int64  `json:"target_character_id" jsonschema:"description=Target character ID when creating a new relation. The old relation automatically becomes historical"`
	RelationDescribe  string `json:"relation_describe"   jsonschema:"description=Free-text description of the relation, e.g. 'master-apprentice, quietly rivals each other'. A detailed description, not a simple category word. Can be omitted when editing an existing relation"`
	Description       string `json:"description"         jsonschema:"description=Detailed description of the current relation stage"`
	ChapterID         *int64 `json:"chapter_id"          jsonschema:"description=Stable chapter ID where this relation was established/changed" validate:"omitempty,min=1"`
}

// UpdateCharacterRelationshipTool 创建或更新角色关系。
type UpdateCharacterRelationshipTool struct{}

func (t *UpdateCharacterRelationshipTool) Name() string { return "update_character_relationship" }
func (t *UpdateCharacterRelationshipTool) Description() string {
	return "Update a character relation. Two usages (mutually exclusive, cannot be passed together): " +
		"1) Edit an existing relation — pass only relation_id to change the description wording; do not pass source_character_id/target_character_id. " +
		"2) Create or evolve a relation — pass only source_character_id + target_character_id (do not pass relation_id). If the two characters have no prior relation, this creates a new one; if a relation already exists, the old one is automatically archived as history and the new one is set as current. " +
		"Relations are directional; to describe the relationship from both sides, call once for each direction. " +
		"relation_describe uses natural language, e.g. 'master-apprentice but quietly watchful of each other'; do not use simple enum words."
}
func (t *UpdateCharacterRelationshipTool) Category() ToolCategory { return CategoryWritingAssistant }

func (t *UpdateCharacterRelationshipTool) JSONSchema() json.RawMessage {
	return SchemaOf(UpdateCharacterRelationshipArgs{})
}
func (t *UpdateCharacterRelationshipTool) ExposeToLLM() bool { return true }
func (t *UpdateCharacterRelationshipTool) NewArgs() any      { return &UpdateCharacterRelationshipArgs{} }

func (t *UpdateCharacterRelationshipTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*UpdateCharacterRelationshipArgs)

	// 从原始参数判断字段是否存在（不能用结构体零值，否则无法区分"省略"和"显式 0"，
	// LLM 显式传 0 会绕过 ==0 判断进入 editRelation，Unmarshal 把 source/target 覆盖为 0）
	var m map[string]json.RawMessage
	if err := json.Unmarshal(tc.RawArgs, &m); err != nil {
		return &ToolResult{Success: false, Error: "Неверный формат параметров: " + err.Error()}, nil
	}
	_, hasRelationID := m["relation_id"]
	_, hasSource := m["source_character_id"]
	_, hasTarget := m["target_character_id"]
	chapterIDs := make([]int64, 0, 1)
	if a.ChapterID != nil {
		chapterIDs = append(chapterIDs, *a.ChapterID)
	}
	if result, err := ensureChapterIDsInNovel(ctx, tc, chapterIDs); err != nil || result != nil {
		return result, err
	}

	switch {
	case hasRelationID && !hasSource && !hasTarget:
		return t.editRelation(ctx, a, tc)
	case !hasRelationID && hasSource && hasTarget:
		return t.evolveRelation(ctx, a, tc)
	default:
		return &ToolResult{
			Success: false,
			Error:   "Недопустимая комбинация параметров: для редактирования отношения передавайте только relation_id, для эволюции — только source_character_id + target_character_id; они взаимоисключающие, не передавайте оба сразу",
		}, nil
	}
}

func (t *UpdateCharacterRelationshipTool) editRelation(ctx context.Context, a *UpdateCharacterRelationshipArgs, tc ToolContext) (*ToolResult, error) {
	var rel character.CharacterRelation
	if err := tc.DB.WithContext(ctx).First(&rel, a.RelationID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("Отношение %d не найдено", a.RelationID)}, nil
		}
		return nil, fmt.Errorf("query relation: %w", err)
	}
	if rel.NovelID != tc.NovelID {
		return &ToolResult{Success: false, Error: fmt.Sprintf("Отношение %d не принадлежит текущему роману", a.RelationID)}, nil
	}

	if err := json.Unmarshal(tc.RawArgs, &rel); err != nil {
		return &ToolResult{Success: false, Error: "Неверный формат параметров: " + err.Error()}, nil
	}

	if err := tc.DB.WithContext(ctx).Save(&rel).Error; err != nil {
		return nil, fmt.Errorf("save relation: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"id": rel.ID, "action": "edit"},
	}, nil
}

func (t *UpdateCharacterRelationshipTool) evolveRelation(ctx context.Context, a *UpdateCharacterRelationshipArgs, tc ToolContext) (*ToolResult, error) {
	if a.SourceCharacterID == a.TargetCharacterID {
		return &ToolResult{Success: false, Error: "source и target не могут быть одним и тем же персонажем"}, nil
	}
	if a.RelationDescribe == "" {
		return &ToolResult{Success: false, Error: "При эволюции отношения relation_describe обязателен"}, nil
	}

	// 校验两个角色存在且属于当前小说
	store := character.NewStore(tc.DB, tc.LoggerOrDefault())
	chars, err := store.GetByIDs(ctx, []int64{a.SourceCharacterID, a.TargetCharacterID})
	if err != nil {
		return nil, fmt.Errorf("query characters: %w", err)
	}
	found := make(map[int64]bool, len(chars))
	for _, ch := range chars {
		found[ch.ID] = true
	}
	if !found[a.SourceCharacterID] {
		return &ToolResult{Success: false, Error: fmt.Sprintf("Персонаж %d не найден", a.SourceCharacterID)}, nil
	}
	if !found[a.TargetCharacterID] {
		return &ToolResult{Success: false, Error: fmt.Sprintf("Персонаж %d не найден", a.TargetCharacterID)}, nil
	}

	var newRel character.CharacterRelation
	action := "create" // 默认全新关系；查到同向旧关系时改为 "evolve"
	err = tc.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 将同向旧关系标记为非当前（append-only），逐行 Save 以触发操作日志回调
		var oldRels []character.CharacterRelation
		if err := tx.Where("source_character_id = ? AND target_character_id = ? AND is_current = ?",
			a.SourceCharacterID, a.TargetCharacterID, true).
			Find(&oldRels).Error; err != nil {
			return fmt.Errorf("query old relations: %w", err)
		}
		if len(oldRels) > 0 {
			action = "evolve"
		}
		for i := range oldRels {
			oldRels[i].IsCurrent = false
			if err := tx.Save(&oldRels[i]).Error; err != nil {
				return fmt.Errorf("deactivate old relation: %w", err)
			}
		}

		newRel = character.CharacterRelation{
			NovelID:           tc.NovelID,
			SourceCharacterID: a.SourceCharacterID,
			TargetCharacterID: a.TargetCharacterID,
			RelationDescribe:  a.RelationDescribe,
			Description:       a.Description,
			ChapterID:         a.ChapterID,
			IsCurrent:         true,
		}
		if err := tx.Create(&newRel).Error; err != nil {
			return fmt.Errorf("create relation: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"id": newRel.ID, "action": action},
	}, nil
}

// ── 格式化工具 ──────────────────────────────────────────

func parseJSONField(raw string) any {
	if raw == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw
	}
	return v
}

// ── 注册 ──────────────────────────────────────────────

// RegisterCharacterTools 注册角色管理类工具。
func RegisterCharacterTools(r *Registry) {
	r.Register(&GetCharactersTool{})
	r.Register(&GetCharacterRelationsTool{})
	r.Register(&CreateCharacterTool{})
	r.Register(&UpdateCharacterTool{})
	r.Register(&UpdateCharacterRelationshipTool{})
}
