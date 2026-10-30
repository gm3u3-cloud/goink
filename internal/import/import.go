package imp

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"gorm.io/gorm"

	"github.com/sigpanic/goink/internal/chapter"
	"github.com/sigpanic/goink/internal/config"
	"github.com/sigpanic/goink/internal/git"
	"github.com/sigpanic/goink/internal/novel"
	"github.com/sigpanic/goink/internal/text"
)

// ImportResult 是完整导入流程的结果。
type ImportResult struct {
	NovelID         int64            `json:"novel_id"`
	Title           string           `json:"title"`
	ChapterCount    int              `json:"chapter_count"`
	SkippedCount    int              `json:"skipped_count"`
	SkippedChapters []SkippedChapter `json:"skipped_chapters"`
	NeedsLLM        bool             `json:"needs_llm"`           // 正则分割结果不合理，提示前端可调用 LLM 分析
	FilePath        string           `json:"file_path,omitempty"` // NeedsLLM 时回传文件路径，供前端调 ImportWithLLM
}

// ProgressCallback 是进度回调函数，app 层用它来推送前端事件。
type ProgressCallback func(stage, message string, current, total, percent int, novelID int64)

// Import 执行完整的小说导入流程：解析文件 → 创建 Novel → 写入章节 → git 提交。
func Import(ctx context.Context, logger *slog.Logger, db *gorm.DB, filePath string, gitName, gitEmail string, onProgress ProgressCallback) (*ImportResult, error) {
	onProgress("parse", "Разбор файла", 0, 0, 0, 0)

	result, err := Parse(filePath, logger)
	if err != nil {
		onProgress("error", "Ошибка разбора файла, импорт остановлен", 0, 0, 0, 0)
		return nil, fmt.Errorf("Ошибка разбора файла: %w", err)
	}

	// 正则分割结果不合理，直接返回 NeedsLLM 标记，不创建 Novel
	// FilePath 回传给前端，供其调用 ImportWithLLM
	if result.NeedsLLM {
		return &ImportResult{
			Title:    result.Title,
			NeedsLLM: true,
			FilePath: filePath,
		}, nil
	}

	title := result.Title
	if title == "" {
		title = "Без названия"
	}

	return doImport(ctx, logger, db, result, title, filePath, gitName, gitEmail, onProgress)
}

// ImportWithResult 使用已有的解析结果执行完整导入流程。
// 用于 LLM 兜底场景：先 AnalyzeWithLLM → ImportWithResult。
func ImportWithResult(ctx context.Context, logger *slog.Logger, db *gorm.DB, result *Result, filePath string, gitName, gitEmail string, onProgress ProgressCallback) (*ImportResult, error) {
	title := result.Title
	if title == "" {
		title = inferTitle(filePath)
		if title == "" {
			title = "Без названия"
		}
	}

	return doImport(ctx, logger, db, result, title, filePath, gitName, gitEmail, onProgress)
}

