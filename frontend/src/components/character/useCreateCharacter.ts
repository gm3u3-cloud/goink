import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CreateCharacter } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { characterKeys } from "@/lib/queryKeys";

// useCreateCharacter: create character mutation.
// mutationFn imports wailsjs CreateCharacter directly (not useApp).
// Input app.CreateCharacterInput: name is required;
// description/personality/abilities are optional.
// The handler is responsible for setEditMode(null) + throwing the error
// (the side effects differ, so they are not placed in the mutation).
// onSuccess invalidates the list: the new character enters the list, and
// the CharacterListView / CharacterList / CharacterGraph nodes stay in sync.
// relations is not invalidated: a newly created character has no relations,
// so the relations cache has no dirty data.
export function useCreateCharacter(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: app.CreateCharacterInput) =>
      CreateCharacter(novelId, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: characterKeys.list(novelId) });
    },
  });
}
