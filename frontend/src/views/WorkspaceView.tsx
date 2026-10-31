import {
  useState,
  useEffect,
  useLayoutEffect,
  useCallback,
  useRef,
} from "react";
import { useTranslation } from "react-i18next";
import type { imp, novel } from "@/lib/wailsjs/go/models";
import ActivityBar from "@/components/shell/ActivityBar";
import StatusBar from "@/components/shell/StatusBar";
import WindowControls from "@/components/shell/WindowControls";
import SidePanel from "@/components/sidebar/SidePanel";
import ContentPanel, {
  type ContentPanelHandle,
} from "@/components/content/ContentPanel";
import CharacterListView from "@/components/character/CharacterListView";
import LocationListView from "@/components/location/LocationListView";
import ArcListView from "@/components/storyarc/ArcListView";
import TimelineView from "@/components/timeline/TimelineView";
import ReaderView from "@/components/reader/ReaderView";
import PreferenceView from "@/components/preference/PreferenceView";
import NovelSettingView from "@/components/novel-setting/NovelSettingView";
import ChapterManagementView from "@/components/chapter-management/ChapterManagementView";
import BookshelfView from "@/components/novel/BookshelfView";
import NovelDialogs from "@/components/novel/NovelDialogs";
import ImportProgressDialog from "@/components/novel/ImportProgressDialog";
import ChatPanel from "@/components/chat/ChatPanel";
import GitHubLink from "@/components/shell/GitHubLink";
import SettingsDialog from "@/components/settings/SettingsDialog";
import HelpDialog from "@/components/help/HelpDialog";
import ProfileView from "@/components/profile/ProfileView";
import GitCommitView from "@/components/git/GitCommitView";
import ExtractWorkspaceView from "@/components/extract/ExtractWorkspaceView";
import UpdateDialog from "@/components/update/UpdateDialog";
import ErrorBoundary from "@/components/shared/ErrorBoundary";
import { GetPlatform, ApproveTool } from "@/lib/wailsjs/go/app/App";
import { Settings, User, HelpCircle, Moon, Sun } from "lucide-react";
import { WindowToggleMaximise } from "@/lib/wailsjs/runtime/runtime";
import Logo from "@/components/Logo";
import { useThemeStore, type Theme } from "@/stores/useThemeStore";
import { useLayoutState } from "@/hooks/useLayoutState";
import { useWindowState } from "@/hooks/useWindowState";
import { useImportNovel } from "@/hooks/useImportNovel";
import type { PanelId, SidebarPanelId } from "@/types/panel";
import { usePanelStore } from "@/stores/usePanelStore";
import { useFocusStore } from "@/stores/useFocusStore";
import { useEditorStore } from "@/stores/useEditorStore";
import { useGitStore } from "@/components/git/useGitStore";
import { useNovels } from "@/components/novel/useNovels";
import { useNovelStore } from "@/components/novel/useNovelStore";
import { useCreateNovel } from "@/components/novel/useCreateNovel";
import { useSaveCover } from "@/components/novel/useSaveCover";
import { novelKeys } from "@/lib/queryKeys";
import { useQueryClient } from "@tanstack/react-query";
import { toastError } from "@/utils/toast";
import { toErrorMessage } from "@/utils/error";
import { useUpdateCheck } from "@/components/update/useUpdateCheck";
import { useContentNavigation } from "./workspace/useContentNavigation";
import { useAIFileCacheInvalidation } from "./workspace/useAIFileCacheInvalidation";
import { useCreativeActivity } from "./workspace/useCreativeActivity";
import { useEditorTabTitles } from "@/components/content/useEditorTabTitles";

const THEME_ICON: Record<Theme, React.ReactNode> = {
  light: <Moon className="w-5 h-5" />,
  dark: <Sun className="w-5 h-5" />,
};

// Panels routed through ContentPanel (the activePanel routing layer).
// ContentPanel internally shows chapters/skills/goink.md/diff/outlines
// via its tab mechanism, bypassing the activePanel routing.
const CONTENT_PANEL_IDS = new Set<PanelId>(["chapters", "skills"]);

