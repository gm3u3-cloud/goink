package skill

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseFile 读取并解析一个 skill markdown 文件。
func ParseFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("skill: чтение файла %s не удалось: %w", path, err)
	}
	sk, err := ParseBytes(data, "user")
	if err != nil {
		return nil, fmt.Errorf("skill: разбор %s не удалось: %w", path, err)
	}
	return sk, nil
}

// ParseFS 从 fs.FS 中读取并解析一个 skill markdown 文件。
func ParseFS(fsys fs.FS, name string) (*Skill, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("skill: чтение %s не удалось: %w", name, err)
	}
	sk, err := ParseBytes(data, "builtin")
	if err != nil {
		return nil, fmt.Errorf("skill: разбор %s не удалось: %w", name, err)
	}
	return sk, nil
}

// ParseBytes 将一段 markdown 字节解析为 Skill。
// 可用于校验原始内容格式是否合法。
func ParseBytes(data []byte, defaultAuthor string) (*Skill, error) {
	fm, body, err := splitFrontmatter(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, err
	}

	sk, err := parseFrontmatter(fm)
	if err != nil {
		return nil, fmt.Errorf("не удалось разобрать YAML frontmatter: %w", err)
	}

	if sk.Name == "" {
		return nil, fmt.Errorf("отсутствует поле name")
	}
	if sk.Author == "" {
		sk.Author = defaultAuthor
	}
	sk.Mode = normalizeMode(sk.Mode)
	sk.Content = body
	sk.RawContent = string(data)
	return sk, nil
}

// splitFrontmatter 从 markdown 原文中分离 frontmatter 和正文。
// frontmatter 由开头的 --- 开始，下一个 --- 结束。
func splitFrontmatter(raw string) (frontmatter, body string, err error) {
	if !strings.HasPrefix(raw, "---") {
		return "", raw, nil
	}

	raw = raw[3:]
	frontmatter, body, found := strings.Cut(raw, "\n---")
	if !found {
		return "", "", fmt.Errorf("frontmatter не закрыт: отсутствует завершающий маркер ---")
	}
	body = strings.TrimSpace(body)
	return frontmatter, body, nil
}

// parseFrontmatter 将 YAML frontmatter 字符串解析为 Skill（不含 Content）。
func parseFrontmatter(raw string) (*Skill, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return &Skill{}, nil
	}

	var sk Skill
	if err := yaml.Unmarshal([]byte(raw), &sk); err != nil {
		return nil, fmt.Errorf("не удалось разобрать YAML: %w", err)
	}
	return &sk, nil
}

// scanFS 扫描 fs.FS 指定目录下的所有 .md 文件并解析为 Skill 切片。
func scanFS(logger *slog.Logger, fsys fs.FS, dir string) ([]Skill, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("skill: чтение каталога %s не удалось: %w", dir, err)
	}

	var skills []Skill
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		sk, err := ParseFS(fsys, dir+"/"+entry.Name())
		if err != nil {
			logger.Warn("skill: failed to parse builtin skill, skipped", "file", entry.Name(), "err", err)
			continue
		}
		skills = append(skills, *sk)
	}
	return skills, nil
}

// scanDir 扫描目录下所有 .md 文件并解析为 Skill 切片。
// YAML name 与文件名不一致时以 YAML name 为准重命名文件。
// 目录不存在时返回空切片（不报错）。
func scanDir(logger *slog.Logger, dir string) ([]Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("чтение каталога %s не удалось: %w", dir, err)
	}

	var skills []Skill
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		sk, err := ParseFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			logger.Warn("skill: failed to parse skill file, skipped", "file", entry.Name(), "err", err)
			continue
		}
		fileBase := strings.TrimSuffix(entry.Name(), ".md")
		if sk.Name != "" && sk.Name != fileBase {
			oldPath := filepath.Join(dir, entry.Name())
			newPath := filepath.Join(dir, sk.Name+".md")
			if err := os.Rename(oldPath, newPath); err != nil {
				return nil, fmt.Errorf("переименование файла скилла %s -> %s не удалось: %w", oldPath, newPath, err)
			}
		}
		skills = append(skills, *sk)
	}
	return skills, nil
}
