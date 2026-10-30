package mcp_tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	wails "github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"

	"github.com/sigpanic/goink/internal/activity"
	"github.com/sigpanic/goink/internal/chapter"
	"github.com/sigpanic/goink/internal/git"
	"github.com/sigpanic/goink/internal/rag"
	"github.com/sigpanic/goink/internal/skill"
	"github.com/sigpanic/goink/internal/text"
	"github.com/sigpanic/goink/internal/volume"
	"github.com/sigpanic/goink/internal/writing"
)

// ── edit ──────────────────────────────────────────────────

// EditArgs 是 edit 工具的参数。
type EditArgs struct {
	Path       string `json:"path" jsonschema:"required,description=Path of the file to edit. Chapter body: chapters/id_{chapter_id}.md (primary form) or chapters/{volume_id}/id_{chapter_id}.md (equivalent lenient form); chapter outline: outlines/id_{chapter_id}.md or outlines/{volume_id}/id_{chapter_id}.md; new chapter: chapters/{volume_id}/new.md or chapters/new.md (without a volume, defaults to the last volume; if the book has no volumes, creates an unassigned chapter); new outline: outlines/{volume_id}/new.md or outlines/new.md (same rules as chapters, full_replace only); volume outline: volumes/id_{volume_id}.md; story state goink.md; novel-level skill skills/<name>.md; user-level skill ~/.goink/skills/<name>.md" validate:"required"`
	ChangeType string `json:"change_type" jsonschema:"required,enum=full_replace,enum=search_replace,enum=line_range_replace,description=Editing mode. full_replace: replace the entire file; search_replace: find and replace the specified text; line_range_replace: replace the specified line range" validate:"required,oneof=full_replace search_replace line_range_replace"`
	SearchText string `json:"search_text" jsonschema:"description=The original text fragment to find (required for search_replace). Copy it exactly from the file" validate:"omitempty"`
	NewContent string `json:"new_content" jsonschema:"description=New content. For full_replace: the complete full text (required; passing empty will error; to clear the entire file use line_range_replace(1, total_lines, \"\") instead, with total_lines obtained from the read result); for search_replace: the replacement text (passing empty deletes the matched text); for line_range_replace: the new content for that line range (passing empty deletes those lines)" validate:"omitempty"`
	ReplaceAll bool   `json:"replace_all" jsonschema:"description=Whether to replace all matches. Defaults to false (only replaces the first match)" validate:"omitempty"`
	StartLine  int    `json:"start_line" jsonschema:"description=Start line number, 1-based inclusive (required for line_range_replace); must be <= end_line" validate:"omitempty,min=1"`
	EndLine    int    `json:"end_line" jsonschema:"description=End line number, 1-based inclusive (required for line_range_replace)" validate:"omitempty,min=1"`
	Reason     string `json:"reason" jsonschema:"required,description=Required. The reason/intent for this change, for the author's approval reference" validate:"required"`
	Title      string `json:"title" jsonschema:"description=Chapter title. Optional when creating via new.md; when passed for an existing chapter/outline, overrides the original title (applies to both chapters/ and outlines/ paths)" validate:"omitempty"`
}

// EditTool 编辑文件（章节或故事状态），支持全文替换、查找替换、行范围替换。
// 修改在内存中完成后生成 git diff 提交审批，通过后写入文件。
type EditTool struct{}

func (t *EditTool) Name() string           { return "edit" }
func (t *EditTool) Description() string    { return editDescription }
func (t *EditTool) Category() ToolCategory { return CategoryWritingAssistant }

func (t *EditTool) JSONSchema() json.RawMessage { return SchemaOf(EditArgs{}) }
func (t *EditTool) ExposeToLLM() bool           { return true }
func (t *EditTool) NewArgs() any                { return &EditArgs{} }

func (t *EditTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*EditArgs)

	// 内置 skill 只读
	if strings.HasPrefix(a.Path, "/builtin/skills/") {
		return &ToolResult{Success: false, Error: "Встроенный skill только для чтения, не изменяется"}, nil
	}

	if volumeID, ok := git.ParseVolumePath(a.Path); ok {
		return t.editVolumeOutline(ctx, a, tc, volumeID)
	}

	// 章节/大纲路径走章节流程；其余（goink.md、技能文件）走通用文件流程
	if ref, ok := git.ParseChapterLikePath(a.Path); ok {
		return t.editChapterLike(ctx, a, tc, ref)
	}
	if !validPath(a.Path) {
		return &ToolResult{Success: false, Error: invalidPathHint}, nil
	}
	return t.editPlainFile(ctx, a, tc)
}

// invalidPathHint 是 edit/read 共用的非法路径提示。
const invalidPathHint = "Недопустимый путь к файлу. Текст главы chapters/id_{id}.md, конспект outlines/id_{id}.md, конспект тома volumes/id_{volume_id}.md, новые chapters/{volume_id}/new.md или chapters/new.md, outlines/{volume_id}/new.md или outlines/new.md, goink.md, skills/<name>.md, ~/.goink/skills/<name>.md"

// readFileForEdit 读取待编辑文件的当前内容。
// 文件不存在时 full_replace 视为从空文件创建，其余模式返回 os.ErrNotExist。
func readFileForEdit(novelID int64, path, changeType string) (string, error) {
	content, err := git.ReadFile(novelID, path)
	if err == nil {
		return content, nil
	}
	if errors.Is(err, os.ErrNotExist) && changeType == "full_replace" {
		return "", nil
	}
	return "", err
}

