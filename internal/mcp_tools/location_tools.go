package mcp_tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/sigpanic/goink/internal/activity"
	"github.com/sigpanic/goink/internal/location"
	"github.com/sigpanic/goink/internal/storage"
)

// ── get_locations ──────────────────────────────────────

// GetLocationsArgs 是 get_locations 的参数。
type GetLocationsArgs struct {
	Mode         string `json:"mode" jsonschema:"required,description=Query mode: list=listing, detail=details, network=full graph,enum=list,enum=detail,enum=network,default=list" validate:"required,oneof=list detail network"`
	LocationID   int64  `json:"location_id" jsonschema:"description=Location ID (required for detail mode)"                                                                    validate:"omitempty,min=1"`
	LocationType string `json:"location_type" jsonschema:"description=Filter by type (optional for list mode)"`
	Search       string `json:"search" jsonschema:"description=Search by name (optional for list mode)"`
	PageArgs            // 嵌入分页参数
}

// GetLocationsTool 获取地点信息，支持三种模式。
type GetLocationsTool struct{}

func (t *GetLocationsTool) Name() string { return "get_locations" }
func (t *GetLocationsTool) Description() string {
	return "Get location information for the current novel. Three modes are supported:\n" +
		"- list: paginated listing, with search by type and name\n" +
		"- detail: location details, including the child location list and internal connectivity\n" +
		"- network: big map — only shows connectivity between root nodes, for macro spatial awareness"
}
func (t *GetLocationsTool) Category() ToolCategory { return CategoryNovelManagement }

func (t *GetLocationsTool) JSONSchema() json.RawMessage { return SchemaOf(GetLocationsArgs{}) }
func (t *GetLocationsTool) ExposeToLLM() bool           { return true }
func (t *GetLocationsTool) NewArgs() any                { return &GetLocationsArgs{} }

func (t *GetLocationsTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*GetLocationsArgs)
	a.NormalizePage()

	store := location.NewStore(tc.DB, tc.LoggerOrDefault())

	switch a.Mode {
	case "detail":
		return t.executeDetail(ctx, a, tc, store)
	case "network":
		return t.executeNetwork(ctx, tc, store)
	default:
		return t.executeList(ctx, a, tc, store)
	}
}

func (t *GetLocationsTool) executeList(ctx context.Context, a *GetLocationsArgs, tc ToolContext, store *location.Store) (*ToolResult, error) {
	result, err := store.ListByNovel(ctx, tc.NovelID, location.ListByNovelOptions{
		PageParams:   storage.PageParams{Page: a.Page, Size: a.Size},
		LocationType: a.LocationType,
		Search:       a.Search,
		Order:        "updated_at DESC", // MCP 按最近编辑排序，截断时保留活跃地点
	})
	if err != nil {
		return nil, fmt.Errorf("list locations: %w", err)
	}

	items := make([]map[string]any, len(result.Items))
	for i, loc := range result.Items {
		desc := loc.Description
		if len([]rune(desc)) > 100 {
			desc = string([]rune(desc)[:100]) + "..."
		}
		items[i] = map[string]any{
			"id":            loc.ID,
			"name":          loc.Name,
			"location_type": loc.LocationType,
			"description":   desc,
			"tags":          parseJSONField(loc.Tags),
		}
	}

	data := PageMeta(result)
	data["locations"] = items

	return &ToolResult{Success: true, Data: data}, nil
}

func (t *GetLocationsTool) executeDetail(ctx context.Context, a *GetLocationsArgs, tc ToolContext, store *location.Store) (*ToolResult, error) {
	if a.LocationID == 0 {
		return &ToolResult{Success: false, Error: "Для режима detail требуется location_id"}, nil
	}

	var loc location.Location
	if err := tc.DB.WithContext(ctx).Where("id = ? AND novel_id = ?", a.LocationID, tc.NovelID).First(&loc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("Местоположение %d не найдено", a.LocationID)}, nil
		}
		return nil, fmt.Errorf("query location: %w", err)
	}

	// 父地点名
	var parentName string
	if loc.ParentLocationID != nil {
		var parent location.Location
		if err := tc.DB.WithContext(ctx).First(&parent, *loc.ParentLocationID).Error; err == nil {
			parentName = parent.Name
		}
	}

	// 子地点
	children, err := store.GetChildren(ctx, loc.ID)
	if err != nil {
		return nil, fmt.Errorf("query children: %w", err)
	}

	// 连通关系：涉及自身 + 所有子地点的边
	var rels []location.LocationRelation
	ids := []int64{loc.ID}
	for _, ch := range children {
		ids = append(ids, ch.ID)
	}
	rels, err = store.ListRelationsInvolving(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("query relations: %w", err)
	}

	// 解析名称：收集关系中涉及的所有地点 ID
	nameSet := make(map[int64]bool)
	for _, rel := range rels {
		nameSet[rel.LocationA] = true
		nameSet[rel.LocationB] = true
	}
	var relIDs []int64
	for id := range nameSet {
		relIDs = append(relIDs, id)
	}
	nameMap := resolveLocationNames(ctx, tc.DB, relIDs)

	formatted := formatLocationDetail(loc, parentName, children, rels, nameMap)

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"content": formatted},
	}, nil
}

