import { useNovelStore } from "./useNovelStore";
import { usePanelStore } from "@/stores/usePanelStore";
import { useNovels } from "./useNovels";
import { useCreateNovel } from "./useCreateNovel";
import { useUpdateNovel } from "./useUpdateNovel";
import { useDeleteNovel } from "./useDeleteNovel";
import { ExportNovel } from "@/lib/wailsjs/go/app/App";
import NovelEditDialog from "./NovelEditDialog";
import NovelDeleteDialog from "./NovelDeleteDialog";
import ExportDialog from "@/components/export/ExportDialog";

// 3.6: Extract the 4 novel dialogs (create/edit/delete/export) from
// WorkspaceView into a standalone component.
// Internally subscribes to useNovelStore for the dialog toggles + the 4
// setters; submits via the 3.3-3.5 mutations.
// 3.8 follow-up: remove the switchToNovel prop; directly call
// useNovelStore.switchNovel + usePanelStore.setActivePanel.
// The reset is called automatically by the WorkspaceView effect watching
// activeNovelId changes.
export default function NovelDialogs() {
  const switchNovel = useNovelStore((s) => s.switchNovel);
  const setActivePanel = usePanelStore((s) => s.setActivePanel);
  // ExportDialog's displayed title is looked up from the novels list;
  // shares the useNovels cache with WorkspaceView.
  const { data: novels = [] } = useNovels();
  const editingNovel = useNovelStore((s) => s.editingNovel);
  const deletingNovel = useNovelStore((s) => s.deletingNovel);
  const showCreateDialog = useNovelStore((s) => s.showCreateDialog);
  const exportNovelId = useNovelStore((s) => s.exportNovelId);
  const setEditingNovel = useNovelStore((s) => s.setEditingNovel);
  const setDeletingNovel = useNovelStore((s) => s.setDeletingNovel);
  const setShowCreateDialog = useNovelStore((s) => s.setShowCreateDialog);
  const setExportNovelId = useNovelStore((s) => s.setExportNovelId);

  const createNovel = useCreateNovel();
  const updateNovel = useUpdateNovel();
  const deleteNovel = useDeleteNovel();

  async function handleCreateNovelFromDialog(input: {
    title: string;
    description: string;
    genre: string;
  }) {
    try {
      // The mutation's onSuccess has already invalidated novelKeys.all;
      // the handler only closes the dialog + switches to the new novel.
      const n = await createNovel.mutateAsync(input);
      setShowCreateDialog(false);
      // 3.8 follow-up: remove the switchToNovel wrapper; directly call
      // switchNovel + setActivePanel.
      await switchNovel(n.id);
      setActivePanel("chapters");
    } catch (err) {
      console.error(err);
      throw err;
    }
  }

  async function handleUpdateNovel(input: {
    title: string;
    description: string;
    genre: string;
  }) {
    if (!editingNovel) return;
    try {
      await updateNovel.mutateAsync({ id: editingNovel.id, input });
      setEditingNovel(null);
    } catch (err) {
      console.error(err);
      throw err;
    }
  }

  async function handleDeleteNovel() {
    if (!deletingNovel) return;
    try {
      // When deleting the current activeNovelId, WorkspaceView's
      // auto-select-novel effect takes over after the refetch.
      await deleteNovel.mutateAsync(deletingNovel.id);
      setDeletingNovel(null);
    } catch (err) {
      console.error(err);
      throw err;
    }
  }

  async function handleExportNovel(format: "epub" | "markdown" | "txt") {
    if (exportNovelId == null) return;
    try {
      await ExportNovel(exportNovelId, format);
    } catch (err) {
      console.error(err);
      throw err;
    }
  }

  return (
    <>
      <NovelEditDialog
        open={showCreateDialog}
        onClose={() => setShowCreateDialog(false)}
        onSave={handleCreateNovelFromDialog}
      />
      <NovelEditDialog
        open={!!editingNovel}
        novel={editingNovel}
        onClose={() => setEditingNovel(null)}
        onSave={handleUpdateNovel}
      />
      <NovelDeleteDialog
        open={!!deletingNovel}
        novelTitle={deletingNovel?.title ?? ""}
        onClose={() => setDeletingNovel(null)}
        onConfirm={handleDeleteNovel}
      />
      {exportNovelId !== null && (
        <ExportDialog
          novelTitle={novels.find((n) => n.id === exportNovelId)?.title ?? ""}
          onClose={() => setExportNovelId(null)}
          onExport={handleExportNovel}
        />
      )}
    </>
  );
}
