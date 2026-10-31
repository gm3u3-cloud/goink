// Panel routing identifier. activePanel is one of the 13 panels rendered in
// the main area.
// Source: the 10 dedicated Views excluded by WorkspaceView's negation chain
// (characters/locations/storyarcs/timeline/reader/preferences/
// novel-settings/profile/git/style-samples) + the BookshelfView branch
// (novels) + ContentPanel defaults (chapters, skills).
export type PanelId =
  | "novels"
  | "chapters"
  | "chapter-management"
  | "characters"
  | "locations"
  | "storyarcs"
  | "timeline"
  | "reader"
  | "preferences"
  | "novel-settings"
  | "profile"
  | "git"
  | "skills"
  | "style-samples";

// The sidebar can additionally be "search" (the search panel overlays inside
// SidePanel).
export type SidebarPanelId = PanelId | "search";