func (t *GetLocationsTool) executeNetwork(ctx context.Context, tc ToolContext, store *location.Store) (*ToolResult, error) {
	// 全部地点
	allResult, err := store.ListByNovel(ctx, tc.NovelID, location.ListByNovelOptions{
		PageParams: storage.PageParams{Page: 1, Size: -1},
		Order:      "name ASC", // network 需构建树，保持 name 升序
	})
	if err != nil {
		return nil, fmt.Errorf("list locations: %w", err)
	}
	allLocs := allResult.Items

	// 根节点
	var roots []location.Location
	childCounts := make(map[int64]int)
	for _, loc := range allLocs {
		if loc.ParentLocationID == nil {
			roots = append(roots, loc)
		} else {
			childCounts[*loc.ParentLocationID]++
		}
	}

	// 根节点 ID 集合
	rootIDs := make([]int64, len(roots))
	for i, r := range roots {
		rootIDs[i] = r.ID
	}

	// 涉及根节点的边
	allRels, err := store.ListRelationsInvolving(ctx, rootIDs)
	if err != nil {
		return nil, fmt.Errorf("query relations: %w", err)
	}

	// 只保留两端都是根节点的边
	rootSet := make(map[int64]bool, len(rootIDs))
	for _, id := range rootIDs {
		rootSet[id] = true
	}
	var rootRels []location.LocationRelation
	for _, rel := range allRels {
		if rootSet[rel.LocationA] && rootSet[rel.LocationB] {
			rootRels = append(rootRels, rel)
		}
	}

	formatted := formatLocationNetwork(roots, rootRels, childCounts)

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"content": formatted},
	}, nil
}

// ── create_location ────────────────────────────────────

// CreateLocationItem 是 create_location 的单条参数。
type CreateLocationItem struct {
	Name             string `json:"name" jsonschema:"required,description=Location name"             validate:"required"`
	LocationType     string `json:"location_type" jsonschema:"description=Location type, free text, e.g. 'forest', 'city', 'cave'"`
	Description      string `json:"description" jsonschema:"description=Description of the environment, atmosphere, features, etc."`
	DetailJSON       string `json:"detail_json" jsonschema:"description=JSON object in string form; structured info: climate, atmosphere, historical events, etc."`
	Tags             string `json:"tags" jsonschema:"description=JSON array in string form; free tags, e.g. [\"dangerous\",\"mysterious\"]"`
	ParentLocationID *int64 `json:"parent_location_id" jsonschema:"description=Parent location ID, used to build the hierarchy tree"`
}

// CreateLocationArgs 是 create_location 的参数。
type CreateLocationArgs struct {
	Locations []CreateLocationItem `json:"locations" jsonschema:"required,description=List of locations to create (1-10)" validate:"required,min=1,max=10,dive"`
}

// CreateLocationTool 创建新地点。
type CreateLocationTool struct{}

func (t *CreateLocationTool) Name() string { return "create_location" }
func (t *CreateLocationTool) Description() string {
	return "Batch-create locations (1-10). Guarantees atomicity; on failure, returns the specific entry reason. " +
		"name is required, location_type is free text. " +
		"parent_location_id can attach to an existing hierarchy tree, e.g. set it to the ID of 'palace' when creating 'great hall'."
}
func (t *CreateLocationTool) Category() ToolCategory { return CategoryNovelManagement }

func (t *CreateLocationTool) JSONSchema() json.RawMessage { return SchemaOf(CreateLocationArgs{}) }
func (t *CreateLocationTool) ExposeToLLM() bool           { return true }
func (t *CreateLocationTool) NewArgs() any                { return &CreateLocationArgs{} }

