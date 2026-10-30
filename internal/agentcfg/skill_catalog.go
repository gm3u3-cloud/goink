package agentcfg

import (
	"strings"

	"github.com/sigpanic/goink/internal/skill"
)

// BuildSkillCatalog 将 auto 模式的 skill 元数据格式化为 LLM 可用的技能目录。
// manual 和 always 模式的 skill 不出现在目录中。
// 按 novel > user > builtin 分组输出，每组只列出 name 和 description。
func BuildSkillCatalog(meta []skill.SkillMeta) string {
	if len(meta) == 0 {
		return ""
	}

	novel := filterBySource(meta, "novel")
	user := filterBySource(meta, "user")
	builtin := filterBySource(meta, "builtin")

	var b strings.Builder
	b.WriteString("<available_skills>\n")

	if len(novel) > 0 {
		b.WriteString("## Novel-specific skills\n")
		for _, s := range novel {
			b.WriteString("- ")
			b.WriteString(s.Name)
			if s.Description != "" {
				b.WriteString(": ")
				b.WriteString(s.Description)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if len(user) > 0 {
		b.WriteString("## User skills\n")
		for _, s := range user {
			b.WriteString("- ")
			b.WriteString(s.Name)
			if s.Description != "" {
				b.WriteString(": ")
				b.WriteString(s.Description)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if len(builtin) > 0 {
		b.WriteString("## Built-in skills (read-only)\n")
		for _, s := range builtin {
			b.WriteString("- ")
			b.WriteString(s.Name)
			if s.Description != "" {
				b.WriteString(": ")
				b.WriteString(s.Description)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("---\n")
	b.WriteString("Use the read tool to load a skill's full content:\n")
	b.WriteString("- skills/<name>.md (novel skill)\n")
	b.WriteString("- ~/.goink/skills/<name>.md (user skill)\n")
	b.WriteString("- /builtin/skills/<name>.md (built-in skill, read-only)\n")
	b.WriteString("Use the edit tool to create or modify a skill. Built-in skills cannot be edited.\n")
	b.WriteString("</available_skills>")

	return b.String()
}

// BuildAlwaysSkillsContent 拼合所有 always 模式的 skill 正文，作为常驻系统消息注入。
// 不包含 YAML frontmatter，只拼接 body 内容。
func BuildAlwaysSkillsContent(meta []skill.SkillMeta, store *skill.Store, novelID int64) string {
	always := filterByMode(meta, skill.ModeAlways)
	if len(always) == 0 || store == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("[Always-on skills]\n")
	b.WriteString("The following skills are always active in this conversation:\n\n")

	for _, m := range always {
		sk, ok := store.Get(novelID, m.Name)
		if !ok {
			continue
		}
		b.WriteString("--- ")
		b.WriteString(sk.Name)
		b.WriteString(" ---\n")
		b.WriteString(sk.Content)
		b.WriteString("\n\n")
	}

	return strings.TrimSpace(b.String())
}

func filterBySource(meta []skill.SkillMeta, source string) []skill.SkillMeta {
	var result []skill.SkillMeta
	for _, s := range meta {
		if s.Source == source {
			result = append(result, s)
		}
	}
	return result
}

func filterByMode(meta []skill.SkillMeta, mode string) []skill.SkillMeta {
	var result []skill.SkillMeta
	for _, s := range meta {
		if s.Mode == mode {
			result = append(result, s)
		}
	}
	return result
}