// fileReadError 将读取文件的已知业务错误转换为 ToolResult。
// handled=false 表示非业务错误，交由上层作为系统错误返回。
func fileReadError(path string, err error) (*ToolResult, bool) {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return &ToolResult{Success: false, Error: "Файл не найден: " + path}, true
	case errors.Is(err, git.ErrPathEscape):
		return &ToolResult{Success: false, Error: "Недопустимый путь: " + path}, true
	default:
		return nil, false
	}
}

// approvalPayload 构造审批事件负载。title 非空时附带展示（标题将被修改）。
func approvalPayload(path, current, proposed string, a *EditArgs) map[string]any {
	payload := map[string]any{
		"original":    current,
		"modified":    proposed,
		"path":        path,
		"change_type": a.ChangeType,
		"reason":      a.Reason,
	}
	if a.Title != "" {
		payload["title"] = a.Title
	}
	return payload
}

// requestApproval 阻塞等待用户审批。
// res 非 nil 表示审批被拒绝或操作被中断，调用方直接返回该结果；
// err 表示审批通道自身的系统错误。
func requestApproval(ctx context.Context, tc ToolContext, payload map[string]any) (feedback string, res *ToolResult, err error) {
	if tc.Approver == nil {
		return "", nil, nil
	}
	if tc.EmitApproval != nil {
		tc.EmitApproval(tc.ToolID, "file_edit", payload)
	}
	ap, err := tc.Approver.RequestApproval(ctx, tc.ToolID, payload)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return "", &ToolResult{Success: false, Error: "Операция прервана"}, nil
		}
		return "", nil, fmt.Errorf("approval: %w", err)
	}
	if !ap.Approved {
		info := "Your edit was rejected by the user"
		if ap.Feedback != "" {
			info += ". User feedback: " + ap.Feedback
		}
		return "", &ToolResult{
			Success: false,
			Error:   "Одобрение не пройдено",
			Data: map[string]any{
				"path":        payload["path"],
				"change_type": payload["change_type"],
				"approved":    false,
			},
			Inject: []InjectMessage{{Role: "user", Content: info}},
		}, nil
	}
	return ap.Feedback, nil, nil
}

// writeApproved 写入前的并发冲突检查 + 落盘 + file:changed 事件。
// res 非 nil 表示业务失败（并发冲突/路径非法），err 表示系统错误。
func writeApproved(ctx context.Context, tc ToolContext, path, current, proposed string) (res *ToolResult, err error) {
	if err := git.WriteFileIfUnchanged(tc.NovelID, path, current, proposed); err != nil {
		if errors.Is(err, git.ErrFileChanged) {
			return &ToolResult{Success: false, Error: "Файл был изменён, перезагрузите свежее содержимое и повторите"}, nil
		}
		if errors.Is(err, git.ErrPathEscape) {
			return &ToolResult{Success: false, Error: "Недопустимый путь: " + path}, nil
		}
		return nil, fmt.Errorf("запись файла: %w", err)
	}
	emitFileChanged(ctx, tc.NovelID, path)
	return nil, nil
}

// emitFileChanged 推送 file:changed 事件。wails runtime 在上下文缺失时
// 会 log.Fatalf 终止进程，因此无 wails 运行时上下文（如测试环境）时静默跳过。
func emitFileChanged(ctx context.Context, novelID int64, path string) {
	if ctx == nil || ctx.Value("events") == nil {
		return
	}
	wails.EventsEmit(ctx, "file:changed", map[string]any{
		"novel_id": novelID,
		"path":     path,
	})
}

func emitFileMetadataChanged(ctx context.Context, novelID int64, path string) {
	if ctx == nil || ctx.Value("events") == nil {
		return
	}
	wails.EventsEmit(ctx, "file:changed", map[string]any{
		"novel_id":      novelID,
		"path":          path,
		"metadata_only": true,
	})
}

// physicalRWPath 章节正文的物理扁平路径（isOutline=false 时为章节，true 时为大纲）。
func physicalRWPath(isOutline bool, id int64) string {
	if isOutline {
		return git.OutlinePath(id)
	}
	return git.ChapterPath(id)
}

// editPlainFile 编辑通用文件（goink.md、技能文件），无章节记录联动。
func (t *EditTool) editPlainFile(ctx context.Context, a *EditArgs, tc ToolContext) (*ToolResult, error) {
	current, err := readFileForEdit(tc.NovelID, a.Path, a.ChangeType)
	if err != nil {
		if res, handled := fileReadError(a.Path, err); handled {
			return res, nil
		}
		return nil, fmt.Errorf("чтение файла %s: %w", a.Path, err)
	}

	proposed, err := applyChange(a, current)
	if err != nil {
		return &ToolResult{Success: false, Error: fmt.Sprintf("Ошибка редактирования: %s", err.Error())}, nil
	}
	if proposed == current {
		return &ToolResult{Success: true, Data: map[string]any{"path": a.Path, "message": "Content unchanged, skipped"}}, nil
	}

	// skill 格式校验（审批前，格式不对直接返回 LLM 修正）
	if isSkillPath(a.Path) {
		if _, err := skill.ParseBytes([]byte(proposed), ""); err != nil {
			return &ToolResult{Success: false, Error: fmt.Sprintf("Ошибка формата skill: %s", err.Error())}, nil
		}
	}

	feedback, res, err := requestApproval(ctx, tc, approvalPayload(a.Path, current, proposed, a))
	if err != nil {
		return nil, err
	}
	if res != nil {
		return res, nil
	}

	if res, err := writeApproved(ctx, tc, a.Path, current, proposed); res != nil || err != nil {
		return res, err
	}

	var injects []InjectMessage
	if feedback != "" {
		injects = append(injects, InjectMessage{Role: "user", Content: "User approved and provided feedback: " + feedback})
	}
	data := map[string]any{
		"path":        a.Path,
		"change_type": a.ChangeType,
		"approved":    true,
	}
	if a.ChangeType == "line_range_replace" {
		afterEnd := a.StartLine + strings.Count(a.NewContent, "\n")
		data["before"] = linePreview(current, a.StartLine, a.EndLine)
		data["after"] = linePreview(proposed, a.StartLine, afterEnd)
	}
	return &ToolResult{Success: true, Data: data, Inject: injects}, nil
}