func (t *CreateLocationTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*CreateLocationArgs)

	// 预校验：批量验证父地点存在性
	parentIDSet := make(map[int64]bool)
	for _, item := range a.Locations {
		if item.ParentLocationID != nil {
			parentIDSet[*item.ParentLocationID] = true
		}
	}
	if len(parentIDSet) > 0 {
		parentIDs := make([]int64, 0, len(parentIDSet))
		for id := range parentIDSet {
			parentIDs = append(parentIDs, id)
		}
		var count int64
		if err := tc.DB.WithContext(ctx).Model(&location.Location{}).
			Where("id IN ? AND novel_id = ?", parentIDs, tc.NovelID).
			Count(&count).Error; err != nil {
			return nil, fmt.Errorf("verify parent locations: %w", err)
		}
		if int(count) != len(parentIDs) {
			return &ToolResult{Success: false, Error: "Родительское местоположение не найдено или не принадлежит текущему роману"}, nil
		}
	}

	var ids []int64
	var failedName string
	var failedErr error
	err := tc.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range a.Locations {
			loc := location.Location{
				NovelID:          tc.NovelID,
				Name:             item.Name,
				LocationType:     item.LocationType,
				Description:      item.Description,
				DetailJSON:       item.DetailJSON,
				Tags:             item.Tags,
				ParentLocationID: item.ParentLocationID,
			}
			if err := tx.Create(&loc).Error; err != nil {
				failedName = item.Name
				failedErr = err
				return err
			}
			ids = append(ids, loc.ID)
		}
		return nil
	})
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Не удалось создать местоположение [%s]: %s", failedName, failedErr),
		}, nil
	}

	activity.NewStore(tc.DB, tc.LoggerOrDefault()).AddActivity(ctx, activity.ActivityDelta{LocationsCreated: int64(len(ids))})

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"ids": ids, "count": len(ids)},
	}, nil
}

// ── update_location ────────────────────────────────────

// UpdateLocationArgs 是 update_location 的参数。
type UpdateLocationArgs struct {
	LocationID       int64  `json:"location_id" jsonschema:"required,description=Location ID"               validate:"required,min=1"`
	Name             string `json:"name" jsonschema:"description=New name"`
	LocationType     string `json:"location_type" jsonschema:"description=New type"`
	Description      string `json:"description" jsonschema:"description=New description"`
	DetailJSON       string `json:"detail_json" jsonschema:"description=New structured info, JSON in string form (fully replaces the old one)"`
	Tags             string `json:"tags" jsonschema:"description=New tags, JSON array in string form (fully replaces the old one)"`
	ParentLocationID *int64 `json:"parent_location_id" jsonschema:"description=New parent location ID (omit to keep unchanged, pass null to become a root node)"`
}

// UpdateLocationTool 更新地点字段。
type UpdateLocationTool struct{}

func (t *UpdateLocationTool) Name() string { return "update_location" }
func (t *UpdateLocationTool) Description() string {
	return "Update the settings of an existing location. Only pass the fields you want to change; unpassed fields stay unchanged. " +
		"Passing null for parent_location_id removes the location from the tree and makes it a root node."
}
func (t *UpdateLocationTool) Category() ToolCategory { return CategoryNovelManagement }

func (t *UpdateLocationTool) JSONSchema() json.RawMessage { return SchemaOf(UpdateLocationArgs{}) }
func (t *UpdateLocationTool) ExposeToLLM() bool           { return true }
func (t *UpdateLocationTool) NewArgs() any                { return &UpdateLocationArgs{} }

