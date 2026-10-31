// queryKey conventions: plural = list, singular = single item; parent id as
// the second segment, child id as the third.
// All useQuery/mutation invalidation uses the constants from this file;
// raw string arrays are forbidden.
// See docs/feat/v1.4.0/refactor-plan/00-conventions.md

export const novelKeys = {
  all: ["novels"] as const,
  detail: (id: number) => ["novel", id] as const,
};

export const chapterKeys = {
  list: (novelId: number) => ["chapters", novelId] as const,
  detail: (filePath: string) => ["chapter", filePath] as const,
};

export const volumeKeys = {
  list: (novelId: number) => ["volumes", novelId] as const,
};

// content: GetContent(novelId, filePath) reads file content (chapter body /
// outline / goink.md / skill).
// Distinct from chapterKeys: chapterKeys is the chapter metadata list
// (GetChapters); contentKeys is the file content cache.
// 5.2 commit 1: useFileContent uses queryClient.fetchQuery with this key,
// sharing the cache across tabs.
export const contentKeys = {
  all: ["content"] as const,
  detail: (novelId: number, filePath: string) =>
    ["content", novelId, filePath] as const,
};

export const characterKeys = {
  list: (novelId: number) => ["characters", novelId] as const,
  groups: (novelId: number) => ["characters", novelId, "groups"] as const,
  memberships: (novelId: number) =>
    ["characters", novelId, "memberships"] as const,
  detail: (id: number) => ["character", id] as const,
  relations: (novelId: number) => ["character-relations", novelId] as const,
};

export const locationKeys = {
  list: (novelId: number) => ["locations", novelId] as const,
  detail: (id: number) => ["location", id] as const,
  relations: (novelId: number) => ["location-relations", novelId] as const,
};

export const storyarcKeys = {
  list: (novelId: number) => ["storyarcs", novelId] as const,
  detail: (id: number) => ["storyarc", id] as const,
};

// arc-nodes: backend GetArcNodes(novelId, fromChapter, toChapter) takes a
// chapter window as its 2nd/3rd args, not an arcId; there is no API to fetch
// by arcId, so the queryKey second segment is novelId (full cache; one
// invalidate refreshes everything).
export const arcNodeKeys = {
  list: (novelId: number) => ["arc-nodes", novelId] as const,
  detail: (id: number) => ["arc-node", id] as const,
};

// maxChapter: the novel's max chapter number (used for the storyarc chapter
// window center windowCenter).
export const maxChapterKeys = {
  detail: (novelId: number) => ["max-chapter", novelId] as const,
};

export const timelineKeys = {
  list: (novelId: number) => ["timeline", novelId] as const,
  detail: (id: number) => ["timeline-entry", id] as const,
};

// chapter-plans: chapter plan 3-slot (next/near/far), independent API
// GetChapterPlans(novelId).
export const chapterPlanKeys = {
  list: (novelId: number) => ["chapter-plans", novelId] as const,
};

export const readerKeys = {
  list: (novelId: number) => ["reader", novelId] as const,
  detail: (id: number) => ["reader-perspective", id] as const,
};

export const preferenceKeys = {
  list: (novelId: number) => ["preferences", novelId] as const,
  detail: (id: number) => ["preference", id] as const,
};

export const novelSettingKeys = {
  list: (novelId: number) => ["novel-settings", novelId] as const,
  detail: (id: number) => ["novel-setting", id] as const,
};

export const styleSampleKeys = {
  // all: prefix invalidation (invalidateQueries invalidates the list +
  // infiniteList caches).
  all: ["style-samples"] as const,
  // list: StyleView single-page pagination (page in key; page change
  // triggers a new query).
  // Distinct cache from infiniteList (StyleSampleList infinite scroll uses
  // infiniteList).
  list: (novelId: number, page: number, size: number, search: string) =>
    ["style-samples", novelId, page, size, search] as const,
  // infiniteList: StyleSampleList infinite scroll (page managed by
  // pageParam, not in key).
  // search change triggers a new query. Follows the sessionKeys.infiniteList
  // pattern.
  infiniteList: (novelId: number, size: number, search: string) =>
    ["style-samples", "infinite", novelId, size, search] as const,
  detail: (id: number) => ["style-sample", id] as const,
};

