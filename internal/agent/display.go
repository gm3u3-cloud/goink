package agent

import (
	"context"
	"fmt"

	"github.com/sigpanic/goink/internal/git"
	"github.com/sigpanic/goink/internal/mcp_tools"
)

// toolDisplayNames 工具名 → 中文展示名称。
var toolDisplayNames = map[string]string{
	"get_volumes":                     "Список томов",
	"place_volume":                    "Создать или переместить том",
	"update_volume":                   "Переименовать том",
	"get_chapter_list":                "Список глав",
	"search_story_memory":             "Поиск в памяти истории",
	"get_timeline":                    "Хронология",
	"create_timeline_entry":           "Добавить запись хронологии",
	"update_timeline_entry":           "Изменить запись хронологии",
	"update_chapter_plan":             "Обновить план главы",
	"get_locations":                   "Список локаций",
	"create_location":                 "Создать локацию",
	"update_location":                 "Изменить локацию",
	"create_location_relation":        "Создать связь локаций",
	"update_location_relation":        "Изменить связь локаций",
	"get_characters":                  "Список персонажей",
	"get_character_relations":         "Связи персонажей",
	"create_character":                "Создать персонажа",
	"update_character":                "Изменить персонажа",
	"update_character_relationship":   "Изменить связь персонажей",
	"run_subagent":                    "Запустить подзадачу AI",
	"run_mcp":                         "Вызов инструмента по запросу",
	"search_mcp":                      "Поиск инструментов",
	"get_reader_perspective":          "Восприятие читателя",
	"create_reader_perspective_entry": "Добавить восприятие читателя",
	"update_reader_perspective_entry": "Изменить восприятие читателя",
	"get_story_arcs":                  "Дуги сюжета",
	"create_story_arc":                "Создать дугу сюжета",
	"update_story_arc":                "Изменить дугу сюжета",
	"create_arc_node":                 "Создать узел дуги",
	"update_arc_node":                 "Изменить узел дуги",
	"upsert_preference":               "Управление предпочтениями",
	"upsert_setting":                  "Управление настройками романа",
	"delete_record":                   "Удалить запись",
	"edit":                            "Редактирование файла",
	"read":                            "Чтение файла",
	"web_search":                      "Поиск в интернете",
	"web_fetch":                       "Загрузка веб-страницы",
}

// toolActivityKinds 工具名 → 前端展示类别。
var toolActivityKinds = map[string]string{
	"get_volumes":                     "browse",
	"place_volume":                    "edit",
	"update_volume":                   "edit",
	"get_chapter_list":                "browse",
	"search_story_memory":             "memory",
	"search_mcp":                      "browse",
	"get_timeline":                    "view",
	"create_timeline_entry":           "write",
	"update_timeline_entry":           "edit",
	"update_chapter_plan":             "edit",
	"get_locations":                   "view",
	"create_location":                 "create",
	"update_location":                 "edit",
	"create_location_relation":        "create",
	"update_location_relation":        "edit",
	"get_characters":                  "view",
	"get_character_relations":         "view",
	"create_character":                "create",
	"update_character":                "edit",
	"update_character_relationship":   "edit",
	"run_subagent":                    "plan",
	"get_reader_perspective":          "view",
	"create_reader_perspective_entry": "write",
	"update_reader_perspective_entry": "edit",
	"get_story_arcs":                  "view",
	"create_story_arc":                "create",
	"update_story_arc":                "edit",
	"create_arc_node":                 "create",
	"update_arc_node":                 "edit",
	"upsert_preference":               "edit",
	"upsert_setting":                  "edit",
	"delete_record":                   "delete",
	"edit":                            "write",
	"read":                            "view",
	"web_search":                      "browse",
	"web_fetch":                       "view",
}

// chapterTools 需要查章节标题的工具集。
var chapterTools = map[string]bool{
	"edit": true,
	"read": true,
}

// deleteRecordTableLabels 把 delete_record 的 args.table 映射到中文展示前缀。
// 用于 buildDisplay 中细化 "删除记录" 为 "删除角色"/"删除地点" 等。
var deleteRecordTableLabels = map[string]string{
	"character":                "Удалить персонажа",
	"character_relation":       "Удалить связь персонажей",
	"location":                 "Удалить локацию",
	"location_relation":        "Удалить связь локаций",
	"timeline_entry":           "Удалить запись хронологии",
	"story_arc":                "Удалить дугу сюжета",
	"arc_node":                 "Удалить узел дуги",
	"reader_perspective_entry": "Удалить восприятие читателя",
	"preference":               "Удалить предпочтение",
}

// resultDataMergeTools 是允许把 result.Data 合并到 AgentEvent.Metadata 的工具白名单。
// 前端通过 event.metadata 拿到这些字段用于完成态富文本渲染。
var resultDataMergeTools = map[string]bool{
	"web_search":    true,
	"web_fetch":     true,
	"delete_record": true,
}

// resultFieldTools 是允许在 toolDisplays 数组里输出 result 字段的工具白名单。
// 用于历史回看（rebuildTurns 从 messages 重建时前端拿到 td.result）。
var resultFieldTools = map[string]bool{
	"web_search":    true,
	"web_fetch":     true,
	"delete_record": true,
}

