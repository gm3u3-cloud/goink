import { useMutation } from "@tanstack/react-query";
import { CompressContext } from "@/lib/wailsjs/go/app/App";
import type { app } from "@/lib/wailsjs/go/models";

// useCompressContext: context compression mutation.
// No onSuccess/onError is attached — back-filling the turnId + toastError +
// isCompressing/compressingRef is handled by the caller's handleCompress
// (the compressing turn animation + loading state are managed inside the
// component).
export function useCompressContext() {
  return useMutation({
    mutationFn: (input: app.CompressInput) => CompressContext(input),
  });
}
