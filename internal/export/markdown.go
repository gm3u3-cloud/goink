package export

import (
	"fmt"
	"strings"
	"time"

	"github.com/sigpanic/goink/internal/novel"
)

func exportMarkdown(n *novel.Novel, chapters []ChapterWithContent) ([]byte, string, error) {
	var b strings.Builder

	// 书名 + 元信息
	fmt.Fprintf(&b, "# %s\n\n", n.Title)
	if n.Genre != "" {
		fmt.Fprintf(&b, "**Жанр**: %s  \n", n.Genre)
	}
	if n.Description != "" {
		fmt.Fprintf(&b, "**Описание**: %s  \n", n.Description)
	}
	fmt.Fprintf(&b, "**Время экспорта**: %s  \n", time.Now().Format("2006-01-02 15:04"))
	b.WriteString("\n---\n\n")

	// 目录
	b.WriteString("## Содержание\n\n")
	for _, cc := range chapters {
		ch := cc.Chapter
		title := ch.Title
		if title == "" {
			title = fmt.Sprintf("Глава %d", ch.ReadingNumber)
		}
		fmt.Fprintf(&b, "- [Глава %d %s](#глава-%d)\n\n", ch.ReadingNumber, title, ch.ReadingNumber)
	}
	b.WriteString("\n---\n\n")

	// 正文
	for _, cc := range chapters {
		ch := cc.Chapter
		title := ch.Title
		if title == "" {
			title = fmt.Sprintf("Глава %d", ch.ReadingNumber)
		}
		fmt.Fprintf(&b, "## Глава %d %s\n\n", ch.ReadingNumber, title)
		b.WriteString(cc.Content)
		b.WriteString("\n\n")
	}

	filename, err := DefaultFilename(n, "markdown")
	if err != nil {
		return nil, "", err
	}
	return []byte(b.String()), filename, nil
}
