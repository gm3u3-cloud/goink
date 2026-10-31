import { useMutation, useQueryClient } from "@tanstack/react-query";
import { DeleteLocation } from "@/lib/wailsjs/go/app/App";
import { locationKeys } from "@/lib/queryKeys";

// useDeleteLocation: delete location mutation (see useDeleteCharacter).
// mutationFn imports wailsjs DeleteLocation directly (not useApp).
// Shared in two places: LocationListView executes the deletion
// (confirmDelete calls mutateAsync); LocationList only dispatches to the
// store and does not execute.
// The handler is responsible for setDeletingLocationId(null) + throwing the
// error (the side effects differ, so they are not placed in the mutation).
// onSuccess invalidates list + relations: the backend DeleteLocation
// transaction cascades to delete LocationRelations (both where location_a
// or location_b = locID), so both frontend query caches must refresh
// (LocationGraph's useLocationRelations also refetches, keeping the cache
// clean).
export function useDeleteLocation(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => DeleteLocation(novelId, id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: locationKeys.list(novelId) });
      qc.invalidateQueries({ queryKey: locationKeys.relations(novelId) });
    },
  });
}
