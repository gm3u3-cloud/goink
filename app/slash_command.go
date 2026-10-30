package app

// ListSlashCommandsInput 是 ListSlashCommands 的入参。
type ListSlashCommandsInput struct {
	NovelID int64 `json:"novel_id"`
}

// SlashCommand 是斜杠菜单的统一条目，数据源来自 skill store，Type 直接对应 skill mode。
type SlashCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"` // "auto" | "manual" | "always"（对应 skill mode）
}

// ListSlashCommands 返回所有 skill 对应的斜杠菜单条目。
func (a *App) ListSlashCommands(input ListSlashCommandsInput) []SlashCommand {
	if a.skill == nil {
		return nil
	}
	meta := a.skill.ListMeta(input.NovelID)
	result := make([]SlashCommand, 0, len(meta))
	for _, m := range meta {
		result = append(result, SlashCommand{
			Name:        m.Name,
			Description: m.Description,
			Type:        m.Mode,
		})
	}
	return result
}

// resolveSlashCommand 根据名称查找 skill 并返回注入内容（空串表示未命中）。
func (a *App) resolveSlashCommand(novelID int64, name string) (inject string, logName string) {
	if a.skill == nil {
		return "", ""
	}
	sk, ok := a.skill.Get(novelID, name)
	if !ok {
		return "", ""
	}

	switch sk.Mode {
	case "always":
		// always 已在会话开头注入，/ 触发时只给出提示
		content := "The user used /" + sk.Name + " to remind you about the always-on skill \"" + sk.Name + "\". Its full content was injected at the start of this conversation; follow it strictly."
		return "<system-reminder>\n" + content + "\n</system-reminder>", sk.Name
	case "manual":
		content := "The user enabled the quick command \"" + sk.Name + "\". Work according to the content of this command and the user's needs.\n\n---\n" + sk.Content + "\n---"
		return "<system-reminder>\n" + content + "\n</system-reminder>", sk.Name
	default: // auto
		content := "The user enabled the skill \"" + sk.Name + "\". Work according to the definition of this skill and the user's needs.\n\n---\n" + sk.RawContent + "\n---"
		return "<system-reminder>\n" + content + "\n</system-reminder>", sk.Name
	}
}
