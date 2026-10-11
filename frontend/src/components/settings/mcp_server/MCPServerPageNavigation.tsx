import { BookOpen, KeyRound, Settings2, Wrench } from "lucide-react";
import { useTranslation } from "react-i18next";

export type MCPServerSection = "connection" | "credentials" | "guide" | "tools";

interface Props {
  activeSection: MCPServerSection;
  onNavigate: (section: MCPServerSection) => void;
}

const sections = [
  { id: "connection", label: "navConnection", icon: Settings2 },
  { id: "credentials", label: "navCredentials", icon: KeyRound },
  { id: "guide", label: "navGuide", icon: BookOpen },
  { id: "tools", label: "navTools", icon: Wrench },
] as const;

export default function MCPServerPageNavigation({
  activeSection,
  onNavigate,
}: Props) {
  const { t } = useTranslation();

  return (
    <nav
      aria-label={t("settings.mcpServer.pageNavigation")}
      className="mb-4 grid shrink-0 grid-cols-2 gap-1 rounded-lg border bg-muted/30 p-1 @min-[480px]:grid-cols-4"
    >
      {sections.map(({ id, label, icon: Icon }) => (
        <button
          key={id}
          type="button"
          aria-controls={`mcp-server-section-${id}`}
          aria-current={activeSection === id ? "location" : undefined}
          onClick={() => onNavigate(id)}
          className={`flex items-center justify-center gap-1.5 rounded-md px-2 py-2 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
            activeSection === id
              ? "bg-background text-primary shadow-sm"
              : "text-muted-foreground hover:bg-background/60 hover:text-foreground"
          }`}
        >
          <Icon className="h-3.5 w-3.5 shrink-0" />
          {t(`settings.mcpServer.${label}`)}
        </button>
      ))}
    </nav>
  );
}
