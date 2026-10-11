import { useState } from "react";
import {
  BookOpen,
  Brain,
  FilePenLine,
  GitBranch,
  Globe,
  ListOrdered,
  LockKeyhole,
  MapPin,
  Search,
  Users,
  Wrench,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import {
  groupAvailableTools,
  type ToolGroup,
} from "@/components/tools/catalog";
import ToolSummary from "@/components/tools/ToolSummary";
import MCPServerToolDisableDialog from "./MCPServerToolDisableDialog";

const coreTools = ["get_chapter_list", "read", "edit"];

interface Props {
  available: string[];
  selected: string[];
  disabled: boolean;
  onChange: (names: string[]) => void;
}

const groupIcons = {
  characters: Users,
  locations: MapPin,
  chapters: BookOpen,
  timeline: ListOrdered,
  storyArcs: GitBranch,
  reader: Brain,
  worldbuilding: Globe,
  files: FilePenLine,
  retrieval: Search,
  general: Wrench,
};

export default function MCPServerTools({
  available,
  selected,
  disabled,
  onChange,
}: Props) {
  const { t } = useTranslation();
  const [pendingRemoval, setPendingRemoval] = useState<string[]>([]);

  function changeSelection(next: string[]) {
    const removed = selected.filter((name) => !next.includes(name));
    if (removed.some((name) => coreTools.includes(name))) {
      setPendingRemoval(removed);
      return;
    }
    onChange(next);
  }

  return (
    <section className="space-y-4" aria-labelledby="mcp-server-tools-title">
      <div className="flex items-center justify-between gap-3">
        <h4 id="mcp-server-tools-title" className="text-sm font-semibold">
          {t("settings.mcpServer.tools")}
        </h4>
        <span className="rounded-full bg-muted px-2.5 py-1 text-xs text-muted-foreground">
          {t("settings.mcpServer.selectedCount", {
            selected: new Set(
              available.filter((name) => selected.includes(name)),
            ).size,
            total: new Set(available).size,
          })}
        </span>
      </div>
      <p className="text-xs leading-relaxed text-muted-foreground">
        {t("settings.mcpServer.toolsHint")}
      </p>
      <div className="flex gap-3 rounded-xl border bg-muted/30 p-4">
        <LockKeyhole className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
        <div className="flex-1">
          <ToolSummary name="get_current_novel" />
        </div>
        <span className="h-fit shrink-0 rounded-full border bg-background px-2 py-0.5 text-[10px] text-muted-foreground">
          {t("settings.mcpServer.builtin")}
        </span>
      </div>
      {groupAvailableTools(available).map((group) => (
        <ToolPermissionGroup
          key={group.id}
          group={group}
          selected={selected}
          disabled={disabled}
          onChange={changeSelection}
        />
      ))}
      <MCPServerToolDisableDialog
        tools={coreTools.filter((name) => pendingRemoval.includes(name))}
        disabled={disabled}
        onKeep={() => setPendingRemoval([])}
        onDisable={() => {
          if (disabled) return;
          onChange(selected.filter((name) => !pendingRemoval.includes(name)));
          setPendingRemoval([]);
        }}
      />
    </section>
  );
}

function ToolPermissionGroup({
  group,
  selected,
  disabled,
  onChange,
}: Omit<Props, "available"> & { group: ToolGroup }) {
  const { t } = useTranslation();
  const Icon = groupIcons[group.id];
  const title = t(`tools.groups.${group.id}`);
  const count = group.tools.filter((name) => selected.includes(name)).length;
  const allSelected = count === group.tools.length;
  const partiallySelected = count > 0 && !allSelected;

  function toggle(names: string[], checked: boolean) {
    onChange(
      checked
        ? [...new Set([...selected, ...names])]
        : selected.filter((name) => !names.includes(name)),
    );
  }

  return (
    <fieldset className="min-w-0 rounded-xl border bg-card">
      <legend className="sr-only">{title}</legend>
      <div className="flex flex-wrap items-center justify-between gap-2 rounded-t-xl border-b bg-muted/30 px-4 py-3">
        <div className="flex items-center gap-2">
          <Icon className="h-4 w-4 text-primary" />
          <h5 className="text-xs font-semibold">{title}</h5>
          <span className="text-[11px] tabular-nums text-muted-foreground">
            {count}/{group.tools.length}
          </span>
        </div>
        <label className="flex cursor-pointer items-center gap-2 text-xs text-muted-foreground has-[:disabled]:cursor-default">
          <input
            type="checkbox"
            aria-label={t("settings.mcpServer.selectGroup", { group: title })}
            checked={allSelected}
            ref={(node) => {
              if (node) node.indeterminate = partiallySelected;
            }}
            disabled={disabled}
            onChange={(event) => toggle(group.tools, event.target.checked)}
            className="h-3.5 w-3.5 accent-primary focus-visible:outline-primary disabled:opacity-50"
          />
          {allSelected
            ? t("settings.mcpServer.deselectAll")
            : t("settings.mcpServer.selectAll")}
        </label>
      </div>
      <div className="divide-y">
        {group.tools.map((name) => {
          const checked = selected.includes(name);
          return (
            <label
              key={name}
              className={`flex cursor-pointer items-start gap-3 px-4 py-3 transition-colors has-[:disabled]:cursor-default ${
                checked ? "bg-primary/5" : "hover:bg-muted/30"
              }`}
            >
              <input
                type="checkbox"
                aria-label={t(`tools.names.${name}`, { defaultValue: name })}
                checked={checked}
                disabled={disabled}
                onChange={(event) => toggle([name], event.target.checked)}
                className="mt-1 h-4 w-4 shrink-0 accent-primary focus-visible:outline-primary disabled:opacity-50"
              />
              <ToolSummary name={name} />
            </label>
          );
        })}
      </div>
    </fieldset>
  );
}
