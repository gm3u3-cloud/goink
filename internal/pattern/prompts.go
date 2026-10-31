package pattern

import (
	"fmt"
	"slices"
	"strings"
)

func boundaryMessages(chapters []ChapterSource) []map[string]any {
	var b strings.Builder
	for _, ch := range chapters {
		title := strings.TrimSpace(ch.Title)
		if title == "" {
			title = fmt.Sprintf("Chapter %d (untitled)", ch.ReadingNumber)
		}
		fmt.Fprintf(&b, "Chapter %d (chapter_id: %d): %s\n", ch.ReadingNumber, ch.ID, title)
	}
	return []map[string]any{
		{"role": "system", "content": "You are a structural analyst for long-form novels. Infer likely narrative stage boundaries from the chapter titles. Chapter numbers are only used to understand reading order; when returning boundary endpoints you must use the corresponding chapter_id. You may only return results by calling output_boundary_hints."},
		{"role": "user", "content": "Find the possible narrative stage boundaries. Boundaries are reference clues, not final conclusions. Keep each hint concise.\n\n" + b.String()},
	}
}

func summaryMessages(chapters []ChapterSource, boundaries []BoundaryHint) []map[string]any {
	var b strings.Builder
	if len(boundaries) > 0 {
		b.WriteString("Suspected stage boundary references:\n")
		for _, h := range boundaries {
			fmt.Fprintf(&b, "- Chapters %d-%d (chapter_id: %d-%d): %s\n", h.StartReadingNumber, h.EndReadingNumber, h.StartChapterID, h.EndChapterID, h.Hint)
		}
		b.WriteString("\n")
	}
	b.WriteString("Chapter list:\n")
	for _, ch := range chapters {
		title := strings.TrimSpace(ch.Title)
		if title == "" {
			title = fmt.Sprintf("Chapter %d (untitled)", ch.ReadingNumber)
		}
		fmt.Fprintf(&b, "\n## Chapter %d (chapter_id: %d): %s\n", ch.ReadingNumber, ch.ID, title)
		if strings.TrimSpace(ch.Summary) != "" {
			fmt.Fprintf(&b, "[Context reference, no need to regenerate]\n%s\n", strings.TrimSpace(ch.Summary))
			continue
		}
		if strings.TrimSpace(ch.Content) == "" {
			b.WriteString("[Needs extraction]\nNo content.\n")
			continue
		}
		fmt.Fprintf(&b, "[Needs extraction]\n%s\n", ch.Content)
	}
	return []map[string]any{
		{"role": "system", "content": "You generate chapter summaries for narrative pattern extraction. Chapters marked [Needs extraction] each need one narrative summary of 80-150 characters, covering core events, key character actions and decisions, and emotional or relational turning points. Preserve enough detail to support later determination of narrative stage boundaries. Chapters marked [Context reference] serve only as narrative background; unless also marked [Needs extraction], do not include them in the output. Chapter numbers are only used to understand reading order; when returning summaries you must use the corresponding chapter_id. You may only return results by calling output_chapter_summaries."},
		{"role": "user", "content": b.String()},
	}
}

func initialChunkMessages(summaries []ChapterSummaryItem) []map[string]any {
	var b strings.Builder
	for _, summary := range summaries {
		fmt.Fprintf(&b, "Chapter %d (chapter_id: %d): %s\n", summary.ReadingNumber, summary.ChapterID, summary.Summary)
	}
	return []map[string]any{
		{"role": "system", "content": "You compress chapter-level summaries into narrative stage chunks. Use narrative stage transitions as the dividing standard: when a summary shows a clear change in narrative direction, emotional tone, or core conflict, a boundary should be drawn there. Adjacent chapters belonging to the same narrative stage should be merged into one chunk. Each chunk's content should summarize the core events, key character actions, and turning points of that stage, in about 100-200 characters. Chapter numbers are only used to understand reading order; when returning chunk endpoints you must use the corresponding start_chapter_id and end_chapter_id. You may only return results by calling output_chunks."},
		{"role": "user", "content": "Generate the first round of narrative stage chunks from the following chapter summaries:\n\n" + b.String()},
	}
}