// doImport 执行导入的核心流程：创建 Novel → 建章节记录（拿 id）→ 写 id 命名文件 → git add → git commit。
// Import 和 ImportWithResult 共享此逻辑。
//
// 事务策略（章节文件按 id 命名，必须先建 DB 记录拿到 id）：
//  1. 创建 Novel + 建 Chapters 记录（DB 事务，原子）
//  2. 写章节文件（chapters/id_{id}.md）+ git add（可逆操作，不 commit）
//  3. git commit（不可逆但极低失败率，放在文件成功之后）
func doImport(ctx context.Context, logger *slog.Logger, db *gorm.DB, result *Result, title, filePath, gitName, gitEmail string, onProgress ProgressCallback) (*ImportResult, error) {
	chapterCount := len(result.Chapters)
	onProgress("create_novel", "Создание произведения", 0, chapterCount, 10, 0)

	n := novel.Novel{
		Title:       title,
		Description: fmt.Sprintf("Импортировано из %s, всего %d гл.", filePath, chapterCount),
	}

	// ── Step 1: 创建 Novel 记录（拿 ID，文件路径依赖此 ID） ──
	// 后续步骤失败时 cleanupImport 会删除 Novel 记录和文件。

	if err := db.WithContext(ctx).Create(&n).Error; err != nil {
		onProgress("error", "Не удалось создать произведение", 0, chapterCount, 0, 0)
		return nil, fmt.Errorf("Не удалось создать произведение: %w", err)
	}

	repo, err := git.New(n.ID, gitName, gitEmail, logger)
	if err != nil {
		cleanupImport(db, ctx, logger, n.ID)
		onProgress("error", "Не удалось инициализировать репозиторий произведения", 0, chapterCount, 0, n.ID)
		return nil, fmt.Errorf("Не удалось инициализировать репозиторий произведения: %w", err)
	}

	if err := git.WriteFile(n.ID, git.GoinkPath(), ""); err != nil {
		cleanupImport(db, ctx, logger, n.ID)
		onProgress("error", "Не удалось создать goink.md", 0, chapterCount, 0, n.ID)
		return nil, fmt.Errorf("Не удалось создать goink.md: %w", err)
	}

	// ── Step 2: DB 事务写入 Chapters（原子操作，拿 id） ──
	// 章节文件以 id 命名（chapters/id_{id}.md），必须先建记录拿到 id 再写文件。
	onProgress("saving_metadata", "Сохранение метаданных глав", 0, chapterCount, 12, n.ID)
	chapters := make([]chapter.Chapter, 0, chapterCount)
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, ch := range result.Chapters {
			chapNum := i + 1
			chap := chapter.Chapter{
				NovelID:   n.ID,
				SortOrder: chapNum,
				Title:     ch.Title,
				WordCount: text.ComputeStats(ch.Content).WordCount,
			}
			if err := tx.Create(&chap).Error; err != nil {
				return fmt.Errorf("Не удалось создать метаданные главы %d: %w", chapNum, err)
			}
			chapters = append(chapters, chap)
		}
		return nil
	}); err != nil {
		// DB 写入失败，完整清理：删除 Novel 记录 + 文件目录
		cleanupImport(db, ctx, logger, n.ID)
		onProgress("error", "Импорт не удался, созданные данные и файлы отменены", 0, chapterCount, 0, n.ID)
		return nil, err
	}

	// ── Step 3: 写章节文件（id 命名） + git add（可逆，不 commit） ──
	onProgress("write_chapters", "Запись глав", 0, chapterCount, 15, n.ID)
	for i, ch := range result.Chapters {
		chapNum := i + 1
		if err := git.WriteFile(n.ID, git.ChapterPath(chapters[i].ID), ch.Content); err != nil {
			cleanupImport(db, ctx, logger, n.ID)
			onProgress("error", fmt.Sprintf("Не удалось записать главу %d", chapNum), chapNum, chapterCount, 0, n.ID)
			return nil, fmt.Errorf("Не удалось записать главу %d: %w", chapNum, err)
		}
		onProgress(
			"write_chapters",
			fmt.Sprintf("Запись главы %d/%d", chapNum, chapterCount),
			chapNum,
			chapterCount,
			importProgressPercent(chapNum, chapterCount),
			n.ID,
		)
	}

	onProgress("commit", "Добавление импортированных файлов в git", chapterCount, chapterCount, 90, n.ID)
	if err := repo.StageAll(); err != nil {
		cleanupImport(db, ctx, logger, n.ID)
		onProgress("error", "Не удалось добавить импортированные файлы в git", 0, chapterCount, 0, n.ID)
		return nil, fmt.Errorf("Не удалось добавить импортированные файлы в git: %w", err)
	}

	// ── Step 4: git commit（不可逆但极低失败率） ──
	// DB 已提交，Chapters 已创建。git commit 失败时数据完整，只是缺版本记录。
	if _, err := repo.Commit(fmt.Sprintf("import novel: %s", title)); err != nil {
		// 不回滚 DB：数据完整，只是缺 git 初始提交
		// 下次任何文件变更都会触发新的 commit，数据不会丢失
		logger.Error("import: git commit failed, data saved but version record missing; next edit will commit automatically",
			"novel_id", n.ID, "err", err)
		onProgress("done", fmt.Sprintf("Импортировано «%s», всего %d гл. (версия будет зафиксирована при следующем редактировании)", title, chapterCount), chapterCount, chapterCount, 100, n.ID)
	} else {
		onProgress("done", fmt.Sprintf("Импортировано «%s», всего %d гл.", title, chapterCount), chapterCount, chapterCount, 100, n.ID)
	}

	return &ImportResult{
		NovelID:         n.ID,
		Title:           title,
		ChapterCount:    chapterCount,
		SkippedCount:    len(result.SkippedChapters),
		SkippedChapters: result.SkippedChapters,
	}, nil
}

// cleanupImport 清理导入失败后的残留：删除文件目录、Chapters 记录和 Novel 记录。
func cleanupImport(db *gorm.DB, ctx context.Context, logger *slog.Logger, novelID int64) {
	// 删除文件目录（含 .git 仓库）
	if err := os.RemoveAll(config.NovelDirPath(novelID)); err != nil {
		logger.Error("failed to remove novel directory after failed import", "novel_id", novelID, "err", err)
	}
	// 删除 Chapters 记录（DB 建记录先于写文件，文件失败时记录已存在，需一并清理）
	if err := db.WithContext(ctx).Where("novel_id = ?", novelID).Delete(&chapter.Chapter{}).Error; err != nil {
		logger.Error("failed to delete chapter records after failed import", "novel_id", novelID, "err", err)
	}
	// 删除 Novel 记录
	if err := db.WithContext(ctx).Delete(&novel.Novel{}, novelID).Error; err != nil {
		logger.Error("failed to delete novel record after failed import", "novel_id", novelID, "err", err)
	}
}

func importProgressPercent(current, total int) int {
	if total <= 0 {
		return 15
	}
	percent := 15 + current*75/total
	if percent > 90 {
		return 90
	}
	return percent
}