func (t *UpdateLocationTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*UpdateLocationArgs)

	// ParentLocationID 为 nil 有两种情况：LLM 没传 vs 传了 null。
	// 前者不应视为"有修改"，后者应（清除父节点）。检查 RawArgs 区分。
	var raw map[string]any
	if err := json.Unmarshal(tc.RawArgs, &raw); err != nil {
		return &ToolResult{Success: false, Error: "Неверный формат параметров: " + err.Error()}, nil
	}
	_, hasParent := raw["parent_location_id"]

	if a.Name == "" && a.LocationType == "" && a.Description == "" && a.DetailJSON == "" && a.Tags == "" && !hasParent {
		return &ToolResult{Success: false, Error: "Требуется указать хотя бы одно поле для изменения"}, nil
	}

	var loc location.Location
	if err := tc.DB.WithContext(ctx).Where("id = ? AND novel_id = ?", a.LocationID, tc.NovelID).First(&loc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("Местоположение %d не найдено", a.LocationID)}, nil
		}
		return nil, fmt.Errorf("query location: %w", err)
	}

	// 校验父地点存在性
	if hasParent && a.ParentLocationID != nil {
		var parentCount int64
		if err := tc.DB.WithContext(ctx).Model(&location.Location{}).
			Where("id = ? AND novel_id = ?", *a.ParentLocationID, tc.NovelID).
			Count(&parentCount).Error; err != nil {
			return nil, fmt.Errorf("verify parent location: %w", err)
		}
		if parentCount == 0 {
			return &ToolResult{Success: false, Error: "Родительское местоположение не найдено или не принадлежит текущему роману"}, nil
		}
	}

	if err := json.Unmarshal(tc.RawArgs, &loc); err != nil {
		return &ToolResult{Success: false, Error: "Неверный формат параметров: " + err.Error()}, nil
	}

	if err := tc.DB.WithContext(ctx).Save(&loc).Error; err != nil {
		return nil, fmt.Errorf("save location: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data:    map[string]any{"id": loc.ID},
	}, nil
}

// ── create_location_relation ────────────────────────────

// CreateLocationRelationItem 是 create_location_relation 的单条参数。
type CreateLocationRelationItem struct {
	LocationA    int64  `json:"location_a_id" jsonschema:"required,description=ID of location A"               validate:"required,min=1"`
	LocationB    int64  `json:"location_b_id" jsonschema:"required,description=ID of location B"               validate:"required,min=1"`
	RelationType string `json:"relation_type" jsonschema:"required,description=Spatial relation description, e.g. 'adjacent', 'connected by a mountain road'" validate:"required"`
	Description  string `json:"description" jsonschema:"description=Supplementary details"`
}

// CreateLocationRelationArgs 是 create_location_relation 的参数。
type CreateLocationRelationArgs struct {
	Relations []CreateLocationRelationItem `json:"relations" jsonschema:"required,description=List of relations to create (1-10)" validate:"required,min=1,max=10,dive"`
}

// CreateLocationRelationTool 创建地点之间的无向空间关系边。
type CreateLocationRelationTool struct{}

func (t *CreateLocationRelationTool) Name() string { return "create_location_relation" }
func (t *CreateLocationRelationTool) Description() string {
	return "Batch-create spatial connectivity relations between locations (1-10). Guarantees atomicity; on failure, returns the specific entry reason. " +
		"Relations are undirected edges (A-B is equivalent to B-A); if the edge already exists, an error is returned — use update_location_relation to modify an existing edge."
}
func (t *CreateLocationRelationTool) Category() ToolCategory { return CategoryWritingAssistant }

func (t *CreateLocationRelationTool) JSONSchema() json.RawMessage {
	return SchemaOf(CreateLocationRelationArgs{})
}
func (t *CreateLocationRelationTool) ExposeToLLM() bool { return true }
func (t *CreateLocationRelationTool) NewArgs() any      { return &CreateLocationRelationArgs{} }

