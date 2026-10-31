import { useMutation, useQueryClient } from "@tanstack/react-query";
import { UpdateLocation } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";
import { locationKeys } from "@/lib/queryKeys";

// useUpdateLocation: update location mutation (see useUpdateCharacter).
// mutationFn imports wailsjs UpdateLocation directly (not useApp).
// Input {id, input}: input uses app.UpdateLocationInput (PUT semantics,
// full payload).
// The handler is responsible for setEditMode(null) + throwing the error
// (the side effects differ, so they are not placed in the mutation).
// onSuccess invalidates the list: after renaming/description/type/parent
// changes the list stays in sync (LocationList tree structure + Graph nodes
// are rebuilt in sync).
// relations is not invalidated: LocationRelation only stores
// location_a/location_b, not name, so renaming/description/type/parent
// changes do not affect the relations cache (the Graph node names come from
// the useLocations refetch).
export function useUpdateLocation(novelId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      input,
    }: {
      id: number;
      input: app.UpdateLocationInput;
    }) => UpdateLocation(novelId, id, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: locationKeys.list(novelId) });
    },
  });
}