interface Props {
  initialNovelId: number;
  initialShowHelp?: boolean;
}

export default function WorkspaceView({
  initialNovelId,
  initialShowHelp,
}: Props) {
  useCreativeActivity();
  const { t } = useTranslation();
  const THEME_LABEL: Record<Theme, string> = {
    light: t("workspace.darkMode"),
    dark: t("workspace.lightMode"),
  };
  const contentRef = useRef<ContentPanelHandle>(null);
  const initialNovelReported = useRef(false);
  const automaticNovelSwitchPending = useRef(false);

  // novels go through the useNovels query (3.1): replaces the former
  // novels state + loadNovels + useEffect.
  // Switching panels within the 30s staleTime does not refetch;
  // novelsLoading guards the "auto-select novel" effect (replaces loadedRef).
  const {
    data: novels = [],
    isLoading: novelsLoading,
    isSuccess: novelsLoaded,
  } = useNovels();
  const queryClient = useQueryClient();
  useAIFileCacheInvalidation();
  // Novel-domain UI state: activeNovelId stays in WorkspaceView (routing);
  // dialog open/close + setters are subscribed by NovelDialogs (3.6); only
  // setExportNovelId remains here — SidePanel opens the dialog via
  // onExportNovel, and the stable setter reference avoids re-renders.
  const activeNovelId = useNovelStore((s) => s.activeNovelId);
  useEditorTabTitles(activeNovelId);
  const setActiveNovelId = useNovelStore((s) => s.setActiveNovelId);
  const setExportNovelId = useNovelStore((s) => s.setExportNovelId);
  // 3.7: switchNovel action (set activeNovelId + SetActiveNovel backend).
  // switchToNovel is a thin wrapper calling it; closeAllTabs is removed
  // (useEditorTabs takes over tab switching).
  const switchNovel = useNovelStore((s) => s.switchNovel);
  // 3.3: create-novel mutation. onSuccess invalidates novelKeys.all;
  // dedicated to handleCreateNovel (SidePanel inline form); the dialog
  // path's createNovel instance lives in NovelDialogs.
  const createNovel = useCreateNovel();
  // 5.9: save-cover mutation. onSuccess invalidates novelKeys.all so
  // useNovels refetches and picks up the new cover.
  const saveCover = useSaveCover();
  // activePanel/sidebarPanel/sidebarClosed externalized to usePanelStore (2.7).
  // Subscribed via selectors (not whole-store destructuring): action refs
  // are stable and do not trigger re-renders; only value changes like
  // activePanel re-render, avoiding loops from same-value sets.
  const activePanel = usePanelStore((s) => s.activePanel);
  const sidebarPanel = usePanelStore((s) => s.sidebarPanel);
  const sidebarClosed = usePanelStore((s) => s.sidebarClosed);
  const setActivePanel = usePanelStore((s) => s.setActivePanel);
  const setSidebarPanel = usePanelStore((s) => s.setSidebarPanel);
  const setSidebarClosed = usePanelStore((s) => s.setSidebarClosed);
  // focusMap externalized to useFocusStore (2.8). Each View subscribes to
  // focusId on its own.
  // styleSampleFocusId has different semantics (null = handled), so it
  // stays as local state.
  const focusEntity = useFocusStore((s) => s.focusEntity);
  const [styleSampleFocusId, setStyleSampleFocusId] = useState<number | null>(
    null,
  );
  const [showCreate, setShowCreate] = useState(false);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [showSettings, setShowSettings] = useState(false);
  const [showHelp, setShowHelp] = useState(false);
  const [activeSkillName, setActiveSkillName] = useState<string | null>(null);
  // 3.8 follow-up: tabTarget/activeContent/isDirty → useEditorStore;
  // selectedGitFile → useGitStore. Writers call getState().setXxx; reader
  // components subscribe on their own.
  // reset is called automatically by the effect below watching activeNovelId
  // changes, equivalent to the 3 setState lines of the old switchToNovel wrapper.
  const [platformOS, setPlatformOS] = useState("");
  const { showUpdate, updateResult, setShowUpdate } = useUpdateCheck();
  const { theme, toggle: toggleTheme } = useThemeStore();
  const { isMaximised, setIsMaximised } = useWindowState();
  const {
    sidePanelWidth,
    chatPanelWidth,
    setSidePanelWidth,
    setChatPanelWidth,
  } = useLayoutState();
  // On first mount, synchronously initialize activeNovelId + activePanel
  // (store defaults to 0/"novels"; overridden with initialNovelId).
  // useLayoutEffect runs before paint, avoiding a flash on first render from
  // using the default value and then switching.
  // Note: the former useState(initialNovelId) synchronous initial value;
  // after moving to the store (default 0), it must be synchronously
  // overridden here.
  // The restored ID is now only a candidate; the unselected-book state is
  // kept until the backend confirms successfully.
  useLayoutEffect(() => {
    setActiveNovelId(0);
    setActivePanel("novels");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // ── Update check (scheduling and state encapsulated in useUpdateCheck) ──

  // ── Book management dialogs (state in useNovelStore, externalized in 3.2) ──

  // ── Window state ────────────────────────────────────────────

  useEffect(() => {
    GetPlatform().then((info) => {
      if (info.os) setPlatformOS(info.os as string);
    });
  }, []);

  // ── Auto-show help on first entry ──────────────────────────

  useEffect(() => {
    if (initialShowHelp) setShowHelp(true);
  }, [initialShowHelp]);

  // 3.8 follow-up: remove the switchToNovel wrapper. The novel-switch reset
  // is now called automatically by the effect below watching activeNovelId
  // changes, invoking useEditorStore/useGitStore reset() — reactive rather
  // than imperative.
  // Callers (handleImportedNovel/handleSelectNovel/handleCreateNovel/
  // NovelDialogs) directly call switchNovel(id) + setActivePanel("chapters");
  // tab switching is still handled by useEditorTabs' novelId effect.
  // useLayoutEffect (not useEffect): reset must run before paint, otherwise
  // StatusBar would show one extra frame of stale word count when switching
  // novels (useEffect runs after paint, keeping the old store value one frame).
  useLayoutEffect(() => {
    useEditorStore.getState().reset();
    useGitStore.getState().reset();
  }, [activeNovelId]);

  const handleImportedNovel = useCallback(
    async (res: imp.ImportResult) => {
      // Temporary: invalidateQueries triggers a useNovels refetch; once the
      // 3.3 mutation is in place, onSuccess takes over.
      await queryClient.invalidateQueries({ queryKey: novelKeys.all });
      // 3.8 follow-up: remove the switchToNovel wrapper, directly call
      // switchNovel + setActivePanel.
      // reset is called automatically by the effect watching activeNovelId.
      await switchNovel(res.novel_id);
      setActivePanel("chapters");
    },
    [queryClient, switchNovel, setActivePanel],
  );

  const importNovel = useImportNovel({ onImported: handleImportedNovel });

  // ── SidePanel → ContentPanel bridge ─────────────────────────

  const {
    handleSelectChapter,
    handleSelectGoink,
    handleSelectVolumeOutline,
    handleSelectSkill,
    handleEditSkill,
    handleNewSkill,
    handleSearchNavigateChapter,
  } = useContentNavigation({
    contentRef,
    setActivePanel,
    setActiveSkillName,
  });

  // ── Approval ────────────────────────────────────────────

  async function handleApprove(toolId: string, feedback: string) {
    try {
      await ApproveTool(toolId, true, feedback);
    } catch (err) {
      toastError(toErrorMessage(err, t("approval.approveFailed")));
      return;
    }
  }

  function handleFileEditFinished(toolId: string, path?: string) {
    if (path) {
      void contentRef.current?.handleDiffApprove(toolId, path);
    } else {
      void contentRef.current?.handleDiffReject(toolId);
    }
  }

  async function handleReject(toolId: string, feedback: string) {
    try {
      await ApproveTool(toolId, false, feedback);
    } catch (err) {
      toastError(toErrorMessage(err, t("approval.rejectFailed")));
      return;
    }
    await contentRef.current?.handleDiffReject(toolId);
  }

  function handleApprovalFileEdit(data: {
    path: string;
    title: string;
    diff: string;
    original: string;
    modified: string;
    changeType: string;
    reason: string;
    toolId: string;
  }) {
    contentRef.current?.openDiffTab(data);
  }

  // ── Auto-select novel ────────────────────────────────────────

  useEffect(() => {
    if (novelsLoading || !novelsLoaded || automaticNovelSwitchPending.current)
      return;
    const restoring = !initialNovelReported.current;
    initialNovelReported.current = true;
    const currentID = useNovelStore.getState().activeNovelId;
    const candidateID = restoring ? initialNovelId : currentID;
    const exists = novels.find((n) => n.id === candidateID);
    if (restoring || !exists) {
      const first = exists ?? novels[0];
      const nextID = first?.id ?? 0;
      automaticNovelSwitchPending.current = true;
      // 5.9: use switchNovel (encapsulates setActiveNovelId + SetActiveNovel
      // backend), replacing app.SetActiveNovel.
      // No await inside the effect (the void marker intentionally does not
      // wait), to avoid making the effect async.
      void switchNovel(nextID)
        .then(() => {
          if (useNovelStore.getState().activeNovelId === nextID) {
            setActivePanel(nextID ? "chapters" : "novels");
          }
        })
        .catch((err: unknown) => {
          toastError(toErrorMessage(err));
        })
        .finally(() => {
          automaticNovelSwitchPending.current = false;
        });
    }
    // 3.9 fix: trigger only on novels changes (no activeNovelId).
    // Otherwise, when creating a new novel, switchToNovel sets
    // activeNovelId=new novel, but the useNovels refetch is not yet complete,
    // the old novels list does not contain the new novel → find fails →
    // novels[0] (the old novel) is mistakenly selected.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [novels, novelsLoading, novelsLoaded]);

  function handleActivitySelect(id: SidebarPanelId) {
    const currentPanel = sidebarPanel ?? activePanel;
    if (id === currentPanel && !sidebarClosed) {
      setSidebarClosed(true);
      return;
    }
    setSidebarClosed(false);
    if (id === "search") {
      setSidebarPanel("search");
    } else {
      setSidebarPanel(null);
      setActivePanel(id);
      contentRef.current?.clearHighlight();
    }
  }

  // 3.8 follow-up: selectedGitFile moved to useGitStore; GitHistoryList
  // writes to the store itself, so onSelectGitFile is no longer passed through.
  // 4b: pass type through to focusEntity (storyarc global search distinguishes
  // arc/node navigation; other domains use undefined).
  function handleSearchNavigateEntity(
    panelId: PanelId,
    entityId: number,
    type?: "arc" | "node",
  ) {
    focusEntity(panelId, entityId, type);
    setActivePanel(panelId);
  }

  async function handleSelectNovel(n: novel.Novel) {
    try {
      // 3.8 follow-up: remove the switchToNovel wrapper, directly call
      // switchNovel + setActivePanel.
      await switchNovel(n.id);
      setActivePanel("chapters");
    } catch (err) {
      console.error(err);
      toastError(toErrorMessage(err));
    }
  }

  async function handleCreateNovel() {
    try {
      if (!title.trim()) return;
      // 3.3: use the mutation; invalidate is handled by onSuccess;
      // switchNovel + form clearing stay in the handler.
      const n = await createNovel.mutateAsync({
        title: title.trim(),
        description: description.trim(),
      });
      setTitle("");
      setDescription("");
      setShowCreate(false);
      // 3.8 follow-up: remove the switchToNovel wrapper, directly call
      // switchNovel + setActivePanel.
      await switchNovel(n.id);
      setActivePanel("chapters");
    } catch (err) {
      console.error(err);
      toastError(toErrorMessage(err));
    }
  }

  // 3.6: the 4 dialog handlers (create-dialog/update/delete/export) have
  // moved to NovelDialogs.
  // handleCreateNovel (SidePanel inline form) stays here — it does not go
  // through a dialog, directly doing mutateAsync + switchNovel.

  async function handleSaveCover(novelID: number, file: File) {
    const buf = await file.arrayBuffer();
    try {
      await saveCover.mutateAsync({
        novelId: novelID,
        cover: Array.from(new Uint8Array(buf)),
      });
    } catch (err) {
      toastError(toErrorMessage(err, t("novel.coverSaveFailed")));
    }
  }

  const activeNovel = novels.find((n) => n.id === activeNovelId);

  return (
    <>
      <div className="h-screen flex flex-col overflow-hidden">
        <header
          className="h-11 flex items-center border-b bg-sidebar shrink-0 select-none cursor-default"
          style={{ "--wails-draggable": "drag" } as React.CSSProperties}
          onDoubleClick={() => {
            WindowToggleMaximise();
            setIsMaximised((prev) => !prev);
          }}
        >
          <Logo className="h-7 w-7 ml-3" />
          <span className="text-sm font-medium pl-2 flex-1">
            {activeNovel?.title ?? "Goink"}
          </span>
          <div
            className="flex items-center h-full"
            style={{ "--wails-draggable": "no-drag" } as React.CSSProperties}
          >
            <GitHubLink />
            <button
              onClick={() => setActivePanel("profile")}
              className={`text-muted-foreground hover:text-foreground transition-colors cursor-pointer w-8 h-8 flex items-center justify-center ml-2 ${activePanel === "profile" ? "text-foreground" : ""}`}
              title={t("workspace.profile")}
            >
              <User className="w-5 h-5" />
            </button>
            <button
              onClick={() => setShowHelp(true)}
              className="text-muted-foreground hover:text-foreground transition-colors cursor-pointer w-8 h-8 flex items-center justify-center"
              title={t("workspace.help")}
            >
              <HelpCircle className="w-5 h-5" />
            </button>
            <button
              onClick={toggleTheme}
              className="text-muted-foreground hover:text-foreground transition-colors cursor-pointer w-8 h-8 flex items-center justify-center"
              title={THEME_LABEL[theme]}
            >
              {THEME_ICON[theme]}
            </button>
            <button
              onClick={() => setShowSettings(true)}
              className="text-muted-foreground hover:text-foreground transition-colors cursor-pointer w-8 h-8 flex items-center justify-center mr-1"
              title={t("workspace.settings")}
            >
              <Settings className="w-5 h-5" />
            </button>
            <WindowControls
              platformOS={platformOS}
              isMaximised={isMaximised}
              setIsMaximised={setIsMaximised}
            />
          </div>
        </header>

        <div className="flex-1 flex min-h-0 overflow-hidden">
          <ActivityBar onSelect={handleActivitySelect} />

          {!sidebarClosed && activePanel !== "chapter-management" && (
            <SidePanel
              novels={novels}
              novelId={activeNovelId}
              onSelectNovel={handleSelectNovel}
              onSelectChapter={handleSelectChapter}
              onSelectGoink={handleSelectGoink}
              onSelectVolumeOutline={handleSelectVolumeOutline}
              onExportNovel={(id) => setExportNovelId(id)}
              showCreate={showCreate}
              setShowCreate={setShowCreate}
              title={title}
              setTitle={setTitle}
              description={description}
              setDescription={setDescription}
              onCreateNovel={handleCreateNovel}
              activeSkillName={activeSkillName}
              onSelectSkill={handleSelectSkill}
              onEditSkill={handleEditSkill}
              onNewSkill={handleNewSkill}
              onSearchNavigateEntity={handleSearchNavigateEntity}
              onSearchNavigateChapter={handleSearchNavigateChapter}
              onSelectStyleSample={(id) => setStyleSampleFocusId(id)}
              sidePanelWidth={sidePanelWidth}
              onSidePanelResize={setSidePanelWidth}
            />
          )}

          {activePanel === "chapter-management" ? (
            <ErrorBoundary>
              <ChapterManagementView
                key={activeNovelId}
                novelId={activeNovelId}
              />
            </ErrorBoundary>
          ) : activePanel === "novels" ? (
            <BookshelfView
              onSelectNovel={handleSelectNovel}
              onSaveCover={handleSaveCover}
              onImportNovel={() => importNovel.startImport()}
            />
          ) : (
            CONTENT_PANEL_IDS.has(activePanel) && (
              <ContentPanel ref={contentRef} />
            )
          )}

          {/* Always mounted: pattern extraction is a long-running task, unmounting would interrupt progress listeners */}
          <div
            className={
              activePanel === "style-samples"
                ? "flex-1 flex flex-col min-h-0"
                : "hidden"
            }
          >
            <ErrorBoundary>
              <ExtractWorkspaceView
                novelId={activeNovelId}
                focusSampleId={styleSampleFocusId}
                onFocusSampleHandled={() => setStyleSampleFocusId(null)}
              />
            </ErrorBoundary>
          </div>
          {activePanel === "characters" ? (
            <ErrorBoundary>
              <CharacterListView novelId={activeNovelId} />
            </ErrorBoundary>
          ) : activePanel === "locations" ? (
            <ErrorBoundary>
              <LocationListView novelId={activeNovelId} />
            </ErrorBoundary>
          ) : activePanel === "storyarcs" ? (
            <ErrorBoundary>
              <ArcListView novelId={activeNovelId} />
            </ErrorBoundary>
          ) : activePanel === "timeline" ? (
            <ErrorBoundary>
              <TimelineView novelId={activeNovelId} />
            </ErrorBoundary>
          ) : activePanel === "reader" ? (
            <ErrorBoundary>
              <ReaderView novelId={activeNovelId} />
            </ErrorBoundary>
          ) : activePanel === "preferences" ? (
            <ErrorBoundary>
              <PreferenceView novelId={activeNovelId} />
            </ErrorBoundary>
          ) : activePanel === "novel-settings" ? (
            <ErrorBoundary>
              <NovelSettingView novelId={activeNovelId} />
            </ErrorBoundary>
          ) : activePanel === "git" ? (
            <ErrorBoundary>
              <GitCommitView />
            </ErrorBoundary>
          ) : activePanel === "profile" ? (
            <ErrorBoundary>
              <ProfileView />
            </ErrorBoundary>
          ) : null}

          <div
            style={{
              display:
                activePanel === "chapter-management" ||
                activePanel === "profile"
                  ? "none"
                  : "contents",
            }}
          >
            <ChatPanel
              novelId={activeNovelId}
              onApprove={handleApprove}
              onReject={handleReject}
              onApprovalFileEdit={handleApprovalFileEdit}
              onFileEditFinished={handleFileEditFinished}
              chatPanelWidth={chatPanelWidth}
              onChatPanelResize={setChatPanelWidth}
            />
          </div>
        </div>

        <StatusBar />

        <SettingsDialog
          open={showSettings}
          onClose={() => setShowSettings(false)}
          initialTab="general"
        />

        <HelpDialog open={showHelp} onClose={() => setShowHelp(false)} />

        <NovelDialogs />

        <ImportProgressDialog
          {...importNovel.dialogProps}
          selectedModel={importNovel.selectedModel}
          models={importNovel.models}
          onModelChange={importNovel.onModelChange}
          onStartLLM={importNovel.startLLMImport}
        />

        <UpdateDialog
          open={showUpdate}
          result={updateResult}
          onClose={() => setShowUpdate(false)}
        />
      </div>
    </>
  );
}
