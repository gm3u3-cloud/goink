import { useMutation, useQueryClient } from "@tanstack/react-query";
import { DeleteCharacter } from "@/lib/wailsjs/go/app/App";
import { characterKeys } from "@/lib/queryKeys";

// useDeleteCharacter: delete character mutation.
// mutationFn imports wailsjs DeleteCharacter directly (not useApp).
// Shared in two places: CharacterListView executes the deletion
// (confirmDelete calls mutateAsync); CharacterList only dispatches to the
// store and does not execute.
// The handler is responsible for setDeletingCharacterId(null) + throwing the
// error (the side effects differ, so they are not placed in the mutation).
// onSuccess invalidates list + relations: the backend DeleteCharacter
// transaction cascades to delete the relation records, so both frontend
// query caches must refresh (CharacterGraph's useCharacterRelations also
// refetches, keeping the cache clean).
export function useDeleteCharacter(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => DeleteCharacter(novelId, id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: characterKeys.list(novelId) });
      qc.invalidateQueries({ queryKey: characterKeys.relations(novelId) });
    },
  });
}
