#!/usr/bin/env node
// CJK scan gate script for the ru-translation plan.
// Counts CJK characters per zone (in-scope vs out-of-scope) and prints a
// summary table. Run from anywhere: `node scripts/cjk-scan.mjs [repoRoot]`.
//
// In-scope zones (must reach 0 CJK after the sweep):
//   - Go strings: *.go (excluding *_test.go)
//   - Frontend UI: frontend/src/**/*.{ts,tsx} (excluding tests, wailsjs, i18n locales)
//   - Builtin skills: internal/skill/builtin/*.md
//
// Out-of-scope (legacy, intentionally untouched):
//   - docs/**, internal/**/DESIGN.md, repo root *.md, .trae/**, .claude/**,
//     .github/**, .githooks/**, build/**
//   - *_test.go, *.test.ts(x)
//   - frontend/src/i18n/locales/zh-CN.json
//   - Go // comments (counted inside the Go total; not separable by this scan)

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

const root = process.argv[2] ? process.argv[2] : ".";

// CJK Unified Ideographs (+ Ext A), CJK punctuation, fullwidth forms.
const CJK_RE = /[\u3400-\u4DBF\u4E00-\u9FFF\u3000-\u303F\uFF00-\uFFEF]/g;

const SKIP_DIRS = new Set([
  ".git",
  "node_modules",
  "dist",
  "build",
  "third_party",
]);

function walk(dir, files = []) {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    let st;
    try {
      st = statSync(full);
    } catch {
      continue;
    }
    if (st.isDirectory()) {
      if (!SKIP_DIRS.has(entry)) walk(full, files);
    } else if (st.isFile()) {
      files.push(full);
    }
  }
  return files;
}

function readText(file) {
  try {
    return readFileSync(file, "utf8");
  } catch {
    return null;
  }
}

function cjkCount(text) {
  const m = text.match(CJK_RE);
  return m ? m.length : 0;
}

function countCJK(file) {
  const text = readText(file);
  return text === null ? 0 : cjkCount(text);
}

// Strip Go line and block comments (approximation: does not track string
// literals, so a "//" inside a string may be mis-stripped; fine for a gate).
function goNonCommentCJK(file) {
  const text = readText(file);
  if (text === null) return 0;
  let out = "";
  let i = 0;
  const n = text.length;
  while (i < n) {
    const c = text[i];
    const next = text[i + 1];
    if (c === "/" && next === "/") {
      while (i < n && text[i] !== "\n") i++;
      continue;
    }
    if (c === "/" && next === "*") {
      i += 2;
      while (i < n && !(text[i] === "*" && text[i + 1] === "/")) i++;
      i += 2;
      continue;
    }
    out += c;
    i++;
  }
  return cjkCount(out);
}

// CJK in these files is functional data, not translatable prose:
// regex patterns for Chinese chapter-heading detection, punctuation
// counting/normalization maps, provider-error keywords, and native
// language names. Per plan rule §1 (signatures vs data values) these
// must NOT be translated; they form the expected final residual and are
// reported separately from translatable residual.
const FUNCTIONAL_FILES = new Set([
  "internal/import/txt.go",
  "internal/import/llm.go",
  "internal/style/utils.go",
  "internal/mcp_tools/rw_tools.go",
  "internal/rag/splitter.go",
  "frontend/src/utils/errorExplain.ts",
  "frontend/src/views/InitView.tsx",
]);

const inScope = { go: [], frontend: [], skills: [], functional: [] };
const outScope = { goTest: [], frontendTest: [], locales: [], docs: [], other: [] };