func compressChunkMessages(chunks []Chunk, readingNumbers map[int64]int, round int) []map[string]any {
	var b strings.Builder
	for _, chunk := range chunks {
		fmt.Fprintf(&b, "Chapters %d-%d (chapter_id: %d-%d) %s: %s\n", chunk.StartReadingNumber, chunk.EndReadingNumber, chunk.StartChapterID, chunk.EndChapterID, chunk.Name, chunk.Content)
	}
	references := chapterReferences(chunks, readingNumbers)
	return []map[string]any{
		{"role": "system", "content": "You merge multiple narrative stage chunks into fewer, larger stage chunks. Follow these rules:\n1. Prefer merging adjacent chunks that belong to the same narrative stage (consistent narrative direction, same emotional tone).\n2. If a chunk internally contains two clearly different narrative sub-stages (e.g. setup → eruption), it may be split, but splitting should be minimized.\n3. Each output chunk's content should re-summarize the core events and turning points of the merged stage, in about 100-200 characters.\n4. The accurate start_chapter_id and end_chapter_id must be preserved; chapter numbers are only used to understand reading order. The available chapter reference table lists all IDs that can serve as endpoints. You may only return results by calling output_chunks."},
		{"role": "user", "content": fmt.Sprintf("Round %d compression. Merge the following stage chunks into fewer narrative stage chunks:\n\n%s\nAvailable chapter reference table:\n%s", round, b.String(), references)},
	}
}

func chapterReferences(chunks []Chunk, readingNumbers map[int64]int) string {
	if len(chunks) == 0 || len(readingNumbers) == 0 {
		return ""
	}
	type chapterReference struct {
		id     int64
		number int
	}
	refs := make([]chapterReference, 0, len(readingNumbers))
	for id, number := range readingNumbers {
		refs = append(refs, chapterReference{id: id, number: number})
	}
	slices.SortFunc(refs, func(a, b chapterReference) int { return a.number - b.number })
	var b strings.Builder
	for _, ref := range refs {
		fmt.Fprintf(&b, "Chapter %d = %d\n", ref.number, ref.id)
	}
	return b.String()
}

func finalSkillMessages(chunks []Chunk) []map[string]any {
	var b strings.Builder
	for _, chunk := range chunks {
		fmt.Fprintf(&b, "Chapters %d-%d (chapter_id: %d-%d) %s: %s\n", chunk.StartReadingNumber, chunk.EndReadingNumber, chunk.StartChapterID, chunk.EndChapterID, chunk.Name, chunk.Content)
	}
	return []map[string]any{
		{"role": "system", "content": finalSkillSystemPrompt},
		{"role": "user", "content": "Generate the final reusable narrative pattern skill from the following narrative stage chunks:\n\n" + b.String()},
	}
}

const finalSkillSystemPrompt = `You are a professional novel structure analyst.

Based on the provided narrative stage chunks, create a reusable narrative pattern skill (a template). Do not merely describe the original book's content; abstract a reusable template.

You may only return results by calling the output_skill tool. Tool parameter requirements:
- name: the pattern name in Russian (e.g. a "comeback face-slap" style).
- description: a one-sentence description of when to use this narrative pattern.
- content: the skill body markdown, without frontmatter (frontmatter is auto-generated by the system). It must include the following sections:

  ## Pattern Overview
  Summarize the reusable narrative structure.
  ## Stage Breakdown
  List each stage, including the source chapter range, structural function, common signals, and how to reuse it.
  ## Payoff Rhythm
  Explain the distribution of suppression, release, revelation, setbacks, and climax.
  ## Character Function Templates
  Abstract the functional roles of the protagonist, ally, rival, antagonist, mentor, utility character, and sacrificial/contrast character.
  ## Reusable Narrative Rules
  Provide concrete writing rules that can guide the creation of new works.
  ## Usage Notes
  Explain which parts must be adapted to avoid the finished work looking derivative.

Reiterating: the content should abstract a reusable template, not merely describe the original book's content; the content must not include YAML frontmatter.`
