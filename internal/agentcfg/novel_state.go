package agentcfg

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/sigpanic/goink/internal/git"
	"github.com/sigpanic/goink/internal/novel"
)

// NovelState 构建小说动态状态快照（原 System3），每轮对话开头注入。
// 只包含基本信息 + 故事状态文档（goink.md）。
// 稳定内容（设定/偏好）由 NovelProfile 注入，与 NovelState 分离以优化 KV cache。
// 具体数据（角色、时间线等）由 MCP 工具按需提供。
//
// 调用方负责事务外构建（compress 模式）：传入的 db 应为带 ctx 的非事务句柄，
// 避免在事务内调用导致 SQLite 单连接池死锁。
func NovelState(ctx context.Context, db *gorm.DB, novelID int64) (string, error) {
	var n novel.Novel
	if err := db.WithContext(ctx).First(&n, novelID).Error; err != nil {
		return "", fmt.Errorf("agentcfg: load novel %d: %w", novelID, err)
	}

	var b strings.Builder
	b.WriteString("[Basic novel information]\n")
	fmt.Fprintf(&b, "Title: %s\n", n.Title)
	if n.Genre != "" {
		fmt.Fprintf(&b, "Genre: %s\n", n.Genre)
	}
	if n.Description != "" {
		fmt.Fprintf(&b, "Synopsis: %s\n", n.Description)
	}

	state, err := git.ReadFile(novelID, git.GoinkPath())
	if err == nil && state != "" {
		b.WriteString("\n[Story state document]\n")
		b.WriteString(state)
	}

	return b.String(), nil
}
