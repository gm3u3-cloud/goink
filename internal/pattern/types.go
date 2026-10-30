package pattern

import "github.com/sigpanic/goink/internal/llm"

type ExtractPatternInput struct {
	TaskID          string  `json:"task_id,omitempty"`
	NovelID         int64   `json:"novel_id"`
	ProviderName    string  `json:"provider_name"`
	ModelID         string  `json:"model_id"`
	ReasoningEffort string  `json:"reasoning_effort"`
	ChapterIDs      []int64 `json:"chapter_ids,omitempty"`
}

type ExtractPatternResult struct {
	TaskID      string `json:"task_id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description"`
	RawContent  string `json:"raw_content"`
	FilePath    string `json:"file_path"`
	Trace       *Trace `json:"trace,omitempty"`
}

// Pipeline stage names
const (
	StageLoaded         = "loaded"
	StageBoundaries     = "boundaries"
	StageSummaries      = "summaries"
	StageInitialChunks  = "initial_chunks"
	StageCompressChunks = "compress_chunks"
	StageFinalizing     = "finalizing"
	StageDone           = "done"
)

// LLMStatus 是 llm.CallToolStatus 的别名，保持 pattern 包对外类型兼容。
type LLMStatus = llm.CallToolStatus

const (
	LLMThinking   = llm.CallToolThinking   // 模型正在推理/思考
	LLMGenerating = llm.CallToolGenerating // 模型正在输出结果
)

type Progress struct {
	TaskID     string               `json:"task_id,omitempty"`
	NovelID    int64                `json:"novel_id"`
	Stage      string               `json:"stage"`
	Message    string               `json:"message"`
	LLMStatus  LLMStatus            `json:"llm_status,omitempty"`
	Round      int                  `json:"round,omitempty"`
	BatchIndex int                  `json:"batch_index,omitempty"`
	BatchTotal int                  `json:"batch_total,omitempty"`
	Tokens     int                  `json:"tokens,omitempty"`
	Boundaries []BoundaryHint       `json:"boundaries,omitempty"`
	Summaries  []ChapterSummaryItem `json:"summaries,omitempty"`
	Chunks     []Chunk              `json:"chunks,omitempty"`
}

type Trace struct {
	TaskID        string               `json:"task_id,omitempty"`
	NovelID       int64                `json:"novel_id"`
	ChapterCount  int                  `json:"chapter_count"`
	ContextWindow int                  `json:"context_window"`
	BatchBudget   int                  `json:"batch_budget"`
	Boundaries    []BoundaryHint       `json:"boundaries"`
	Summaries     []ChapterSummaryItem `json:"summaries"`
	ChunkRounds   []ChunkRoundTrace    `json:"chunk_rounds"`
	FinalTokens   int                  `json:"final_tokens"`
}

type ChunkRoundTrace struct {
	Round       int          `json:"round"`
	InputCount  int          `json:"input_count"`
	OutputCount int          `json:"output_count"`
	TokenCount  int          `json:"token_count"`
	Batches     []BatchTrace `json:"batches"`
	Chunks      []Chunk      `json:"chunks"`
}

type BatchTrace struct {
	Index        int `json:"index"`
	InputCount   int `json:"input_count"`
	OutputCount  int `json:"output_count"`
	ApproxTokens int `json:"approx_tokens"`
}

type BoundaryHint struct {
	StartChapterID     int64  `json:"start_chapter_id" jsonschema:"required,description=The stable ID of the start chapter"`
	EndChapterID       int64  `json:"end_chapter_id" jsonschema:"required,description=The stable ID of the end chapter"`
	StartReadingNumber int    `json:"-"`
	EndReadingNumber   int    `json:"-"`
	Hint               string `json:"hint" jsonschema:"required,description=Why this may be a narrative stage boundary"`
}

type BoundaryHintsOutput struct {
	Boundaries []BoundaryHint `json:"boundaries" jsonschema:"required,description=List of possible narrative stage boundaries"`
}

type ChapterSummaryItem struct {
	ChapterID     int64  `json:"chapter_id" jsonschema:"required,description=The stable ID of the chapter"`
	ReadingNumber int    `json:"-"`
	Summary       string `json:"summary" jsonschema:"required,description=An 80-150 character narrative summary of the chapter, covering core events, character actions, and turning points"`
}

type ChapterSummariesOutput struct {
	Summaries []ChapterSummaryItem `json:"summaries" jsonschema:"required,description=List of generated chapter summaries"`
}

type Chunk struct {
	Name               string `json:"name" jsonschema:"required,description=The narrative stage name (e.g. rise, turning point)"`
	StartChapterID     int64  `json:"start_chapter_id" jsonschema:"required,description=The stable ID of the start chapter"`
	EndChapterID       int64  `json:"end_chapter_id" jsonschema:"required,description=The stable ID of the end chapter"`
	StartReadingNumber int    `json:"-"`
	EndReadingNumber   int    `json:"-"`
	Content            string `json:"content" jsonschema:"required,description=A 100-200 character narrative summary of the stage, covering core events and turning points"`
}

type ChunksOutput struct {
	Chunks []Chunk `json:"chunks" jsonschema:"required,description=List of compressed narrative stage chunks"`
}

type SkillOutput struct {
	Name        string `json:"name" jsonschema:"required,description=The skill name (Chinese pattern name)"`
	Description string `json:"description" jsonschema:"required,description=A one-sentence description of when to use this narrative pattern"`
	Content     string `json:"content" jsonschema:"required,description=The skill body markdown, containing sections such as ## Pattern Overview / Stage Breakdown / Payoff Rhythm / Character Function Templates / Reusable Narrative Rules / Usage Notes, without frontmatter"`
}

// ChapterSource 是从 DB 元数据和 git 正文拼出的章节数据传递对象
type ChapterSource struct {
	ID            int64
	ReadingNumber int
	Title         string
	Summary       string
	Content       string
}