export const skillKeys = {
  // all: prefix invalidation (invalidateQueries invalidates list caches for
  // all novels).
  all: ["skills"] as const,
  // list: aligned with ListSkillsInput.novel_id to avoid cross-novel cache
  // collisions (novel-level skills differ per novel).
  list: (novelId: number) => ["skills", novelId] as const,
  detail: (name: string) => ["skill", name] as const,
  // remoteList: SkillMarketplace remote skill marketplace list (new apperr
  // API).
  // input contains page/size/query, included in key to avoid cache
  // collisions across pagination/search.
  remoteList: (input: { page: number; size: number; query: string }) =>
    ["remote-skills", input.page, input.size, input.query] as const,
  // remoteContent: SkillMarketplace remote skill content (detail phase).
  remoteContent: (name: string) => ["remote-skill-content", name] as const,
};

// chat domain GET endpoint query keys (5.1 commit 1).
// models/settings are global config, but the first consumer is ChatPanel, so
// the hook lives in components/chat/ for now; shared cache when the global
// config domain is migrated.
export const modelKeys = {
  all: ["models"] as const,
};

export const settingsKeys = {
  all: ["settings"] as const,
};

export const mcpServerKeys = {
  all: ["mcp-server"] as const,
};

// 5.8 commit 1: LLM config query key.
// GetLLMConfig returns LLMConfigView (with providers), semantically distinct
// from modelKeys (GetModels returns the model list), so a separate key.
// useSaveLLMConfig onSuccess invalidates this key so ModelConfigTab
// refetches automatically.
export const llmConfigKeys = {
  all: ["llm-config"] as const,
};

export const slashCommandKeys = {
  list: (novelId: number) => ["slash-commands", novelId] as const,
};

export const sessionMessagesKeys = {
  detail: (sessionId: string) => ["session-messages", sessionId] as const,
};

export const sessionKeys = {
  // list: single-page query (ChatPanel recent sessions page=1 size=5).
  // queryKey contains pagination/search params; distinct cache from
  // SessionHistory's infiniteList (different size/search = no sharing).
  list: (novelId: number, page: number, size: number, search: string) =>
    ["sessions", novelId, page, size, search] as const,
  // infiniteList: SessionHistory infinite scroll sequence (size=20 + search).
  // page is managed by useInfiniteQuery's pageParam, not in key; search
  // change triggers a new query.
  infiniteList: (novelId: number, size: number, search: string) =>
    ["sessions", "infinite", novelId, size, search] as const,
  // detail: session_id is a string (fixes the original number type bug).
  detail: (id: string) => ["session", id] as const,
};

// 5.4 commit 5: git domain query keys.
// git commit history uses cursor pagination (afterHash), not in key;
// commitFiles/fileDiff fetch by hash+filePath.
export const gitCommitKeys = {
  // infiniteList: GetCommitLog cursor pagination (GitHistoryList size=50).
  // pageParam is the afterHash string (git log's natural pagination), not in
  // key; size is in key to distinguish caches of different page sizes.
  infiniteList: (novelId: number, size: number) =>
    ["git-commits", novelId, size] as const,
};

export const commitFileKeys = {
  // list: GetCommitFileList(novelId, hash), fetches the file list on demand
  // when a commit is expanded.
  list: (novelId: number, hash: string) =>
    ["commit-files", novelId, hash] as const,
};

export const fileDiffKeys = {
  // detail: GetFileDiff(novelId, hash, filePath), fetches the diff on demand
  // when a file is selected.
  detail: (novelId: number, hash: string, filePath: string) =>
    ["file-diff", novelId, hash, filePath] as const,
};

// 5.5 commit 1: search domain query key.
// SearchAll(novelId, query) is a global cross-entity search. The query string
// is encoded in the key to drive refetch + let the query's built-in race
// protection take over (replacing the old reqIdRef manual race guard).
// staleTime=0 (search is a user-initiated operation expecting fresh results)
// + short gcTime (prevents a pile-up of cached entries per typed character).
export const searchKeys = {
  list: (novelId: number, query: string) => ["search", novelId, query] as const,
};

// 5.7 commit 1: profile domain query keys.
// writingActivity: GetWritingActivity(months) is the writing calendar for the
// past N months (green square data).
// writingStats: GetWritingStats() is global writing stats (no args, singleton
// cache).
// settings reuses settingsKeys.all (shared cache with chat; profile reads the
// user_name/avatar fields).
export const writingActivityKeys = {
  detail: (months: number) => ["writing-activity", months] as const,
};

export const writingStatsKeys = {
  all: ["writing-stats"] as const,
};
