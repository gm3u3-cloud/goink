import { useState, useRef, useEffect, type ReactNode } from "react";
import { ChevronDown } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { llm } from "@/lib/wailsjs/go/models";

interface Props {
  providers: llm.ProviderView[];
  selectedKey: string;
  onSelect: (key: string) => void;
  // The icon is rendered by the caller: Builtin passes ProviderIcon,
  // Custom passes a circular initial
  renderIcon: (key: string, name: string) => ReactNode;
  // Failure red-dot badge: shown when testResults[key].ok === false
  testResults?: Record<
    string,
    { ok: boolean; msg?: string; warning?: string } | undefined
  >;
}

/**
 * Provider selection dropdown: custom button + dropdown list.
 * Shared by Builtin and Custom; the difference is injected via renderIcon
 * (Builtin uses ProviderIcon, Custom uses a circular initial fallback).
 *
 * The red-dot failure badge on the right of each list item is shown when
 * testResults[key].ok === false, so the user can see at a glance which
 * providers are failing while switching.
 *
 * Note: dropdownOpen is reset automatically when selectedKey changes (the
 * list collapses after switching).
 */
export default function ProviderDropdown({
  providers,
  selectedKey,
  onSelect,
  renderIcon,
  testResults,
}: Props) {
  const { t } = useTranslation();
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const provider = providers.find((p) => p.key === selectedKey);

  // Reset the collapse and dropdown state when the provider changes
  useEffect(() => {
    setDropdownOpen(false);
  }, [selectedKey]);

  // Click outside to close the dropdown
  useEffect(() => {
    if (!dropdownOpen) return;
    const handle = (e: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(e.target as Node)
      ) {
        setDropdownOpen(false);
      }
    };
    document.addEventListener("mousedown", handle);
    return () => document.removeEventListener("mousedown", handle);
  }, [dropdownOpen]);

  if (!provider) return null;

  return (
    <div className="relative flex-1" ref={dropdownRef}>
      <button
        onClick={() => setDropdownOpen(!dropdownOpen)}
        className="flex items-center gap-2 w-full h-8 rounded-md border bg-background px-2.5 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
      >
        {renderIcon(provider.key, provider.name)}
        <span className="flex-1 text-left">{provider.name}</span>
        <ChevronDown
          className={`w-3.5 h-3.5 shrink-0 text-muted-foreground transition-transform duration-200 ${dropdownOpen ? "rotate-180" : ""}`}
        />
      </button>
      {dropdownOpen && (
        <div className="absolute z-50 top-full left-0 right-0 mt-1 rounded-md border bg-popover text-popover-foreground shadow-md py-1 max-h-56 overflow-auto">
          {providers.map((p) => {
            const tr = testResults?.[p.key];
            // Status badge has five states: not configured (gray) / not
            // tested (gray) / passed (green) / passed with rate limit
            // (yellow) / failed (red)
            // All colors use theme color variables
            // (--muted-foreground / --success-foreground /
            // --warning-foreground / --destructive)
            // No initial value: every branch assigns, and an initial "" would
            // be flagged by ESLint no-useless-assignment
            let badgeColor: string;
            let badgeLabel: string;
            if (!p.api_key) {
              badgeColor = "bg-muted-foreground";
              badgeLabel = t("settings.notConfigured");
            } else if (!tr) {
              badgeColor = "bg-muted-foreground";
              badgeLabel = t("settings.notTested");
            } else if (tr.ok) {
              if (tr.warning) {
                badgeColor = "bg-warning-foreground";
                badgeLabel = t("settings.passedWithWarning");
              } else {
                badgeColor = "bg-success-foreground";
                badgeLabel = t("settings.passed");
              }
            } else {
              badgeColor = "bg-destructive";
              badgeLabel = t("settings.connectionTestFailed");
            }
            return (
              <button
                key={p.key}
                onClick={() => {
                  onSelect(p.key);
                  setDropdownOpen(false);
                }}
                className={`flex items-center gap-2 w-full px-2.5 py-1.5 text-sm hover:bg-muted/50 transition-colors ${p.key === selectedKey ? "bg-muted/30" : ""}`}
              >
                {renderIcon(p.key, p.name)}
                <span>{p.name}</span>
                <span
                  className={`ml-auto w-2 h-2 rounded-full ${badgeColor} shrink-0`}
                  aria-label={badgeLabel}
                />
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
