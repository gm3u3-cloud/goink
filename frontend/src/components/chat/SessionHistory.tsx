import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { MessageSquare, Loader2, History, Trash2 } from "lucide-react";
import { useInView } from "react-intersection-observer";
import { useTimeAgo } from "@/hooks/useTimeAgo";
import { useInfiniteSessions } from "./useInfiniteSessions";
import { useChatStore } from "./useChatStore";

interface Props {
  open: boolean;
  novelId: number;
  onClose: () => void;
  onSelectSession: (sessionId: string) => void;
}

export default function SessionHistory({
  open,
  novelId,
  onClose,
  onSelectSession,
}: Props) {
  const { t } = useTranslation();
  const [mounted, setMounted] = useState(false);
  // Relative time: auto-refreshes every minute while the panel is open; the
  // timer is stopped when it closes
  const timeAgo = useTimeAgo(open);
  const [visible, setVisible] = useState(false);
  const [search, setSearch] = useState("");
  const [submittedSearch, setSubmittedSearch] = useState("");
  // Clicking delete only dispatches to the store; the ConfirmDialog +
  // execution are handled centrally by DeleteSessionDialog
  const setDeletingSession = useChatStore((s) => s.setDeletingSession);
  // Infinite scroll: fetch the next page when the sentinel enters the
  // viewport (react-intersection-observer useInView)
  const { ref: sentinelRef, inView } = useInView({ rootMargin: "100px" });

  // Infinite scroll query: page is managed by pageParam (not in queryKey);
  // submittedSearch changes → queryKey changes → auto-refetch from the
  // first page.
  // enabled: open && !!novelId (no fetch while the panel is closed).
  const sessionsQuery = useInfiniteSessions({
    novelId,
    size: 20,
    search: submittedSearch,
    enabled: open,
  });

  const sessions =
    sessionsQuery.data?.pages.flatMap((p) => p.items ?? []) ?? [];
  const total = sessionsQuery.data?.pages[0]?.total ?? 0;
  const hasMore = sessionsQuery.hasNextPage;
  const isLoading = sessionsQuery.isLoading;
  const isFetchingMore = sessionsQuery.isFetchingNextPage;

  useEffect(() => {
    if (open) {
      setMounted(true);
      requestAnimationFrame(() => setVisible(true));
    } else {
      setVisible(false);
      const timer = setTimeout(() => setMounted(false), 200);
      return () => clearTimeout(timer);
    }
  }, [open]);

  // Search debounce 300ms: search input → submittedSearch updates →
  // queryKey changes → refetch
  useEffect(() => {
    const timer = setTimeout(() => {
      setSubmittedSearch(search);
    }, 300);
    return () => clearTimeout(timer);
  }, [search]);

  // Infinite scroll: fetch the next page when the sentinel enters the
  // viewport (replaces the old hand-written scroll event + distance check)
  useEffect(() => {
    if (inView && hasMore && !isFetchingMore) {
      sessionsQuery.fetchNextPage();
    }
  }, [inView, hasMore, isFetchingMore, sessionsQuery]);

  if (!mounted) return null;

  return (
    <div className="absolute inset-0 pointer-events-none">
      <div
        className="absolute inset-0 z-30 pointer-events-auto"
        onClick={onClose}
      />
      <div
        className={`absolute right-3 left-3 z-40 flex flex-col bg-card border rounded-xl shadow-lg pointer-events-auto transition-all duration-200 ease-out overflow-hidden ${visible ? "opacity-100 translate-y-0" : "opacity-0 -translate-y-2"}`}
        style={{ height: "40%", top: "4px" }}
      >
        <div className="flex items-center justify-between px-4 py-2 border-b shrink-0">
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2">
              <History className="w-4 h-4 text-muted-foreground" />
              <span className="text-xs font-medium">
                {t("chat.historySessions")}
              </span>
            </div>
            {total > 0 && (
              <span className="text-[10px] text-muted-foreground">
                {t("chat.totalSessions", { count: total })}
              </span>
            )}
          </div>
        </div>

        <div className="px-4 py-2 shrink-0">
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("chat.searchSessions")}
            className="w-full h-7 rounded-md border bg-muted/30 px-2.5 text-xs"
          />
        </div>

        {/* Session list */}
        <div className="flex-1 overflow-y-auto overscroll-contain px-3 pb-2">
          {sessions.length === 0 && isLoading ? (
            <div className="flex items-center justify-center h-full">
              <Loader2 className="w-4 h-4 animate-spin text-muted-foreground" />
            </div>
          ) : sessions.length === 0 && submittedSearch ? (
            <div className="flex items-center justify-center h-full">
              <span className="text-xs text-muted-foreground">
                {t("chat.noMatchingSessions")}
              </span>
            </div>
          ) : (
            <div className="space-y-0.5">
              {sessions.map((s) => (
                <div
                  key={s.session_id}
                  onClick={() => {
                    onSelectSession(s.session_id);
                    onClose();
                  }}
                  className="w-full flex items-center gap-2.5 px-2.5 py-2.5 rounded-lg text-left hover:bg-muted/50 transition-colors cursor-pointer select-none group"
                >
                  <MessageSquare className="w-4 h-4 shrink-0 text-muted-foreground" />
                  <div className="min-w-0 flex-1">
                    <div className="text-xs truncate">
                      {s.title || t("chat.newChat")}
                    </div>
                    <div className="text-[10px] text-muted-foreground mt-0.5">
                      {timeAgo(s.updated_at)}
                    </div>
                  </div>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      setDeletingSession(s);
                    }}
                    className="shrink-0 p-1 rounded text-muted-foreground hover:text-destructive opacity-0 group-hover:opacity-100 transition-opacity"
                    title={t("common.delete")}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </div>
              ))}
              {isFetchingMore && (
                <div className="flex justify-center py-3">
                  <Loader2 className="w-4 h-4 animate-spin text-muted-foreground" />
                </div>
              )}
              {!hasMore && sessions.length > 0 && (
                <div className="text-center text-[10px] text-muted-foreground py-2">
                  {t("chat.allSessionsShown")}
                </div>
              )}
              {/* Infinite scroll sentinel: triggers fetchNextPage when it
               * enters the viewport */}
              <div ref={sentinelRef} className="h-4" />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
