# Goink Project Collaboration and Coding Rules

Goink is a desktop AI web-novel writing assistant built with Wails (Go + React). Users primarily communicate in Chinese; reply to users in Chinese.

## Workspace and Git

- Only read, edit, and run commands in the currently launched worktree. Do not enter another worktree to operate on files.
- The current repository uses two worktrees: `/home/nianhe/projects/todo` (`master`) and `/home/nianhe/projects/goink` (`dev`). Treat the current working directory as authoritative; do not hard-code switching to the other directory.
- Before running any Git command, confirm the current CWD is the worktree root for this task; do not pass another worktree's path to Git commands.
- Git defaults to read-only operations only: `status`, `diff`, `log`, `show`, `branch`, `tag`, `ls-files`, `blame`, `grep`, `rev-parse`, `rev-list`, `stash list`, etc.
- Do not perform any Git write operation without explicit user permission; before executing, first explain the specific operation to the user and obtain permission. This includes `add`, `commit`, `push`, `pull`, `fetch`, `merge`, `rebase`, `reset`, `revert`, `checkout`, `stash`, `clean`, creating/deleting branches or tags, and modifying Git configuration.
- When the user explicitly requests rolling back changes, after verifying the target scope and existing differences, prefer precise restoration via Git operations; do not hand-write a reverse patch. If the target file contains changes that must be preserved, do not overwrite the whole file. Git write operations still require the authorization above.
- After finishing changes, do not automatically `add`, commit, or push, and do not proactively ask "should I commit"; after completing code, stop first and wait for the user's review, and perform Git write operations only after the user explicitly instructs.
- After the user authorizes a commit, if pre-commit, tests, or other verification find problems and as a result add or modify any files, the original commit authorization does not cover those new changes; after fixing, you must stop, wait for the user to re-review and explicitly re-authorize, and must not continue `add`, commit, amend, revert, or reset on your own.
- Commit messages use English, are specific in description, contain no emoji, no `Co-Authored-By`, and must follow Conventional Commits: `<type>(<optional-scope>): <description>`.
- The only allowed types are: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.
- A commit message must contain both a subject and a body; leave a blank line after the subject, then use the body to explain what changed and why — a subject alone is not enough.
- The commit body must let a future `git blame` reader independently understand exactly what was done and why that design was chosen. For non-obvious behavior, state whether it is an intentional design or a temporary transition, under what conditions it can be removed or replaced, and any known limitations, risks, and common pitfalls. Do not just repeat the subject or list modified files.
- When a multi-paragraph commit message is needed, pass real newlines via a `git commit -F - <<'EOF'` heredoc; do not write literal `\\n` in a `-m` string.
- Reference issues using `Refs #NN` at the end of the body; do not use `fixes`, `closes`, or `resolves` unless the user explicitly asks to close the issue.

## Development Process and Authorization Boundaries

- The default flow is: first discuss the design → set the implementation and commit plan → user confirms → start writing code → verify → wait for user review → perform Git write operations only after explicit user authorization. When the user explicitly requests documentation, write or update the design doc before implementation.
- For tasks involving multiple files, data structures, APIs, databases, UI flows, or behavior changes, first explain the design, scope of impact, risks, and verification plan; do not write code before the user confirms.
- Only when the user explicitly requests documentation, after the design is confirmed, first capture the plan into the appropriate `docs/` document; when no documentation is requested, just explain the design, scope of impact, and commit plan in the conversation.
- Before starting implementation, list the expected file scope, implementation steps, and commit breakdown/plan; do not casually edit and commit incrementally without a plan.
- Without explicit user permission, do not create, modify, or delete code files on your own. You may perform read-only inspection, analysis, propose plans, and draft documentation; code implementation must wait for user confirmation.
- Pure formatting, obvious typo fixes, or small changes the user explicitly asks to "just fix" can be handled directly, but the other rules still apply, and after completing them wait for review.
- After writing code, report the changes and verification results, then stop and wait for the user's review; do not automatically stage, commit, push, or merge just because tests pass.
- After completing each implementation stage, explain to the user what was done, how it was done, why it was done that way, and what problem it solved, and report the verification results, known limitations, and commit status; the report should be specific to the actual changes in that stage to make review easier.

## Build and Verification

