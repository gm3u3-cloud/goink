import { useState, useEffect } from "react";
import { Loader2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useInView } from "react-intersection-observer";
import SearchInput from "@/components/shared/SearchInput";
import { useInfiniteStyleSamples } from "./useInfiniteStyleSamples";

const PAGE_SIZE = 50;

interface Props {
  onSelectSample: (id: number) => void;
  activeId?: number | null;
  novelId?: number;
}

export default function StyleSampleList({
  onSelectSample,
  activeId,
  novelId = 0,
}: Props) {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  const [submittedSearch, setSubmittedSearch] = useState("");
  // Infinite scroll: fetch the next page when the sentinel enters the
  // viewport (react-intersection-observer useInView)
  const { ref: sentinelRef, inView } = useInView({ rootMargin: "100px" });

  // 5.3 commit 1: samples go through the useInfiniteStyleSamples query
  // (no more useApp.ListStyleSamples + loadPageRef trio).
  // page is managed by useInfiniteQuery's pageParam (not in queryKey); a
  // submittedSearch change triggers a new query.
  // data.pages.flatMap(p => p.items) gives the accumulated list; a novelId
  // change auto-refetches (queryKey includes novelId).
  // GET errors are handled by the global middleware (queryErrorToast.ts);
  // the component does not mount toastError. Aligned with the
  // SessionHistory pattern.
  const samplesQuery = useInfiniteStyleSamples({
    novelId,
    size: PAGE_SIZE,
    search: submittedSearch,
  });

  const samples = samplesQuery.data?.pages.flatMap((p) => p.items ?? []) ?? [];
  const total = samplesQuery.data?.pages[0]?.total ?? 0;
  const hasMore = samplesQuery.hasNextPage;
  const isLoading = samplesQuery.isLoading;
  const isFetchingMore = samplesQuery.isFetchingNextPage;

  // Reset the search term when novelId changes (the query auto-refetches
  // because the queryKey includes novelId)
  useEffect(() => {
    setSearch("");
    setSubmittedSearch("");
  }, [novelId]);

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
      samplesQuery.fetchNextPage();
    }
  }, [inView, hasMore, isFetchingMore, samplesQuery]);

  return (
    <>
      <div className="flex items-center justify-between px-3 py-2.5 border-b">
        <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
          {t("styleSample.samples")} ({total})
        </span>
      </div>
      <div className="px-2 py-1.5 border-b">
        <SearchInput
          value={search}
          onChange={setSearch}
          placeholder={t("styleSample.searchPlaceholder")}
        />
      </div>
      <div className="flex-1 overflow-y-auto overscroll-contain">
        {isLoading ? (
          <div className="flex items-center justify-center py-8 text-xs text-muted-foreground">
            {t("styleSample.loading")}
          </div>
        ) : samplesQuery.isError ? (
          <div className="flex items-center justify-center py-8 text-xs text-destructive">
            {t("styleSample.loadFailed")}
          </div>
        ) : samples.length === 0 ? (
          <div className="flex items-center justify-center py-8 text-xs text-muted-foreground">
            {search ? t("styleSample.noMatching") : t("styleSample.noSamples")}
          </div>
        ) : (
          <>
            {samples.map((s) => (
              <button
                key={s.id}
                onClick={() => onSelectSample(s.id)}
                className={`relative w-full flex flex-col px-3 py-1.5 text-left hover:bg-muted/50 transition-colors ${
                  activeId === s.id ? "bg-muted" : ""
                }`}
              >
                {activeId === s.id && (
                  <span className="absolute left-0 top-1/2 -translate-y-1/2 w-0.5 h-5 bg-primary rounded-r-full" />
                )}
                <span className="text-sm truncate">{s.name}</span>
                <span className="text-[11px] text-muted-foreground truncate">
                  {s.word_count} {t("styleSample.charCount")}
                </span>
              </button>
            ))}
            {isFetchingMore && (
              <div className="flex justify-center py-3">
                <Loader2 className="w-4 h-4 animate-spin text-muted-foreground" />
              </div>
            )}
            {/* Infinite scroll sentinel: triggers fetchNextPage when it
             * enters the viewport */}
            <div ref={sentinelRef} className="h-4" />
          </>
        )}
      </div>
    </>
  );
}