// editVolumeOutline 编辑已有卷的卷纲文件。文件可首次由 full_replace 创建，
// 但卷记录必须存在且属于当前小说，避免落盘孤儿卷纲。
func (t *EditTool) editVolumeOutline(ctx context.Context, a *EditArgs, tc ToolContext, volumeID int64) (*ToolResult, error) {
	if _, err := volume.NewStore(tc.DB, tc.LoggerOrDefault()).GetByID(ctx, nil, tc.NovelID, volumeID); err != nil {
		if errors.Is(err, volume.ErrNotFound) {
			return &ToolResult{Success: false, Error: err.Error()}, nil
		}
		return nil, fmt.Errorf("запрос тома: %w", err)
	}
	return t.editPlainFile(ctx, a, tc)
}

// editChapterLike 编辑章节正文或大纲（含 new.md 新建通道）。
//
// 路径与存储的关系：
//   - 物理文件永远按 id 扁平落盘（chapters/id_{id}.md / outlines/id_{id}.md），
//     两级路径 chapters/{vid}/id_{id}.md 仅为容错别名，读写时归一化为扁平路径
//   - 新建走 new.md 通道：先建章节记录拿 id，再写物理文件；写文件失败补偿删记录
func (t *EditTool) editChapterLike(ctx context.Context, a *EditArgs, tc ToolContext, ref git.ChapterPathRef) (*ToolResult, error) {
	physical := physicalRWPath(ref.IsOutline, ref.ID)
	chStore := chapter.NewStore(tc.DB, tc.LoggerOrDefault())

	var (
		ch       *chapter.Chapter
		volumeID *int64
	)
	if ref.IsNew {
		// 新建通道：只能 full_replace；卷可选——带卷校验归属，
		// 不带卷时默认最后一卷；无卷则归入未分卷组。
		if a.ChangeType != "full_replace" {
			return &ToolResult{Success: false, Error: "new.md поддерживает только full_replace (у нового файла нет исходного текста для поиска и замены)"}, nil
		}
		if ref.VolumeID != 0 {
			v, err := volume.NewStore(tc.DB, tc.LoggerOrDefault()).GetByID(ctx, nil, tc.NovelID, ref.VolumeID)
			if err != nil {
				if errors.Is(err, volume.ErrNotFound) {
					return &ToolResult{Success: false, Error: err.Error()}, nil
				}
				return nil, fmt.Errorf("запрос тома: %w", err)
			}
			volumeID = &v.ID
		}
	} else {
		// 已有章节：记录必须存在，新建走 new.md 通道
		rec, err := getChapterRecord(ctx, tc.DB, tc.NovelID, ref.ID)
		if err != nil {
			return nil, fmt.Errorf("запрос записи главы: %w", err)
		}
		if rec == nil {
			return &ToolResult{Success: false, Error: fmt.Sprintf("Запись главы не найдена: id=%d, для создания используйте chapters/{volume_id}/new.md или chapters/new.md", ref.ID)}, nil
		}
		ch = rec
	}

	// 读取当前内容（new 通道视为空文件）
	current := ""
	if !ref.IsNew {
		content, err := git.ReadFile(tc.NovelID, physical)
		if err != nil {
			// 大纲先行等工作流：记录存在但文件尚未落盘，full_replace 视为空文件创建
			if !errors.Is(err, os.ErrNotExist) || a.ChangeType != "full_replace" {
				if res, handled := fileReadError(physical, err); handled {
					return res, nil
				}
				return nil, fmt.Errorf("чтение файла %s: %w", physical, err)
			}
		} else {
			current = content
		}
	}

	proposed, err := applyChange(a, current)
	if err != nil {
		return &ToolResult{Success: false, Error: fmt.Sprintf("Ошибка редактирования: %s", err.Error())}, nil
	}
	if proposed == current {
		// 内容未变：title 有变化时仍执行标题更新，否则直接跳过
		if !ref.IsNew && a.Title != "" && ch.Title != a.Title {
			if err := chStore.UpdateTitle(ctx, nil, tc.NovelID, ch.ID, a.Title); err != nil {
				return nil, fmt.Errorf("обновление названия главы: %w", err)
			}
			ch.Title = a.Title
			emitFileMetadataChanged(ctx, tc.NovelID, physical)
			return &ToolResult{Success: true, Data: map[string]any{
				"path":    physical,
				"title":   a.Title,
				"message": "Content unchanged, only title updated",
			}}, nil
		}
		return &ToolResult{Success: true, Data: map[string]any{"path": physical, "message": "Content unchanged, skipped"}}, nil
	}

	// 审批（new 通道展示虚拟路径，已有章节展示物理路径）
	payloadPath := physical
	if ref.IsNew {
		payloadPath = a.Path
	}
	feedback, res, err := requestApproval(ctx, tc, approvalPayload(payloadPath, current, proposed, a))
	if err != nil {
		return nil, err
	}
	if res != nil {
		return res, nil
	}

	// 新建在外层事务中取得 id 并写入首个文件；文件失败时回滚记录与排序调整。
	// 已有章节的标题更新仍延后到正文落盘之后，见下。
	if ref.IsNew {
		// 未指定卷：默认最后一卷；整本书无卷则创建未分卷章节。
		if volumeID == nil {
			last, err := volume.NewStore(tc.DB, tc.LoggerOrDefault()).LastByNovel(ctx, nil, tc.NovelID)
			if err != nil {
				return nil, fmt.Errorf("определение последнего тома: %w", err)
			}
			if last != nil {
				volumeID = &last.ID
			}
		}
		var created *chapter.Chapter
		if err := tc.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var err error
			created, err = chStore.Create(ctx, tx, tc.NovelID, volumeID, a.Title)
			if err != nil {
				return fmt.Errorf("создание записи главы: %w", err)
			}
			// 物理路径按刚分配的章节 id 重新计算（ref.ID 对新建为 0）。
			physical = physicalRWPath(ref.IsOutline, created.ID)
			if err := git.WriteFile(tc.NovelID, physical, proposed); err != nil {
				return fmt.Errorf("запись файла: %w", err)
			}
			return nil
		}); err != nil {
			if errors.Is(err, git.ErrPathEscape) {
				return &ToolResult{Success: false, Error: "Недопустимый путь: " + physical}, nil
			}
			return nil, err
		}
		ch = created
	} else if err := git.WriteFileIfUnchanged(tc.NovelID, physical, current, proposed); err != nil {
		if errors.Is(err, git.ErrFileChanged) {
			return &ToolResult{Success: false, Error: "Файл был изменён, перезагрузите свежее содержимое и повторите"}, nil
		}
		if errors.Is(err, git.ErrPathEscape) {
			return &ToolResult{Success: false, Error: "Недопустимый путь: " + physical}, nil
		}
		return nil, fmt.Errorf("запись файла: %w", err)
	}

	// 标题更新放在正文写入成功之后：写文件失败时不会留下「标题已改、正文未写」的不一致状态，
	// 用户与 AI 看到的标题不会先于正文生效。也必须早于下面的维护链路——
	// RAG 刷新是异步的，切块时读的是 DB 里的标题。
	if !ref.IsNew && a.Title != "" && ch.Title != a.Title {
		if err := chStore.UpdateTitle(ctx, nil, tc.NovelID, ch.ID, a.Title); err != nil {
			if rollbackErr := git.WriteFileIfUnchanged(tc.NovelID, physical, proposed, current); rollbackErr != nil {
				if errors.Is(rollbackErr, git.ErrFileChanged) {
					return nil, fmt.Errorf("обновление названия главы (название не обновлено, текст уже изменён последующей операцией, откат %s не выполнен): %w", physical, err)
				}
				return nil, fmt.Errorf("обновление названия главы (текст записан %s, название не обновлено): %v; откат текста не удался: %w", physical, err, rollbackErr)
			}
			return nil, fmt.Errorf("обновление названия главы (текст откатан %s): %w", physical, err)
		}
		ch.Title = a.Title
	}

	// 正文（非大纲）落盘后的维护链路：向量/搜索缓存/字数/写作日志
	if !ref.IsOutline {
		maintainChapterAfterEdit(ctx, tc, ch, current, proposed)
	}
	emitFileChanged(ctx, tc.NovelID, physical)

	data := map[string]any{
		"path":        physical,
		"change_type": a.ChangeType,
		"approved":    true,
	}
	if ref.IsNew {
		activity.NewStore(tc.DB, tc.LoggerOrDefault()).AddActivity(ctx, activity.ActivityDelta{ChaptersCreated: 1})
		data["chapter_id"] = ch.ID
		if ch.VolumeID != nil {
			data["volume_id"] = *ch.VolumeID
		} else {
			data["volume_id"] = nil
		}
	}

	var injects []InjectMessage
	if feedback != "" {
		injects = append(injects, InjectMessage{Role: "user", Content: "User approved and provided feedback: " + feedback})
	}
	// 章节正文全量替换且内容较长时注入维护提醒
	if !ref.IsOutline && a.ChangeType == "full_replace" && len([]rune(proposed)) > 500 {
		reminder := fmt.Sprintf("You just completed a full replacement of %q.", ch.Title)
		if number, err := chStore.GetReadingNumberByID(ctx, nil, tc.NovelID, ch.ID); err != nil {
			tc.LoggerOrDefault().Warn("failed to compute live chapter number", "chapter_id", ch.ID, "err", err)
		} else {
			reminder = fmt.Sprintf("You just completed a full replacement of %q (chapter %d).", ch.Title, number)
		}
		injects = append(injects, InjectMessage{
			Role:    "user",
			Content: reminder + "Please perform the following maintenance tasks:\n1. Review and update character settings (personality changes, new abilities, identity shifts, etc.)\n2. Update the story timeline (payoff of foreshadowing, recording new foreshadowing, advancing the chapter plan)\n3. Update reader perception (new suspense, resolved suspense)\n4. Update the progress of story arc nodes\nAfter finishing, report a summary of the changes to the user.",
		})
	}
	if a.ChangeType == "line_range_replace" {
		afterEnd := a.StartLine + strings.Count(a.NewContent, "\n")
		data["before"] = linePreview(current, a.StartLine, a.EndLine)
		data["after"] = linePreview(proposed, a.StartLine, afterEnd)
	}
	return &ToolResult{Success: true, Data: data, Inject: injects}, nil
}