func (t *CreateLocationRelationTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*CreateLocationRelationArgs)

	// 预校验：自环检查 + 归一化
	for i := range a.Relations {
		if a.Relations[i].LocationA == a.Relations[i].LocationB {
			return &ToolResult{Success: false, Error: "Нельзя создать пространственное отношение местоположения с самим собой"}, nil
		}
		if a.Relations[i].LocationA > a.Relations[i].LocationB {
			a.Relations[i].LocationA, a.Relations[i].LocationB = a.Relations[i].LocationB, a.Relations[i].LocationA
		}
	}

	// 预校验：收集所有涉及的地点 ID，批量校验存在性
	idSet := make(map[int64]bool)
	for _, item := range a.Relations {
		idSet[item.LocationA] = true
		idSet[item.LocationB] = true
	}
	allIDs := make([]int64, 0, len(idSet))
	for id := range idSet {
		allIDs = append(allIDs, id)
	}
	var count int64
	if err := tc.DB.WithContext(ctx).Model(&location.Location{}).
		Where("id IN ? AND novel_id = ?", allIDs, tc.NovelID).
		Count(&count).Error; err != nil {
		return nil, fmt.Errorf("verify locations: %w", err)
	}
	if int(count) != len(allIDs) {
		return &ToolResult{Success: false, Error: "Некоторые местоположения не найдены или не принадлежат текущему роману"}, nil
	}

	// 预校验：批量检查是否已存在关系边
	type pair struct{ a, b int64 }
	seen := make(map[pair]bool)
	var pairs []pair
	for _, item := range a.Relations {
		p := pair{item.LocationA, item.LocationB}
		if seen[p] {
			return &ToolResult{Success: false, Error: fmt.Sprintf("В параметрах есть дублирующееся отношение: местоположение %d и %d", item.LocationA, item.LocationB)}, nil
		}
		seen[p] = true
		pairs = append(pairs, p)
	}
	var existing []location.LocationRelation
	for _, p := range pairs {
		var rel location.LocationRelation
		err := tc.DB.WithContext(ctx).
			Where("location_a = ? AND location_b = ? AND novel_id = ?", p.a, p.b, tc.NovelID).
			First(&rel).Error
		if err == nil {
			existing = append(existing, rel)
		} else if err != gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("check existing relation: %w", err)
		}
	}
	if len(existing) > 0 {
		return &ToolResult{Success: false, Error: fmt.Sprintf("Между местоположениями %d и %d уже есть ребро отношения, используйте update_location_relation для изменения", existing[0].LocationA, existing[0].LocationB)}, nil
	}

	var ids []int64
	var failedIdx int
	var failedErr error
	err := tc.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, item := range a.Relations {
			rel := location.LocationRelation{
				NovelID:      tc.NovelID,
				LocationA:    item.LocationA,
				LocationB:    item.LocationB,
				RelationType: item.RelationType,
				Description:  item.Description,
			}
			if err := tx.Create(&rel).Error; err != nil {
				failedIdx = i + 1
				failedErr = err
				return err
			}
			ids = append(ids, rel.ID)
		}
		return nil
	})
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Не удалось создать отношение %d: %s", failedIdx, failedErr),
		}, nil
	}

	return &ToolResult{Success: true, Data: map[string]any{"ids": ids, "count": len(ids)}}, nil
}

// ── update_location_relation ────────────────────────────

// UpdateLocationRelationArgs 是 update_location_relation 的参数。
type UpdateLocationRelationArgs struct {
	RelationID   int64  `json:"relation_id" jsonschema:"required,description=Relation edge ID" validate:"required,min=1"`
	RelationType string `json:"relation_type" jsonschema:"description=New spatial relation description"`
	Description  string `json:"description" jsonschema:"description=New supplementary details"`
}

// UpdateLocationRelationTool 更新地点空间关系边（PATCH 语义）。
type UpdateLocationRelationTool struct{}

func (t *UpdateLocationRelationTool) Name() string { return "update_location_relation" }
func (t *UpdateLocationRelationTool) Description() string {
	return "Update an existing location spatial connectivity relation. Only pass the fields you want to change; unpassed fields stay unchanged. " +
		"Locate the edge to modify via relation_id."
}
func (t *UpdateLocationRelationTool) Category() ToolCategory { return CategoryWritingAssistant }

func (t *UpdateLocationRelationTool) JSONSchema() json.RawMessage {
	return SchemaOf(UpdateLocationRelationArgs{})
}
func (t *UpdateLocationRelationTool) ExposeToLLM() bool { return true }
func (t *UpdateLocationRelationTool) NewArgs() any      { return &UpdateLocationRelationArgs{} }

func (t *UpdateLocationRelationTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*UpdateLocationRelationArgs)

	if a.RelationType == "" && a.Description == "" {
		return &ToolResult{Success: false, Error: "Требуется указать хотя бы одно поле для изменения"}, nil
	}

	var rel location.LocationRelation
	if err := tc.DB.WithContext(ctx).
		Where("id = ? AND novel_id = ?", a.RelationID, tc.NovelID).
		First(&rel).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ToolResult{Success: false, Error: fmt.Sprintf("Ребро отношения %d не найдено", a.RelationID)}, nil
		}
		return nil, fmt.Errorf("query relation: %w", err)
	}

	if err := json.Unmarshal(tc.RawArgs, &rel); err != nil {
		return &ToolResult{Success: false, Error: "Неверный формат параметров: " + err.Error()}, nil
	}

	if err := tc.DB.WithContext(ctx).Save(&rel).Error; err != nil {
		return nil, fmt.Errorf("save relation: %w", err)
	}

	return &ToolResult{Success: true, Data: map[string]any{"id": rel.ID}}, nil
}