for (const file of walk(root)) {
  const rel = relative(root, file);
  const n = countCJK(file);
  if (n === 0) continue;

  // Out-of-scope buckets first.
  if (
    rel.startsWith("docs/") ||
    rel.endsWith("DESIGN.md") ||
    rel.startsWith(".trae/") ||
    rel.startsWith(".claude/") ||
    rel.startsWith(".github/") ||
    rel.startsWith(".githooks/") ||
    (rel.endsWith(".md") && !rel.includes("/"))
  ) {
    outScope.docs.push([rel, n]);
    continue;
  }
  if (file.endsWith("_test.go")) {
    outScope.goTest.push([rel, n]);
    continue;
  }
  if (/(^|\/)frontend\/src\/.*\.(test|spec)\.(ts|tsx)$/.test(file)) {
    outScope.frontendTest.push([rel, n]);
    continue;
  }
  if (rel.includes("frontend/src/i18n/locales/")) {
    outScope.locales.push([rel, n]);
    continue;
  }

  // Functional data values (not translatable) — separate expected bucket.
  if (FUNCTIONAL_FILES.has(rel)) {
    inScope.functional.push([rel, n]);
    continue;
  }

  // In-scope buckets.
  if (file.endsWith(".go") && !rel.includes("frontend")) {
    inScope.go.push([rel, n, goNonCommentCJK(file)]);
    continue;
  }
  if (/^frontend\/src\/.*\.(ts|tsx)$/.test(rel)) {
    if (rel.includes("frontend/src/lib/wailsjs/")) {
      outScope.other.push([rel, n]);
      continue;
    }
    inScope.frontend.push([rel, n]);
    continue;
  }
  if (rel.includes("internal/skill/builtin/") && file.endsWith(".md")) {
    inScope.skills.push([rel, n]);
    continue;
  }
  outScope.other.push([rel, n]);
}

function total(list) {
  return list.reduce((s, [, n]) => s + n, 0);
}

function printList(name, list, top = 10, fmt = ([f, n]) => `${String(n).padStart(6)}  ${f}`, sortIdx = 1) {
  console.log(`\n== ${name}: ${total(list)} CJK in ${list.length} files ==`);
  const sorted = [...list].sort((a, b) => b[sortIdx] - a[sortIdx]).slice(0, top);
  for (const entry of sorted)
    console.log(`  ${fmt(entry)}`);
  if (list.length > top) console.log(`  ... and ${list.length - top} more`);
}

const goTotal = inScope.go.reduce((s, [, , nc]) => s + nc, 0);
console.log(`CJK scan of ${root}`);
console.log(
  `IN SCOPE total: ${total(inScope.go) + total(inScope.frontend) + total(inScope.skills)} CJK`
  + ` (functional data: ${total(inScope.functional)} CJK, not translatable)`
);
console.log(
  `  Go: ${total(inScope.go)} total (of which ${goTotal} in strings/code, rest // comments)`
);
printList("Functional data (expected residual)", inScope.functional, inScope.functional.length);
printList(
  "Go (non-test): non-comment CJK / total",
  inScope.go,
  20,
  ([f, t, nc]) => `${String(nc).padStart(5)}/${String(t).padStart(5)}  ${f}`,
  2
);
printList("Frontend UI (non-test)", inScope.frontend, 20);
printList("Builtin skills", inScope.skills, 12);
console.log(
  `\nOUT OF SCOPE total: ${
    total(outScope.goTest) +
    total(outScope.frontendTest) +
    total(outScope.locales) +
    total(outScope.docs) +
    total(outScope.other)
  } CJK`
);
printList("Go tests", outScope.goTest, 5);
printList("Frontend tests", outScope.frontendTest, 5);
printList("i18n locales", outScope.locales, 5);
printList("Docs / repo md", outScope.docs, 5);
printList("Other (wailsjs etc.)", outScope.other, 5);

// Full residual list for the final gate (stage 5). For Go, residual =
// non-comment CJK (comments are out of scope). Functional data files are
// excluded (tracked in their own bucket above).
console.log(`\n== TRANSLATABLE IN-SCOPE RESIDUAL (must reach 0) ==`);
const residual = [
  ...inScope.go.map(([f, , nc]) => [f, nc]),
  ...inScope.frontend.map(([f, n]) => [f, n]),
  ...inScope.skills.map(([f, n]) => [f, n]),
]
  .filter(([, n]) => n > 0)
  .sort((a, b) => b[1] - a[1]);
for (const [f, n] of residual) console.log(`  ${String(n).padStart(6)}  ${f}`);
const residualTotal = residual.reduce((s, [, n]) => s + n, 0);
console.log(`  ${residual.length} files / ${residualTotal} CJK with translatable residual`);
console.log(residualTotal === 0 ? "  GATE: PASS" : "  GATE: FAIL");
