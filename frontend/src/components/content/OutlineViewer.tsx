import { useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import Markdown from "@/components/Markdown";
import { useEditorTabsStore } from "./useEditorTabsStore";

interface Props {
  content: string;
  // Position key `novelId:path:mode`: independently saves/restores the
  // scroll position per (tab, mode). Not passed = not remembered.
  positionKey?: string;
}

export default function OutlineViewer({ content, positionKey }: Props) {
  const { t } = useTranslation();
  const containerRef = useRef<HTMLDivElement>(null);
  const restoredRef = useRef(false);

  // Restore the scroll position once the content is ready (outlineContent
  // starts empty and loads asynchronously, so wait for the first content
  // before restoring)
  useEffect(() => {
    if (!content || !positionKey || restoredRef.current) return;
    restoredRef.current = true;
    const saved =
      useEditorTabsStore.getState().positions[positionKey]?.scrollTop;
    const el = containerRef.current;
    if (el && typeof saved === "number") {
      el.scrollTop = saved;
    }
  }, [content, positionKey]);

  if (!content) {
    return (
      <div className="flex items-center justify-center h-full">
        <p className="text-sm text-muted-foreground">
          {t("content.noOutline")}
        </p>
      </div>
    );
  }

  // Write to the store (memory) synchronously — the localStorage write is
  // handled by the store's global subscribe with debouncing.
  const onScroll = () => {
    if (!positionKey) return;
    const el = containerRef.current;
    if (!el) return;
    useEditorTabsStore.getState().setPosition(positionKey, {
      scrollTop: el.scrollTop,
      updatedAt: Date.now(),
    });
  };

  return (
    <div
      ref={containerRef}
      className="overflow-auto h-full p-6"
      onScroll={onScroll}
    >
      <Markdown content={content} />
    </div>
  );
}