// ── 格式化 ──────────────────────────────────────────────

func formatLocationDetail(loc location.Location, parentName string, children []location.Location, rels []location.LocationRelation, nameMap map[int64]string) string {
	var parts []string

	// 标题
	parts = append(parts, fmt.Sprintf("### %s [location_id:%d]", loc.Name, loc.ID))

	// 基本信息
	if loc.LocationType != "" {
		parts = append(parts, fmt.Sprintf("- Type: %s", loc.LocationType))
	}
	if parentName != "" {
		parts = append(parts, fmt.Sprintf("- Parent location: %s", parentName))
	}
	if loc.Description != "" {
		parts = append(parts, fmt.Sprintf("- Description: %s", loc.Description))
	}
	if v := parseJSONField(loc.DetailJSON); v != nil {
		parts = append(parts, fmt.Sprintf("- Structured info: %v", v))
	}
	if v := parseJSONField(loc.Tags); v != nil {
		parts = append(parts, fmt.Sprintf("- Tags: %v", v))
	}

	// 子地点
	if len(children) > 0 {
		lines := []string{fmt.Sprintf("\n#### Child locations (%d)", len(children))}
		for _, ch := range children {
			lines = append(lines, fmt.Sprintf("- %s [location_id:%d]", ch.Name, ch.ID))
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}

	// 连通
	if len(rels) > 0 {
		lines := []string{"\n#### Connectivity"}
		for _, rel := range rels {
			line := fmt.Sprintf("- %s — %s [relation_id:%d]: %s",
				nameMap[rel.LocationA], nameMap[rel.LocationB], rel.ID, rel.RelationType)
			if rel.Description != "" {
				line += fmt.Sprintf(" (%s)", rel.Description)
			}
			lines = append(lines, line)
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}

	return strings.Join(parts, "\n")
}

func formatLocationNetwork(roots []location.Location, rootRels []location.LocationRelation, childCounts map[int64]int) string {
	var parts []string
	parts = append(parts, "### Big Map")

	// 摘要
	var rootDescs []string
	for _, r := range roots {
		n := childCounts[r.ID]
		if n > 0 {
			rootDescs = append(rootDescs, fmt.Sprintf("%s [location_id:%d] (with %d child locations)", r.Name, r.ID, n))
		} else {
			rootDescs = append(rootDescs, fmt.Sprintf("%s [location_id:%d]", r.Name, r.ID))
		}
	}
	parts = append(parts, fmt.Sprintf("\nTotal %d root nodes: %s.", len(roots), strings.Join(rootDescs, ", ")))

	// 根节点连通
	if len(rootRels) > 0 {
		nameMap := make(map[int64]string)
		for _, r := range roots {
			nameMap[r.ID] = r.Name
		}
		parts = append(parts, "")
		for _, rel := range rootRels {
			line := fmt.Sprintf("- %s — %s [relation_id:%d]: %s",
				nameMap[rel.LocationA], nameMap[rel.LocationB], rel.ID, rel.RelationType)
			if rel.Description != "" {
				line += fmt.Sprintf(" (%s)", rel.Description)
			}
			parts = append(parts, line)
		}
	}

	if len(rootRels) == 0 {
		parts = append(parts, "\nNo connectivity between root nodes yet.")
	}

	return strings.Join(parts, "\n")
}

// resolveLocationNames 批量解析地点名称。
func resolveLocationNames(ctx context.Context, db *gorm.DB, ids []int64) map[int64]string {
	if len(ids) == 0 {
		return nil
	}
	var locs []location.Location
	if err := db.WithContext(ctx).Where("id IN ?", ids).Find(&locs).Error; err != nil {
		return nil
	}
	m := make(map[int64]string, len(locs))
	for _, l := range locs {
		m[l.ID] = l.Name
	}
	return m
}

// ── 注册 ──────────────────────────────────────────────

// RegisterLocationTools 注册地点管理类工具。
func RegisterLocationTools(r *Registry) {
	r.Register(&GetLocationsTool{})
	r.Register(&CreateLocationTool{})
	r.Register(&UpdateLocationTool{})
	r.Register(&CreateLocationRelationTool{})
	r.Register(&UpdateLocationRelationTool{})
}
