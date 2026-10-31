import { useState, useMemo } from "react";
import { Eye } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useFocusStore } from "@/stores/useFocusStore";
import SearchInput from "@/components/shared/SearchInput";
import { buildChapterReferenceMap } from "@/components/chapter/chapterReferenceMap";
import { useChapters } from "@/components/chapter/useChapters";
import { useReaderPerspectives } from "./useReaderPerspectives";

interface Props {
  novelId: number;
}

export default function SidebarReaderList({ novelId }: Props) {
  const { t } = useTranslation();
  // 4.5.1: entries go through a query (shares the cache with ReaderView).
  // 4a: query error toasts are handled by the global middleware; the
  // component adds an inline isError display (aligned with TimelineList).
  const { data: items = [], isError } = useReaderPerspectives(novelId);
  const { data: chapters = [] } = useChapters(novelId);
  const chapterReferences = useMemo(
    () => buildChapterReferenceMap(chapters),
    [chapters],
  );
  // 4b: clicking an entry triggers focusEntity; ReaderView's useEffect
  // handles positioning + expanding + highlighting.
  const focusEntity = useFocusStore((s) => s.focusEntity);
  const [search, setSearch] = useState("");

  // 4b: the filter fields align with the backend Search (content OR
  // related_truth).
  const filtered = useMemo(() => {
    if (!search.trim()) return items;
    const q = search.toLowerCase();
    return items.filter(
      (e) =>
        e.content.toLowerCase().includes(q) ||
        (e.related_truth || "").toLowerCase().includes(q),
    );
  }, [items, search]);

  const typeDot = (type: string) => {
    switch (type) {
      case "known":
        return "bg-tag-green";
      case "suspense":
        return "bg-tag-amber";
      case "misconception":
        return "bg-tag-rose";
      default:
        return "bg-muted";
    }
  };

  return (
    <>
      <div className="flex items-center justify-between px-3 py-2.5 border-b">
        <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
          {t("reader.readerPerspective")} ({items.length})
        </span>
      </div>
      <div className="px-2 py-1.5 border-b">
        <SearchInput
          value={search}
          onChange={setSearch}
          placeholder={t("reader.searchEntries")}
        />
      </div>
      <div className="flex-1 overflow-y-auto overscroll-contain">
        {isError ? (
          <div className="flex items-center justify-center h-full">
            <p className="text-xs text-destructive">{t("reader.loadFailed")}</p>
          </div>
        ) : filtered.length === 0 ? (
          <div className="flex items-center justify-center h-full">
            <p className="text-xs text-muted-foreground">
              {search ? t("reader.noMatchingEntries2") : t("reader.noEntries")}
            </p>
          </div>
        ) : (
          filtered.map((e) => (
            <div
              key={e.id}
              onClick={() => focusEntity("reader", e.id)}
              className="w-full flex items-center gap-2 px-3 py-1.5 text-left cursor-pointer hover:bg-muted transition-colors group"
            >
              <span className="shrink-0 flex h-5 w-5 items-center justify-center rounded bg-tag-blue text-tag-blue-foreground">
                <Eye className="h-3 w-3" />
              </span>
              <div className="flex-1 min-w-0">
                <span className="text-xs truncate block text-foreground">
                  {e.content.length > 30
                    ? e.content.slice(0, 30) + "…"
                    : e.content}
                </span>
                <span className="text-[10px] text-muted-foreground">
                  {e.type} ·{" "}
                  {t("reader.chapterN", {
                    n:
                      chapterReferences.readingNumberByChapterID.get(
                        e.planted_chapter_id ?? 0,
                      ) ?? "?",
                  })}
                </span>
              </div>
              <span
                className={`shrink-0 h-1.5 w-1.5 rounded-full ${typeDot(e.type)}`}
              />
            </div>
          ))
        )}
      </div>
    </>
  );
}