// linePreview 返回指定行范围的前后上下文预览，带行号。区间 1-based 闭区间。
func linePreview(content string, start, end int) string {
	lines := strings.Split(content, "\n")
	ctxStart := start - 1
	if ctxStart < 0 {
		ctxStart = 0
	}
	ctxEnd := end
	if ctxEnd > len(lines) {
		ctxEnd = len(lines)
	}

	// 前后各多取一行上下文
	preStart := ctxStart - 1
	if preStart < 0 {
		preStart = 0
	}
	postEnd := ctxEnd + 1
	if postEnd > len(lines) {
		postEnd = len(lines)
	}

	var b strings.Builder
	for i := preStart; i < postEnd; i++ {
		if i == ctxStart {
			b.WriteString("─── Change range ───\n")
		}
		fmt.Fprintf(&b, "%d|%s\n", i+1, lines[i])
		if i == ctxEnd-1 {
			b.WriteString("─── Change end ───\n")
		}
	}
	return b.String()
}

// ── 编辑操作 ──────────────────────────────────────────────

func applyChange(a *EditArgs, current string) (string, error) {
	switch a.ChangeType {
	case "full_replace":
		if a.NewContent == "" {
			return "", fmt.Errorf("Для режима full_replace требуется new_content; чтобы очистить весь файл, используйте line_range_replace(1, total_lines, \"\")")
		}
		return a.NewContent, nil

	case "search_replace":
		if a.SearchText == "" {
			return "", fmt.Errorf("Для режима search_replace требуется search_text")
		}
		result, found, hint := searchReplace(current, a.SearchText, a.NewContent, a.ReplaceAll)
		if !found {
			if hint != "" {
				return "", fmt.Errorf("%s", hint)
			}
			return "", fmt.Errorf("Совпадающий текст не найден, повторите с точным текстом")
		}
		return result, nil

	case "line_range_replace":
		if a.StartLine <= 0 || a.EndLine <= 0 {
			return "", fmt.Errorf("Для режима line_range_replace требуются start_line и end_line")
		}
		if a.StartLine > a.EndLine {
			return "", fmt.Errorf("start_line не может быть больше end_line")
		}
		return lineRangeReplace(current, a.StartLine, a.EndLine, a.NewContent)

	default:
		return "", fmt.Errorf("Неизвестный change_type: %s", a.ChangeType)
	}
}

