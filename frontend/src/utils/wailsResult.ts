// Adapter layer for the new apperr API.
//
// The project has two Wails API styles:
//   - Old API: returns raw data; on failure wails rejects with a string
//     (Go err.Error()), caught by frontend try/catch.
//   - New API (apperr): returns *apperr.Result[T], always HTTP 200, shape
//     {data, err_code, err_msg}; business errors appear in the err_code field
//     (empty string means success), never throws.
//
// See:
//   - docs/feat/v1.4.0/refactor-plan/04a-query-error-toast.md, "apperr new
//     API adapter" section
//   - internal/apperr/apperr.go (Result[T] / Code / CodeFromError)
//
// Problem: the new API does not throw → queryFn awaiting it never throws →
// the query never errors → the global error middleware (queryErrorToast.ts)
// never fires → errors are silently swallowed (violates rule 8).
//
// Solution: queryFn uses unwrapResult(res) to unwrap manually; when err_code
// is non-empty it throws AppErr, translating "business error" into "query
// error" and reusing the existing query error chain (the middleware takes
// over the toast). AppErr.errCode is kept for component catch blocks to read
// and pass to classifyError for short-code → specific message mapping (e.g.
// SkillMarketplace's inline error bar).
//
// AppErr carries the backend apperr.Code so components can render
// differently per error code.
// extends Error so TanStack Query / the middleware / try-catch treat it as a
// normal error.
export class AppErr extends Error {
  errCode: string;
  constructor(code: string, msg: string) {
    super(msg);
    this.name = "AppErr";
    this.errCode = code;
  }
}

// unwrapResult unwraps apperr.Result[T].
// Throws AppErr when err_code is non-empty (and not the "ok" fallback),
// otherwise returns res.data.
// Backend CodeOK = "" (empty string); on the normal success path err_code is
// empty. The explicit "ok" string is handled defensively.
// data is optional: on failure the backend data is a zero value (wailsjs
// bindings generate data?); on success the backend guarantees a value.
export function unwrapResult<T>(res: {
  err_code: string;
  err_msg?: string;
  data?: T;
}): T {
  if (res.err_code && res.err_code !== "ok") {
    throw new AppErr(res.err_code, res.err_msg ?? res.err_code);
  }
  return res.data as T;
}
