package mcp_tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const maxCatalogSchemaBytes = 64 * 1024

var catalogSchemaPathRe = regexp.MustCompile(`^mcp/schemas/([A-Za-z0-9_-]+)\.json$`)

// readCatalog 从当前注册表生成只读内容，不读取或缓存磁盘文件。
func (r *Registry) readCatalog(path string) (content string, schema bool, err error) {
	if r == nil {
		return "", false, errors.New("工具注册表未初始化")
	}
	if path == "mcp/index.md" {
		tools := r.List()
		sort.Slice(tools, func(i, j int) bool { return tools[i].Name() < tools[j].Name() })
		var output strings.Builder
		output.WriteString("# 拓展工具\n\n先 read 对应 schema，再用 run_mcp 的 tool_name 和 args 调用。")
		count := 0
		for _, tool := range tools {
			if tool.ExposeToLLM() {
				continue
			}
			description, _, _ := strings.Cut(strings.TrimSpace(tool.Description()), "\n")
			description = strings.Join(strings.Fields(description), " ")
			if runes := []rune(description); len(runes) > 120 {
				description = string(runes[:120]) + "…"
			}
			fmt.Fprintf(&output, "\n- `%s` — %s；schema：`mcp/schemas/%s.json`", tool.Name(), description, tool.Name())
			count++
		}
		if count == 0 {
			output.WriteString("\n暂无可用的拓展工具。")
		}
		return output.String(), false, nil
	}
	match := catalogSchemaPathRe.FindStringSubmatch(path)
	if match == nil {
		return "", false, businessErrorf("无效的工具目录路径，请使用 mcp/index.md 或 mcp/schemas/<tool_name>.json")
	}
	tool, ok := r.Get(match[1])
	if !ok || tool.ExposeToLLM() {
		return "", false, businessErrorf("拓展工具不存在: %s", match[1])
	}
	data, err := json.Marshal(struct {
		ToolInfo
		InputSchema json.RawMessage `json:"inputSchema"`
	}{
		ToolInfo: ToolInfo{
			Name: tool.Name(), Description: tool.Description(), Category: string(tool.Category()),
		},
		InputSchema: tool.JSONSchema(),
	})
	if err != nil {
		return "", false, fmt.Errorf("编码工具 %s 的 schema: %w", tool.Name(), err)
	}
	if len(data) > maxCatalogSchemaBytes {
		return "", false, businessErrorf("工具 schema 超过大小上限 %d 字节（实际 %d 字节）", maxCatalogSchemaBytes, len(data))
	}
	return string(data), true, nil
}