// searchReplace 在 content 中查找 searchText 并替换为 newContent。
// replaceAll=false 时仅替换第一个匹配。返回修改后的内容、是否找到匹配、以及失败时的模糊匹配提示。
func searchReplace(content, searchText, newContent string, replaceAll bool) (result string, found bool, hint string) {
	searchText = strings.TrimRight(searchText, "\n")

	// 层 1：精确匹配
	if idx := strings.Index(content, searchText); idx >= 0 {
		n := 1
		if replaceAll {
			n = -1
		}
		return strings.Replace(content, searchText, newContent, n), true, ""
	}

	// 层 2：TrimSpace 后精确匹配
	trimmedSearch := strings.TrimSpace(searchText)
	if trimmedSearch != searchText {
		if idx := strings.Index(content, trimmedSearch); idx >= 0 {
			n := 1
			if replaceAll {
				n = -1
			}
			return strings.Replace(content, trimmedSearch, newContent, n), true, ""
		}
	}

	// 层 3：标点归一化后匹配（引号变体统一再比）
	// 只要两边有一方被归一化改变就重新比对
	normSearch := normalizePunctuation(searchText)
	normContent := normalizePunctuation(content)
	if normSearch != searchText || normContent != content {
		// 归一化后字符宽度可能不同（弯引号 3 字节 → ASCII 1 字节），必须按 rune 对齐
		normCRunes := []rune(normContent)
		normSRunes := []rune(normSearch)
		if pos := runeIndex(normCRunes, normSRunes); pos >= 0 {
			origRunes := []rune(content)
			original := string(origRunes[pos : pos+len(normSRunes)])
			n := 1
			if replaceAll {
				n = -1
			}
			return strings.Replace(content, original, newContent, n), true, ""
		}
	}

	// 层 4：模糊匹配反馈（不替换，只告诉 LLM 正确文本长什么样）
	hint = fuzzyHint(searchText, content)
	return "", false, hint
}

// normalizePunctuation 将中文标点变体统一映射为 ASCII 等效字符。
// 仅在查找匹配时使用，不修改文件内容。
var quoteReplace = strings.NewReplacer(
	"“", `"`, "”", `"`, // " " 弯引号
	"「", `"`, "」", `"`, // 「 」 直角引号
	"『", `"`, "』", `"`, // 『 』 双直角引号
	"＂", `"`, // ＂ 全角引号
	"‘", `'`, "’", `'`, // ' ' 弯单引号
	"＇", `'`, // ＇ 全角单引号
)

func normalizePunctuation(s string) string {
	return quoteReplace.Replace(s)
}

