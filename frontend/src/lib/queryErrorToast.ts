import type { QueryClient } from "@tanstack/react-query";
import { toastError } from "@/utils/toast";
import { toErrorMessage } from "@/utils/error";
import i18n from "@/i18n";

// queryKey prefix → i18n key mapping table.
// Add entries here as more domains are query-ified: style-samples / skills /
// chapters, etc.
// When a mapping is missing, fall back to `${prefix}.loadFailed`, and if that
// i18n key does not exist, fall back to `${prefix} load failed`, so errors
// are never silent.
const QUERY_ERROR_I18N: Record<string, string> = {
  characters: "character.charsLoadFailed",
  "character-relations": "character.relationsLoadFailed",
  novels: "novel.loadFailed",
  // Add entries here as more domains are query-ified:
  locations: "location.locationsLoadFailed",
  "location-relations": "location.relationsLoadFailed",
  storyarcs: "storyarc.arcsLoadFailed",
  "arc-nodes": "storyarc.nodesLoadFailed",
  "max-chapter": "storyarc.maxChapterLoadFailed",
  timeline: "timeline.loadFailed",
  "chapter-plans": "timeline.chapterPlansLoadFailed",
  reader: "reader.loadFailed",
  preferences: "preference.loadFailed",
  "novel-settings": "novelSetting.loadFailed",
  // chat domain GET endpoints (5.1 commit 1):
  models: "chat.modelsLoadFailed",
  settings: "chat.settingsLoadFailed",
  sessions: "chat.sessionsLoadFailed",
  session: "chat.sessionLoadFailed",
  "session-messages": "chat.messagesLoadFailed",
  "slash-commands": "chat.slashCommandsLoadFailed",
  // 5.2 commit 1: content domain GET endpoints
  chapters: "chapter.loadFailed",
  content: "content.loadFailed",
  // 5.3 commit 1: style domain GET endpoints (list + infiniteList share the
  // style-samples prefix; detail uses style-sample)
  "style-samples": "styleSample.loadFailed",
  "style-sample": "styleSample.loadFailed",
  // 5.4 commit 1: skill domain GET endpoints (ListSkills)
  skills: "skill.loadFailed",
  // 5.4 commit 3: SkillMarketplace remote skills (new apperr API;
  // unwrapResult throw triggers the middleware)
  "remote-skills": "skill.marketplace.loadFailed",
  "remote-skill-content": "skill.marketplace.contentLoadFailed",
  // 5.4 commit 5: git domain GET endpoints
  // (GetCommitLog/GetCommitFileList/GetFileDiff)
  "git-commits": "git.commitsLoadFailed",
  "commit-files": "git.commitFilesLoadFailed",
  "file-diff": "git.fileDiffLoadFailed",
  // 5.5 commit 1: search domain GET endpoints (SearchAll global
  // cross-entity search)
  search: "search.loadFailed",
  // 5.7 commit 1: profile domain GET endpoints
  // (GetWritingActivity/GetWritingStats)
  // settings reuses chat.settingsLoadFailed (GetSettings shared cache,
  // generic wording)
  "writing-activity": "profile.activityLoadFailed",
  "writing-stats": "profile.statsLoadFailed",
  // 5.8 commit 1: LLM config query (GetLLMConfig)
  "llm-config": "settings.llmConfigLoadFailed",
  "mcp-server": "settings.mcpServer.loadFailed",
};

// 4a global query error toast middleware.
// Design rationale: docs/feat/v1.4.0/refactor-plan/04a-query-error-toast.md.
//
// Triggers: query state becomes error (including after all retries fail,
// refetch failure, refetch failure after invalidateQueries).
// Does not trigger: during retry (state.status='pending'), successful
// refetch (action.type='success').
// Not wired in: mutation errors (mutationCache is not subscribed by this
// middleware); mutations are handled by their own onError.
//
// No dedup needed: subscribe is a query-level callback; multiple components
// subscribing to the same queryKey still fire only once; no error action
// fires during retry; TanStack Query creates a new error object reference on
// refetch failure, so the reference always changes.
//
// Silent on component unmount: when observers.length === 0 the query has no
// component subscribers (the user has left the page); a background refetch
// failure does not toast, so the user does not suddenly see an error from a
// page they have already left.
export function installQueryErrorToast(queryClient: QueryClient): () => void {
  return queryClient.getQueryCache().subscribe((event) => {
    // Only care about events where the query state became error
    if (event.type !== "updated") return;
    if (event.action.type !== "error") return;

    const query = event.query;
    const error = query.state.error;
    if (!error) return;

    // Silent on component unmount: no observer subscriptions = no toast for
    // background refetch failures
    if (query.observers.length === 0) return;

    // Look up the i18n key: check the mapping table by queryKey prefix first;
    // fall back to `${prefix}.loadFailed` when no match
    const prefix = String(query.queryKey[0] ?? "");
    const i18nKey = QUERY_ERROR_I18N[prefix] ?? `${prefix}.loadFailed`;
    const label = i18n.exists(i18nKey)
      ? i18n.t(i18nKey)
      : `${prefix} load failed`;

    toastError(`${label}: ${toErrorMessage(error)}`);
    console.error(`[query error] ${prefix}:`, error);
  });
}