- Run Go commands from the repository root: `go build ./...`, `go test ./...`.
- Frontend commands must first enter `frontend/`: use `npm run build`, `npm run lint`, `npm run test`; do not run `npm install` at the project root.
- When installing frontend dependencies, run `npm install <pkg> --save` inside `frontend/`.
- `.githooks/pre-commit` runs verification scoped to the staged files: Go changes trigger build/test/golangci-lint, frontend changes trigger build/lint/test, and documentation and configuration changes are skipped. Unless the user asks or diagnosis requires it, do not re-run the full verification suite.
- The `commit-msg` hook validates Conventional Commits and requires a blank line between subject and body with a non-empty body; proactively comply before committing rather than relying on the hook to correct errors.
- System dependencies (Ubuntu/Debian): `libsqlite3-dev libgtk-3-dev libwebkit2gtk-4.1-dev gcc`.

## Codebase Structure and Conventions

- `app/`: the Wails binding layer; exported methods form the frontend API and only perform input/output DTO conversion, calling internal capabilities, and error adaptation — they do not carry domain orchestration.
- `internal/agent/`, `agentcfg/`, `llm/`, `session/`: LLM conversation, system prompts, transport, and sessions.
- `internal/mcp_tools/`: MCP tools; the specific rules for this directory are in `internal/mcp_tools/AGENTS.md`.
- `frontend/`: React 19 + TypeScript + Tailwind 4 + shadcn/ui.
- The data directory is `~/Goink/` on Linux/macOS and near the executable on Windows; it contains `models/`, `runtime/`, the database, and novel repositories.
- Each novel is an independent Git repository; chapters live in `chapters/NNN.md` and outlines in `outlines/NNN.md`.
- ONNX and sqlite-vec code use `//go:build cgo`; cgo-related compilation failures during Windows diagnostics are a known limitation.
- All ONNX, VectorStore, and RefreshQueue instances are maintained as global singletons per the existing code convention; do not casually introduce a second instance.
- `target_chapter` is used for sorting only; do not treat it as an exact filter condition.
- Messages are append-only, with distinct API, frontend, and full-audit query paths.

## Layering Boundaries

- Structs and DTOs: reuse when field semantics match; use composition or anonymous embedding when extra fields are needed; declare separately only when the field scope or semantics truly differ. Do not redeclare the same field just because input, storage, and query belong to different layers; when reusing, check that you are not bringing in fields the current interface does not need or should not expose.
- Simple CRUD within a single persistence medium can follow `App → Store → DB`; do not create empty-shell Services for operations with no business orchestration.
- A `Store` is only responsible for reading/writing, querying, and data consistency within transactions for its medium; for example, adjusting a chapter's `volume_id` / `sort_order` belongs in the Store.
- Cases involving multiple persistence media, multiple domains, or compensating flows must follow `App → internal/<domain>.Service → Store/dependencies`. The Service is responsible for business semantics and orchestration, e.g., coordinating DB records, Git files, cross-references, vector indexes, and caches.
- A Service must not depend back on `app/` or Wails; for optional infrastructure (such as a vector store or search cache), use a small interface or provider injection to avoid circular dependencies among domain packages.

## Modification Safety Boundaries

- Do not delete or modify logger statements or code comments unless the user explicitly requests it.
- Do not hand-edit Wails auto-generated binding files (for example `frontend/src/lib/wailsjs/go/models.ts`, `App.d.ts`). Regenerate them via `wails generate module` or `make build`.
- Use the editing tools to modify code; do not write code files directly with `sed` or Python scripts.
- When replacing text for the first time, use the exact original text copied from the file; if the match fails, then check formatting, indentation, or widen the anchor — do not format the whole file first.
- Multiple edits to the same file must be executed sequentially, not in parallel, to avoid a later write overwriting an earlier change.
- Unless the user explicitly requests it, do not proactively expand the task scope, and do not auto-generate commits, releases, or remote operations.

## Domain Details

- Character relationships are append-only, using `is_current` to mark the current relationship; an update adds a new record and preserves the history.
- Preferences are divided into global and per-novel preferences; classification is handled by the LLM.
- The ONNX embedder, VectorStore, and RefreshQueue are global singletons.
- The ONNX library lookup order, model directory, and vector table naming follow the existing implementation; do not redefine path rules within a single feature.

## Code Review Focus

- Check for accidental changes to auto-generated files, logs/comments, data append-only semantics, or cgo build tags.
- Check that Git write operations have explicit user authorization.
- Check that Go and frontend commands are run in the correct directory, and whether the changes require corresponding tests.