// runeIndex 在 rune 切片 a 中查找子切片 b，返回首位置，未找到返回 -1。
func runeIndex(a, b []rune) int {
	for i := 0; i <= len(a)-len(b); i++ {
		match := true
		for j, r := range b {
			if a[i+j] != r {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// fuzzyHint 在 content 中找到与 searchText 最相似的段落，返回格式化的提示信息。
// 失败时调用，帮助 LLM 根据实际内容修正 search_text 后重试。
func fuzzyHint(searchText, content string) string {
	searchLines := strings.Split(strings.TrimSpace(searchText), "\n")
	contentLines := strings.Split(content, "\n")
	if len(searchLines) == 0 || len(contentLines) == 0 {
		return ""
	}

	w := len(searchLines)
	bestScore := 0.0
	bestStart := 0
	bestW := w

	// 滑动窗口逐段比较
	for i := 0; i <= len(contentLines)-w; i++ {
		candidate := strings.Join(contentLines[i:i+w], "\n")
		score := partialRatio(searchText, candidate)
		if score > bestScore {
			bestScore = score
			bestStart = i
		}
	}
	// 也尝试窗口 ±2 行
	for _, delta := range []int{2, -2, 1, -1} {
		ws := w + delta
		if ws <= 0 || ws > len(contentLines) {
			continue
		}
		for i := 0; i <= len(contentLines)-ws; i++ {
			candidate := strings.Join(contentLines[i:i+ws], "\n")
			score := partialRatio(searchText, candidate)
			if score > bestScore {
				bestScore = score
				bestStart = i
				bestW = ws
			}
		}
	}

	if bestScore < 0.4 {
		return "Не найдено ни одного похожего содержимого, повторите с точным текстом или line_range_replace."
	}

	contextStart := bestStart
	if contextStart > 2 {
		contextStart -= 2
	}
	contextEnd := bestStart + bestW
	if contextEnd+2 < len(contentLines) {
		contextEnd += 2
	} else {
		contextEnd = len(contentLines)
	}
	// 取匹配行 + 前后各 2 行上下文
	contextLines := contentLines[contextStart:contextEnd]
	if len(contextLines) > 8 {
		contextLines = contextLines[:8]
	}
	nearby := strings.Join(contextLines, "\n")

	return fmt.Sprintf(
		"Точное совпадение не найдено. Ниже приведён наиболее похожий фрагмент по нечёткому совпадению (степень сходства %.0f%%, около строк %d-%d), только для справки — решите сами, это ли место, которое вы хотите изменить:\n%s\nЕсли уверены, что это именно оно, можно изменить его напрямую через line_range_replace(start_line=%d, end_line=%d) или исправить search_text по фактическому содержимому и повторно вызвать search_replace.",
		bestScore*100, bestStart+1, bestStart+bestW, nearby, bestStart+1, bestStart+bestW,
	)
}

// partialRatio 计算两段文本的字符级相似度（0.0~1.0）。
// 使用滑动窗口在较长的文本中找与较短文本最匹配的片段。
func partialRatio(a, b string) float64 {
	short, long := a, b
	if len(a) > len(b) {
		short, long = b, a
	}
	if len(short) == 0 {
		if len(long) == 0 {
			return 1.0
		}
		return 0
	}
	shortRunes := []rune(short)
	longRunes := []rune(long)
	if len(longRunes) < len(shortRunes) {
		longRunes, shortRunes = shortRunes, longRunes
	}

	best := 0.0
	for i := 0; i <= len(longRunes)-len(shortRunes); i++ {
		matches := 0
		for j, sr := range shortRunes {
			if sr == longRunes[i+j] {
				matches++
			}
		}
		score := float64(matches) / float64(len(shortRunes))
		if score > best {
			best = score
		}
	}
	return best
}

// lineRangeReplace 替换 [startLine, endLine] 区间（1-based，含两端）。
func lineRangeReplace(content string, startLine, endLine int, newContent string) (string, error) {
	lines := strings.Split(content, "\n")
	if startLine < 1 || endLine > len(lines) || startLine > endLine {
		return "", fmt.Errorf("Номер строки вне диапазона: start=%d end=%d всего строк=%d", startLine, endLine, len(lines))
	}

	var result []string
	result = append(result, lines[:startLine-1]...)
	if newContent != "" {
		result = append(result, strings.Split(newContent, "\n")...)
	}
	result = append(result, lines[endLine:]...)
	return strings.Join(result, "\n"), nil
}

// ── 路径解析 ──────────────────────────────────────────────

var plainPathRe = regexp.MustCompile(`^(goink\.md|skills/[^/]+\.md|~/.goink/skills/[^/]+\.md)$`)

// validPath 校验 edit/read 支持的全部路径形态。
func validPath(p string) bool {
	if _, ok := git.ParseChapterLikePath(p); ok {
		return true
	}
	if _, ok := git.ParseVolumePath(p); ok {
		return true
	}
	return plainPathRe.MatchString(p)
}

func isSkillPath(p string) bool {
	return strings.HasPrefix(p, "skills/") || strings.HasPrefix(p, "~/.goink/skills/")
}

// ── 章节记录 ──────────────────────────────────────────────

// getChapterRecord 按章节 id 取当前小说的章节记录，不存在返回 (nil, nil)。
func getChapterRecord(ctx context.Context, db *gorm.DB, novelID, id int64) (*chapter.Chapter, error) {
	var ch chapter.Chapter
	err := db.WithContext(ctx).Where("id = ? AND novel_id = ?", id, novelID).First(&ch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

// maintainChapterAfterEdit 章节正文落盘后的维护链路：向量刷新、搜索缓存、字数统计。
// 向量与搜索按 chapter_id 键控；写作日志将在其所属领域迁移时切换。
func maintainChapterAfterEdit(ctx context.Context, tc ToolContext, ch *chapter.Chapter, previous, content string) {
	rag.SubmitRefresh(tc.NovelID, ch.ID, content)
	if tc.SearchService != nil {
		tc.SearchService.UpdateCachedChapter(tc.NovelID, ch.ID, content)
	}

	stats := text.ComputeStats(content)

	changes := text.ComputeWordChanges(previous, content)
	writing.NewStore(tc.DB, tc.LoggerOrDefault()).LogChanges(ctx, tc.NovelID, ch.ID, changes.Added, changes.Deleted)

	tc.DB.WithContext(ctx).Model(&chapter.Chapter{}).
		Where("id = ?", ch.ID).
		Update("word_count", stats.WordCount)
}

// ── 工具描述 ──────────────────────────────────────────────

const editDescription = `Edit a novel file (chapter body, outline, volume outline, story state goink.md, or skill file). Three editing modes are supported: full_replace (replace the entire file), search_replace (find and replace), line_range_replace (replace a line range).

Creating a new chapter or outline: use the new.md form for path (see the path parameter description for the format), and only full_replace is allowed; on success the result carries the assigned chapter_id, volume_id, and physical path, and all subsequent edits must use the returned physical path.
Volume outline: use volumes/id_{volume_id}.md for path; the volume must exist, and full_replace can create the corresponding volume outline file for the first time.

Required parameters per mode:
- full_replace: new_content
- search_replace: search_text + new_content
- line_range_replace: start_line + end_line + new_content
Required for all modes: path, change_type, reason
(see the new_content parameter description for the specific behavior when new_content is empty)

Mode selection strategy:
- If search_replace fails twice in a row with "no match found", switch directly to line_range_replace; do not keep retrying the same mode
- Before using line_range_replace, always re-read to confirm the line numbers (they may have shifted due to earlier operations); after running, before/after context is returned — if before does not match expectations, the line numbers have shifted and you must re-read and correct them before retrying

All changes first generate a git diff for user approval; the file is written only after approval is granted. When rejected, the user's feedback is returned, and you can revise and retry based on the feedback.`

// ── read ──────────────────────────────────────────────────

// ReadArgs 是 read 工具的参数。
type ReadArgs struct {
	Path         string `json:"path" jsonschema:"required,description=Path of the file to read. Chapter body: chapters/id_{chapter_id}.md (primary form) or chapters/{volume_id}/id_{chapter_id}.md (equivalent lenient form); chapter outline: outlines/id_{chapter_id}.md or outlines/{volume_id}/id_{chapter_id}.md; volume outline: volumes/id_{volume_id}.md; story state goink.md; novel-level skill skills/<name>.md; user-level skill ~/.goink/skills/<name>.md; builtin skill /builtin/skills/<name>.md (read-only); extended tools directory mcp/index.md; tool description and parameter schema mcp/schemas/<tool_name>.json (virtual read-only path)" validate:"required"`
	IncludeLines *bool  `json:"include_lines" jsonschema:"default=true,description=Whether to include the line number prefix (e.g. 123|). Defaults to true, used for precise referencing and line range editing. Pass false to get plain text"`
	StartLine    int    `json:"start_line" jsonschema:"default=1,description=Start line number, 1-based inclusive; must be <= end_line" validate:"omitempty,min=1"`
	EndLine      int    `json:"end_line" jsonschema:"default=2000,description=End line number, 1-based inclusive; values beyond the end are auto-truncated to the end of the file. Omitting or passing 0 both use the default 2000" validate:"omitempty,min=0"`
}

// ReadTool 读取文件内容（章节正文或故事状态 goink.md）。
// 默认含行号前缀（123|），LLM 传 include_lines=false 获取纯文本。
// start_line/end_line 支持行范围读取，用于翻页和精确引用。
type ReadTool struct {
	registry *Registry
}

func (t *ReadTool) Name() string           { return "read" }
func (t *ReadTool) Description() string    { return readDescription }
func (t *ReadTool) Category() ToolCategory { return CategoryNovelManagement }

func (t *ReadTool) JSONSchema() json.RawMessage { return SchemaOf(ReadArgs{}) }
func (t *ReadTool) ExposeToLLM() bool           { return true }
func (t *ReadTool) NewArgs() any                { return &ReadArgs{} }

func (t *ReadTool) Execute(ctx context.Context, args any, tc ToolContext) (*ToolResult, error) {
	a := args.(*ReadArgs)

	if strings.HasPrefix(a.Path, "mcp/") {
		content, schema, err := t.registry.readCatalog(a.Path)
		if err != nil {
			if isBusinessError(err) {
				return &ToolResult{Success: false, Error: err.Error()}, nil
			}
			return nil, err
		}
		if schema {
			return &ToolResult{Success: true, Data: map[string]any{
				"path": a.Path, "content": content,
			}}, nil
		}
		return renderReadResult(a, a.Path, a.Path, content)
	}

	// builtin skill 走 store 内存
	if strings.HasPrefix(a.Path, "/builtin/skills/") {
		return t.readBuiltinSkill(a, tc)
	}

	if volumeID, ok := git.ParseVolumePath(a.Path); ok {
		return t.readVolumeOutline(ctx, a, tc, volumeID)
	}

	// 章节/大纲按 id 扁平路径读取（两级路径归一化）
	if ref, ok := git.ParseChapterLikePath(a.Path); ok {
		return t.readChapterLike(ctx, a, tc, ref)
	}
	if !validPath(a.Path) {
		return &ToolResult{Success: false, Error: invalidPathHint}, nil
	}

	content, err := git.ReadFile(tc.NovelID, a.Path)
	if err != nil {
		if res, handled := fileReadError(a.Path, err); handled {
			return res, nil
		}
		return nil, fmt.Errorf("чтение файла %s: %w", a.Path, err)
	}
	return renderReadResult(a, a.Path, a.Path, content)
}

// readChapterLike 读取章节正文或大纲。
// 物理文件永远按 id 扁平路径读，两级路径仅作容错别名归一化。
func (t *ReadTool) readChapterLike(ctx context.Context, a *ReadArgs, tc ToolContext, ref git.ChapterPathRef) (*ToolResult, error) {
	if ref.IsNew {
		return &ToolResult{Success: false, Error: "new.md предназначен только для создания новой главы/конспекта через edit; его нельзя читать"}, nil
	}

	physical := git.ChapterPath(ref.ID)
	suffix := ""
	if ref.IsOutline {
		physical = git.OutlinePath(ref.ID)
		suffix = " (outline)"
	}

	content, err := git.ReadFile(tc.NovelID, physical)
	if err != nil {
		if res, handled := fileReadError(physical, err); handled {
			return res, nil
		}
		return nil, fmt.Errorf("чтение файла %s: %w", physical, err)
	}

	display := physical
	ch, err := getChapterRecord(ctx, tc.DB, tc.NovelID, ref.ID)
	if err != nil {
		return nil, fmt.Errorf("query chapter record: %w", err)
	}
	if ch != nil {
		display = ch.Title + suffix
	}

	return renderReadResult(a, physical, display, content)
}

// readVolumeOutline 读取已有卷的卷纲文件，并以卷名作为展示标题。
func (t *ReadTool) readVolumeOutline(ctx context.Context, a *ReadArgs, tc ToolContext, volumeID int64) (*ToolResult, error) {
	v, err := volume.NewStore(tc.DB, tc.LoggerOrDefault()).GetByID(ctx, nil, tc.NovelID, volumeID)
	if err != nil {
		if errors.Is(err, volume.ErrNotFound) {
			return &ToolResult{Success: false, Error: err.Error()}, nil
		}
		return nil, fmt.Errorf("запрос тома: %w", err)
	}

	path := git.VolumePath(volumeID)
	content, err := git.ReadFile(tc.NovelID, path)
	if err != nil {
		if res, handled := fileReadError(path, err); handled {
			return res, nil
		}
		return nil, fmt.Errorf("чтение файла %s: %w", path, err)
	}
	return renderReadResult(a, path, v.Name+" (volume outline)", content)
}

// renderReadResult 按行窗口切片并渲染读取结果（行号前缀、截断标记）。
func renderReadResult(a *ReadArgs, path, display, content string) (*ToolResult, error) {
	start := a.StartLine
	if start == 0 {
		start = 1
	}
	end := a.EndLine
	if end == 0 {
		end = 2000
	}

	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	if start > end {
		return &ToolResult{Success: false, Error: fmt.Sprintf("start_line(%d) не может быть больше end_line(%d)", start, end)}, nil
	}
	if start > totalLines {
		return &ToolResult{Success: false, Error: fmt.Sprintf("Стартовая строка %d выходит за пределы общего числа строк %d", start, totalLines)}, nil
	}
	if end > totalLines {
		end = totalLines
	}

	selected := lines[start-1 : end]

	includeLines := a.IncludeLines == nil || *a.IncludeLines

	var output string
	if includeLines {
		var sb strings.Builder
		for i, line := range selected {
			fmt.Fprintf(&sb, "%d|%s\n", start+i, line)
		}
		output = strings.TrimRight(sb.String(), "\n")
	} else {
		output = strings.Join(selected, "\n")
	}

	data := map[string]any{
		"path":        path,
		"display":     display,
		"content":     output,
		"total_lines": totalLines,
		"start_line":  start,
		"end_line":    end,
	}
	if end < totalLines {
		data["truncated"] = true
	}

	return &ToolResult{Success: true, Data: data}, nil
}

// readBuiltinSkill 从 store 内存读取内置 skill，全量返回。
func (t *ReadTool) readBuiltinSkill(a *ReadArgs, tc ToolContext) (*ToolResult, error) {
	if tc.SkillStore == nil {
		return &ToolResult{Success: false, Error: "хранилище skill не инициализировано"}, nil
	}

	name := strings.TrimSuffix(strings.TrimPrefix(a.Path, "/builtin/skills/"), ".md")
	if name == "" {
		return &ToolResult{Success: false, Error: "Недопустимый путь skill: " + a.Path}, nil
	}

	sk, ok := tc.SkillStore.Get(tc.NovelID, name)
	if !ok {
		return &ToolResult{Success: false, Error: fmt.Sprintf("Встроенный skill %q не найден", name)}, nil
	}

	return &ToolResult{Success: true, Data: map[string]any{
		"path":    a.Path,
		"display": fmt.Sprintf("Skill: %s", sk.Name),
		"content": sk.RawContent,
	}}, nil
}

// ── 工具描述 ──────────────────────────────────────────────

const readDescription = `Read a novel file, volume outline, skill file, or the extended tools directory.

Tools directory (virtual read-only content, not corresponding to disk files):
- mcp/index.md: extended tool names, brief descriptions, and schema paths
- mcp/schemas/<tool_name>.json: full tool description and inputSchema; tool_name matches the run_mcp argument

Features:
- Adds a line number prefix by default, to facilitate later line_range_replace and search_replace by the edit tool
- Returns total_lines, the total line count of the full text, used to determine whether content was truncated; use line_range_replace(1, total_lines, "") to clear the entire file
- start_line and end_line support line range reading, for pagination or precise referencing`

// ── 注册 ──────────────────────────────────────────────────

// RegisterRWTools 注册读写工具。
func RegisterRWTools(r *Registry) {
	r.Register(&ReadTool{registry: r})
	r.Register(&EditTool{})
}