// buildDisplay 根据 tool_name + args + phase 生成前端展示文本。
// executing 阶段加 "正在" 前缀，completed/failed/cancelled 去掉。
// chapter 工具通过 novelID + chapter_id 查 DB 获取章节标题和实时阅读编号。
func (a *Agent) buildDisplay(ctx context.Context, name string, args map[string]any, phase mcp_tools.DisplayPhase, novelID int64) *mcp_tools.DisplayInfo {
	baseText := toolDisplayNames[name]
	if baseText == "" {
		baseText = name
	}
	activityKind := toolActivityKinds[name]
	if activityKind == "" {
		activityKind = "general"
	}

	var metadata map[string]any

	if name == "run_mcp" {
		if target, ok := args["tool_name"].(string); ok && target != "" {
			baseText = "Вызов " + target
			if label := toolDisplayNames[target]; label != "" {
				baseText = label
			}
			if kind := toolActivityKinds[target]; kind != "" {
				activityKind = kind
			}
			metadata = map[string]any{"tool_name": target}
		}
	}

	// run_subagent：根据 agent_type 定制展示文本
	if name == "run_subagent" {
		if at, ok := args["agent_type"].(string); ok {
			switch at {
			case "memory":
				baseText = "Изучение памяти истории"
			case "review":
				baseText = "Проверка содержания главы"
			}
		}
		metadata = map[string]any{"agent_type": args["agent_type"]}
	}

	// chapter 工具：查 DB 取章节标题
	if chapterTools[name] {
		if chapterID, ok := chapterID(args); ok {
			label := a.lookupChapterBrief(ctx, novelID, chapterID)
			if isOutlinePath(args) {
				label += "план"
			}
			switch name {
			case "edit":
				baseText = "Редактирование " + label
			case "read":
				baseText = "Просмотр " + label
			}
		}

		// rw 工具的 goink.md 路径特殊处理
		if path, ok := args["path"].(string); ok && path == "goink.md" {
			switch name {
			case "edit":
				baseText = "Редактирование состояния истории"
			case "read":
				baseText = "Просмотр состояния истории"
			}
		}

	}

	// delete_record：根据 args.table 细化展示文本（如 "删除角色" / "删除地点关系"）
	if name == "delete_record" {
		if table, ok := args["table"].(string); ok {
			if label, ok := deleteRecordTableLabels[table]; ok {
				baseText = label
			}
		}
	}

	// executing 阶段加 "正在" 前缀
	isActive := phase == mcp_tools.PhaseExecuting || phase == mcp_tools.PhaseSelected
	if isActive {
		baseText = "Идёт " + baseText
	}

	return &mcp_tools.DisplayInfo{
		DisplayText:  baseText,
		ActivityKind: activityKind,
		Metadata:     metadata,
	}
}

func chapterID(args map[string]any) (int64, bool) {
	if args == nil {
		return 0, false
	}
	if v, ok := args["chapter_id"]; ok {
		switch n := v.(type) {
		case float64:
			if n > 0 && n == float64(int64(n)) {
				return int64(n), true
			}
			return 0, false
		case int:
			if n > 0 {
				return int64(n), true
			}
			return 0, false
		case int64:
			if n > 0 {
				return n, true
			}
			return 0, false
		}
	}
	// rw 工具使用 path 参数，如 "chapters/id_42.md" 或 "outlines/7/id_42.md"。
	if path, ok := args["path"].(string); ok {
		ref, ok := git.ParseChapterLikePath(path)
		return ref.ID, ok && !ref.IsNew && ref.ID > 0
	}
	return 0, false
}

func isOutlinePath(args map[string]any) bool {
	path, ok := args["path"].(string)
	if !ok {
		return false
	}
	ref, ok := git.ParseChapterLikePath(path)
	return ok && !ref.IsNew && ref.IsOutline
}

func (a *Agent) lookupChapterBrief(ctx context.Context, novelID, chapterID int64) string {
	ch, err := a.chapterStore.GetByID(ctx, nil, novelID, chapterID)
	if err != nil {
		return fmt.Sprintf("Глава (ID: %d)", chapterID)
	}
	if ch.Title == "" {
		return fmt.Sprintf("Глава %d", ch.ReadingNumber)
	}
	return fmt.Sprintf("Глава %d %s", ch.ReadingNumber, ch.Title)
}

func buildToolDisplay(toolOutputs []toolOutput) []map[string]any {
	toolDisplays := make([]map[string]any, 0, len(toolOutputs))
	for _, to := range toolOutputs {
		phase := "completed"
		if !to.result.Success {
			phase = "failed"
		}
		entry := map[string]any{
			"tool_id":       to.id,
			"tool_name":     to.name,
			"display_text":  to.displayText,
			"activity_kind": to.activityKind,
			"phase":         phase,
		}
		if resultFieldTools[to.name] && to.result != nil && to.result.Success && to.result.Data != nil {
			entry["result"] = to.result.Data
		}
		// 失败时持久化 error，供前端历史回放显示
		if to.result != nil && !to.result.Success && to.result.Error != "" {
			entry["error"] = to.result.Error
		}
		toolDisplays = append(toolDisplays, entry)
	}
	return toolDisplays
}
