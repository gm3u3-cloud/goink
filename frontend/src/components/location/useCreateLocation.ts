import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CreateLocation } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { locationKeys } from "@/lib/queryKeys";

// useCreateLocation: create location mutation (see useCreateCharacter).
// mutationFn imports wailsjs CreateLocation directly (not useApp).
// Input app.CreateLocationInput: name is required;
// location_type/description/parent_location_id/tags are optional.
// The handler is responsible for setEditMode(null) + throwing the error
// (the side effects differ, so they are not placed in the mutation).
// onSuccess invalidates the list: the new location enters the list, and
// the LocationListView / LocationList / LocationGraph nodes stay in sync.
// relations is not invalidated: a newly created location has no relations,
// so the relations cache has no dirty data.
export function useCreateLocation(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: app.CreateLocationInput) =>
      CreateLocation(novelId, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: locationKeys.list(novelId) });
    },
  });
}
