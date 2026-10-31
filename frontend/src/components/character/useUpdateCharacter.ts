import { useMutation, useQueryClient } from "@tanstack/react-query";
import { UpdateCharacter } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { characterKeys } from "@/lib/queryKeys";

// useUpdateCharacter: update character mutation.
// mutationFn imports wailsjs UpdateCharacter directly (not useApp).
// Input {id, input}: input uses app.UpdateCharacterInput (PUT semantics,
// full payload).
// The handler is responsible for setEditMode(null) + throwing the error
// (the side effects differ, so they are not placed in the mutation).
// onSuccess invalidates the list: the character name/description/abilities
// updates enter the list.
// relations is not invalidated: CharacterRelation only stores character_id,
// not name, so renaming/description/ability changes do not affect the
// relations cache (the Graph node names come from the useCharacters
// refetch).
export function useUpdateCharacter(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      input,
    }: {
      id: number;
      input: app.UpdateCharacterInput;
    }) => UpdateCharacter(novelId, id, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: characterKeys.list(novelId) });
    },
  });
}
