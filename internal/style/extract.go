package style

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sigpanic/goink/internal/llm"
	"github.com/sigpanic/goink/internal/mcp_tools"
	"github.com/sigpanic/goink/internal/skill"
)

// Extract 分析样本文字的写作风格，生成仿写 skill。
// 取消由调用方（app 层）通过 ctx 管理。
func Extract(ctx context.Context, llmClient *llm.Client,
	samples []Sample, providerName, modelID, reasoningEffort string) (*ExtractResult, error) {

	// 计算 stats + 拼接全文
	stats := ComputeStats(samples)
	var combined strings.Builder
	combined.WriteString(formatStatsForLLM(stats))
	for _, sample := range samples {
		combined.WriteString("\n\n---\n\n")
		combined.WriteString(sample.Content)
	}

	// 组装工具：output_style_skill 返回结构化的风格仿写技能
	tools := []map[string]any{{
		"type": "function",
		"function": map[string]any{
			"name":        "output_style_skill",
			"description": "Returns the structured output of the style-imitation skill.",
			"parameters":  mcp_tools.SchemaOf(StyleSkillOutput{}),
		},
	}}

	opts := &llm.CallToolOptions{
		Attempts:  2,
		MaxTokens: 64000,
	}
	if reasoningEffort != "" {
		opts.ReasoningEffort = reasoningEffort
	}

	raw, err := llmClient.CallTool(ctx, providerName, modelID,
		buildExtractMessages(combined.String()), tools, "output_style_skill", opts)
	if err != nil {
		return nil, fmt.Errorf("Ошибка вызова LLM: %w", err)
	}

	var out StyleSkillOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("Не удалось разобрать навык стиля: %w", err)
	}
	if strings.TrimSpace(out.Name) == "" || strings.TrimSpace(out.Content) == "" {
		return nil, fmt.Errorf("Неполное содержимое навыка стиля, возвращённое LLM")
	}

	safeName := skill.SanitizeFileName(out.Name)
	return &ExtractResult{
		Name:        out.Name,
		Description: out.Description,
		RawContent:  buildStyleSkillMarkdown(out.Name, out.Description, out.Content),
		FilePath:    fmt.Sprintf("~/.goink/skills/%s.md", safeName),
	}, nil
}

// buildStyleSkillMarkdown 组装完整的风格仿写 skill markdown：固定 frontmatter + 正文。
func buildStyleSkillMarkdown(name, description, content string) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "description: %s\n", description)
	b.WriteString("category: style-imitation\n")
	b.WriteString("mode: auto\n")
	b.WriteString("author: ai\n")
	b.WriteString("version: 1\n")
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimSpace(content))
	b.WriteString("\n")
	return b.String()
}

func buildExtractMessages(sample string) []map[string]any {
	return []map[string]any{
		{"role": "system", "content": extractSystemPrompt},
		{"role": "user", "content": fmt.Sprintf("The text begins with a 【Text statistics】block (deterministic measurements), followed by the original sample. Analyze the style based on the statistics and submit the result by calling the output_style_skill tool:\n\n%s", sample)},
	}
}

// formatStatsForLLM 将 Stats 格式化为中文统计文本供 LLM 参考。
func formatStatsForLLM(stats Stats) string {
	return fmt.Sprintf(`【Text statistics】
Total words: %d  Total characters: %d
Sentences: %d  Average sentence length: %.1f chars  Sentence length std dev: %.1f
Sentence length distribution: short (<15 chars) %.1f%%  medium (15-30 chars) %.1f%%  long (>30 chars) %.1f%%
Punctuation density: comma %.2f%%  period %.2f%%  exclamation %.2f%%  question mark %.2f%%  quote %.2f%%
Paragraphs: %d  Average paragraph length: %.1f chars
`,
		stats.TotalWords, stats.TotalChars,
		stats.SentenceCount, stats.AvgSentLen, stats.SentLenStdDev,
		stats.ShortSentPct, stats.MidSentPct, stats.LongSentPct,
		stats.CommaDensity, stats.PeriodDensity, stats.ExclaimDensity, stats.QuestionDensity, stats.QuoteDensity,
		stats.ParagraphCount, stats.AvgParaLen)
}

const extractSystemPrompt = `You are a professional writing style analyst. The text begins with a 【Text statistics】block — sentence length distribution, standard deviation, and punctuation density are all computed deterministically in code; cite these numbers directly as the basis of your analysis.

Deconstruct the writing style along the following six dimensions:

1. **Sentence patterns**: Based on the sentence length distribution and standard deviation in the statistics, analyze the mixing pattern of short and long sentences and the rhythmic characteristics
2. **Word choice**: vocabulary size, colloquial vs. formal register, types of frequent words
3. **Rhetorical devices**: commonly used rhetorical types and their frequency
4. **Rhythm control**: Based on the punctuation density data, analyze paragraph organization and the rhythm of sentence breaks
5. **Narrative perspective and distance**: choice of person, the sense of distance between the narrator and the content
6. **Atmosphere and tone**: emotional base tone, warmth of the language

Submit your analysis by calling the output_style_skill tool. Requirements for each field:
- name: Give this style a fitting name in Chinese
- description: A one-sentence description of which writing scenarios it suits and when to invoke it
- content: The body markdown of the style-imitation skill (no frontmatter), containing the following sections:
# {style name}

## Style Overview
Summarize the overall characteristics in one sentence.

## Sentence Patterns
Based on the sentence length distribution and standard deviation in the statistics, analyze the sentence pattern characteristics.

## Word Choice
Analyze word preferences and lexical choice tendencies in detail.

## Rhetorical Devices
Analyze the rhetorical devices used and their characteristics in detail.

## Rhythm Control
Based on the punctuation density data, analyze paragraph organization and the rhythm of sentence breaks.

## Narrative Perspective and Distance
Analyze the narrator's position and the sense of distance from the content in detail.

## Atmosphere and Tone
Analyze the emotional base tone and warmth of the language in detail.

## Imitation Key Points
Distill 3-5 actionable imitation guidelines; each one should be concrete and executable.

## Original Anchors
Pick 4-6 passages from the original text that best embody this style as imitation examples. Each passage is 80-150 characters, quoted verbatim from the original, with a note on what technique the passage demonstrates. Format:

### Anchor N
> [original passage]

Demonstration: [one sentence explaining what technique this passage demonstrates and how to apply it when imitating]`
