package mcp_tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const maxCatalogSchemaBytes = 64 * 1024

var catalogSchemaPathRe = regexp.MustCompile(`^mcp/schemas/([A-Za-z0-9_-]+)\.json$`)
var catalogSearchTermsRe = regexp.MustCompile(`[\p{Han}]+|[a-z0-9]+`)

// readCatalog 从当前注册表生成只读内容，不读取或缓存磁盘文件。
func (r *Registry) readCatalog(path string) (content string, schema bool, err error) {
	if r == nil {
		return "", false, errors.New("реестр инструментов не инициализирован")
	}
	if path == "mcp/index.md" {
		tools := r.List()
		sort.Slice(tools, func(i, j int) bool { return tools[i].Name() < tools[j].Name() })
		var output strings.Builder
		output.WriteString("# Extension Tools\n\nFirst read the corresponding schema, then invoke it via run_mcp using tool_name and args.")
		count := 0
		for _, tool := range tools {
			if tool.ExposeToLLM() {
				continue
			}
			writeCatalogEntry(&output, tool)
			count++
		}
		if count == 0 {
			output.WriteString("\nNo extension tools available.")
		}
		return output.String(), false, nil
	}
	match := catalogSchemaPathRe.FindStringSubmatch(path)
	if match == nil {
		return "", false, businessErrorf("неверный путь к каталогу инструментов, используйте mcp/index.md или mcp/schemas/<tool_name>.json")
	}
	tool, ok := r.Get(match[1])
	if !ok || tool.ExposeToLLM() {
		return "", false, businessErrorf("инструмент не существует: %s", match[1])
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
		return "", false, fmt.Errorf("ошибка кодирования schema инструмента %s: %w", tool.Name(), err)
	}
	if len(data) > maxCatalogSchemaBytes {
		return "", false, businessErrorf("schema инструмента превышает ограничение по размеру %d байт (фактически %d байт)", maxCatalogSchemaBytes, len(data))
	}
	return string(data), true, nil
}

func writeCatalogEntry(output *strings.Builder, tool Tool) {
	description, _, _ := strings.Cut(strings.TrimSpace(tool.Description()), "\n")
	description = strings.Join(strings.Fields(description), " ")
	if runes := []rune(description); len(runes) > 120 {
		description = string(runes[:120]) + "…"
	}
	fmt.Fprintf(output, "\n- `%s` — %s; schema: `mcp/schemas/%s.json`", tool.Name(), description, tool.Name())
}

// searchCatalog 以多条查询的最高得分排序，重复近义表达不会累加排名。
func (r *Registry) searchCatalog(queries []string) []Tool {
	type match struct {
		tool  Tool
		score int
	}
	var matches []match
	for _, tool := range r.List() {
		if tool.ExposeToLLM() {
			continue
		}
		name := normalizeCatalogText(tool.Name())
		description := normalizeCatalogText(tool.Description())
		nameTerms, descriptionTerms := catalogSearchTerms(name), catalogSearchTerms(description)
		best := 0
		for _, query := range queries {
			query = normalizeCatalogText(query)
			score := 0
			for term := range catalogSearchTerms(query) {
				if nameTerms[term] {
					score += 6
				}
				if descriptionTerms[term] {
					score += 2
				}
			}
			if score > 0 {
				if catalogContainsPhrase(name, query) {
					score += 12
				}
				if catalogContainsPhrase(description, query) {
					score += 4
				}
			}
			best = max(best, score)
		}
		if best > 0 {
			matches = append(matches, match{tool: tool, score: best})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].tool.Name() < matches[j].tool.Name()
	})
	results := make([]Tool, 0, min(len(matches), 5))
	for _, match := range matches[:min(len(matches), 5)] {
		results = append(results, match.tool)
	}
	return results
}

func normalizeCatalogText(text string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}), " ")
}

func catalogContainsPhrase(text, query string) bool {
	for _, r := range query {
		if unicode.Is(unicode.Han, r) {
			return strings.Contains(text, query)
		}
	}
	return strings.Contains(" "+text+" ", " "+query+" ")
}

func catalogSearchTerms(text string) map[string]bool {
	terms := map[string]bool{}
	for _, word := range catalogSearchTermsRe.FindAllString(text, -1) {
		runes := []rune(word)
		if !unicode.Is(unicode.Han, runes[0]) {
			terms[word] = true
			continue
		}
		for i := 0; i+1 < len(runes); i++ {
			terms[string(runes[i:i+2])] = true
		}
	}
	return terms
}
