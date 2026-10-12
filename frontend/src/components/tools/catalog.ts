export interface ToolGroup {
  id:
    | "characters"
    | "locations"
    | "chapters"
    | "timeline"
    | "storyArcs"
    | "reader"
    | "worldbuilding"
    | "files"
    | "retrieval"
    | "general";
  tools: string[];
}

export const toolGroups: ToolGroup[] = [
  {
    id: "characters",
    tools: [
      "get_characters",
      "create_character",
      "update_character",
      "get_character_relations",
      "update_character_relationship",
    ],
  },
  {
    id: "locations",
    tools: [
      "get_locations",
      "create_location",
      "update_location",
      "create_location_relation",
      "update_location_relation",
    ],
  },
  {
    id: "chapters",
    tools: [
      "get_volumes",
      "create_volume",
      "get_chapter_list",
      "update_chapter_plan",
    ],
  },
  {
    id: "timeline",
    tools: ["get_timeline", "create_timeline_entry", "update_timeline_entry"],
  },
  {
    id: "storyArcs",
    tools: [
      "get_story_arcs",
      "create_story_arc",
      "update_story_arc",
      "create_arc_node",
      "update_arc_node",
    ],
  },
  {
    id: "reader",
    tools: [
      "get_reader_perspective",
      "create_reader_perspective_entry",
      "update_reader_perspective_entry",
    ],
  },
  {
    id: "worldbuilding",
    tools: ["upsert_setting", "upsert_preference"],
  },
  { id: "files", tools: ["read", "edit"] },
  {
    id: "retrieval",
    tools: ["search_story_memory", "web_search", "web_fetch"],
  },
  { id: "general", tools: ["delete_record", "run_subagent"] },
];

export function groupAvailableTools(available: string[]): ToolGroup[] {
  const remaining = new Set(available);
  const groups = toolGroups.map((group) => {
    const tools = group.tools.filter((name) => remaining.delete(name));
    return { ...group, tools };
  });
  if (remaining.size) {
    groups.find((group) => group.id === "general")!.tools.push(...remaining);
  }
  return groups.filter((group) => group.tools.length > 0);
}
