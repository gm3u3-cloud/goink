import { useCallback } from "react";
import Editor, { type OnMount } from "@monaco-editor/react";
import { useEditorTabsStore } from "./useEditorTabsStore";

interface Props {
  value: string;
  onChange: (value: string | undefined) => void;
  onMount: OnMount;
  editorTheme?: string;
  // Position key `novelId:path:mode`: independently saves/restores the
  // Monaco viewState (scroll + cursor + folding) per (tab, mode). Not
  // passed = not remembered.
  positionKey?: string;
}

type EditorInstance = Parameters<OnMount>[0];
type EditorViewState = Parameters<EditorInstance["restoreViewState"]>[0];

export default function ContentEditor({
  value,
  onChange,
  onMount,
  editorTheme,
  positionKey,
}: Props) {
  const handleMount: OnMount = useCallback(
    (editor, monaco) => {
      if (positionKey) {
        const saved =
          useEditorTabsStore.getState().positions[positionKey]?.viewState;
        if (saved) {
          try {
            editor.restoreViewState(saved as EditorViewState);
          } catch {
            // Silently ignore the saved viewState when it is incompatible
            // with the current model (the content has changed)
          }
          // Monaco does not render the caret (cursor) when unfocused. A
          // synchronous focus() during the onMount phase may silently fail
          // because the editor's internal focus binding is not yet complete
          // (especially under the wails webview); defer the focus to the
          // next frame. Do not call render(true): a full re-render would
          // cause visual flicker.
          let disposed = false;
          editor.onDidDispose(() => {
            disposed = true;
          });
          requestAnimationFrame(() => {
            if (disposed) return;
            editor.focus();
          });
        }
        // Write to the store (memory) synchronously — the localStorage
        // write is handled by the store's global subscribe with debouncing.
        // No in-component debounce: cursor/scroll changes go to memory
        // immediately and are not lost on switch; the write-to-disk
        // frequency is unaffected.
        const save = () => {
          const vs = editor.saveViewState();
          if (vs) {
            useEditorTabsStore.getState().setPosition(positionKey, {
              viewState: vs,
              updatedAt: Date.now(),
            });
          }
        };
        editor.onDidScrollChange(save);
        editor.onDidChangeCursorPosition(save);
      }
      onMount(editor, monaco);
    },
    [positionKey, onMount],
  );

  return (
    <Editor
      height="100%"
      language="plaintext"
      theme={editorTheme ?? "light"}
      value={value}
      onChange={onChange}
      onMount={handleMount}
      options={{
        minimap: { enabled: false },
        lineNumbers: "on",
        scrollBeyondLastLine: false,
        fontSize: 17,
        lineHeight: 30,
        fontFamily: "'Noto Serif SC', 'Source Han Serif SC', serif",
        wordWrap: "on",
        automaticLayout: true,
        renderLineHighlight: "line",
        cursorSmoothCaretAnimation: "on",
        smoothScrolling: true,
        mouseWheelZoom: true,
        unicodeHighlight: {
          nonBasicASCII: false,
          ambiguousCharacters: false,
          invisibleCharacters: false,
        },
        suggestOnTriggerCharacters: false,
        quickSuggestions: false,
        wordBasedSuggestions: "off",
      }}
    />
  );
}
