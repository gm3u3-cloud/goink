// Data-visualization arc color palette.
// Used by StoryArcGraph and ArcListView to distinguish different story
// arcs.
// Chart/visualization libraries do not accept CSS variables; absolute
// color values are required (see docs/frontend/theme-design.md §5§6).
// Therefore it is exempted from theme-check.mjs's oklch check (SKIP_FILES).

export interface ArcColor {
  fill: string;
  stroke: string;
  text: string;
  edge: string;
}

const PALETTE_LIGHT: ArcColor[] = [
  { fill: "#dbeafe", stroke: "#3b82f6", text: "#1d4ed8", edge: "#60a5fa" },
  { fill: "#dcfce7", stroke: "#22c55e", text: "#166534", edge: "#4ade80" },
  { fill: "#fef3c7", stroke: "#f59e0b", text: "#92400e", edge: "#fbbf24" },
  { fill: "#f3e8ff", stroke: "#a855f7", text: "#6b21a8", edge: "#c084fc" },
  { fill: "#ffe4e6", stroke: "#f43f5e", text: "#9f1239", edge: "#fb7185" },
  { fill: "#ccfbf1", stroke: "#14b8a6", text: "#115e59", edge: "#2dd4bf" },
  { fill: "#ffedd5", stroke: "#f97316", text: "#9a3412", edge: "#fb923c" },
];

const PALETTE_DARK: ArcColor[] = [
  {
    fill: "oklch(0.58 0.15 255 / 0.15)",
    stroke: "oklch(0.72 0.15 255)",
    text: "oklch(0.78 0.1 255)",
    edge: "oklch(0.72 0.15 255)",
  },
  {
    fill: "oklch(0.58 0.16 145 / 0.15)",
    stroke: "oklch(0.72 0.15 145)",
    text: "oklch(0.78 0.1 145)",
    edge: "oklch(0.72 0.15 145)",
  },
  {
    fill: "oklch(0.62 0.18 80 / 0.15)",
    stroke: "oklch(0.78 0.16 80)",
    text: "oklch(0.82 0.1 80)",
    edge: "oklch(0.78 0.16 80)",
  },
  {
    fill: "oklch(0.55 0.18 280 / 0.15)",
    stroke: "oklch(0.72 0.15 280)",
    text: "oklch(0.78 0.1 280)",
    edge: "oklch(0.72 0.15 280)",
  },
  {
    fill: "oklch(0.5 0.18 15 / 0.15)",
    stroke: "oklch(0.7 0.15 15)",
    text: "oklch(0.76 0.1 15)",
    edge: "oklch(0.7 0.15 15)",
  },
  {
    fill: "oklch(0.58 0.16 175 / 0.15)",
    stroke: "oklch(0.72 0.15 175)",
    text: "oklch(0.78 0.1 175)",
    edge: "oklch(0.72 0.15 175)",
  },
  {
    fill: "oklch(0.62 0.18 45 / 0.15)",
    stroke: "oklch(0.78 0.16 45)",
    text: "oklch(0.82 0.1 45)",
    edge: "oklch(0.78 0.16 45)",
  },
];

// arcPalette returns the palette for the given theme; unknown themes fall
// back to the light palette.
export function arcPalette(theme: string): ArcColor[] {
  return theme === "dark" ? PALETTE_DARK : PALETTE_LIGHT;
}
