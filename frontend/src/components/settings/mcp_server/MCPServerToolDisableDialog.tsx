import { useRef } from "react";
import { AlertDialog } from "radix-ui";
import { AlertTriangle } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";

interface Props {
  tools: string[];
  disabled: boolean;
  onKeep: () => void;
  onDisable: () => void;
}

export default function MCPServerToolDisableDialog({
  tools,
  disabled,
  onKeep,
  onDisable,
}: Props) {
  const { t } = useTranslation();
  const previousFocus = useRef<HTMLElement | null>(null);
  return (
    <AlertDialog.Root
      open={tools.length > 0}
      onOpenChange={(open) => {
        if (!open) onKeep();
      }}
    >
      <AlertDialog.Portal>
        <AlertDialog.Overlay className="fixed inset-0 z-[60] bg-black/40" />
        <AlertDialog.Content
          className="fixed left-1/2 top-1/2 z-[60] w-[440px] max-w-[90vw] max-h-[90vh] -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-xl border bg-background p-6 shadow-2xl"
          onOpenAutoFocus={() => {
            previousFocus.current =
              document.activeElement instanceof HTMLElement
                ? document.activeElement
                : null;
          }}
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            previousFocus.current?.focus({ preventScroll: true });
          }}
        >
          <AlertDialog.Title className="mb-3 flex items-center gap-2 text-base font-semibold">
            <AlertTriangle className="h-5 w-5 shrink-0 text-warning-foreground" />
            {t("settings.mcpServer.disableCoreToolsTitle")}
          </AlertDialog.Title>
          <AlertDialog.Description className="text-sm leading-relaxed text-muted-foreground">
            {t("settings.mcpServer.disableCoreToolsDescription")}
          </AlertDialog.Description>
          <ul className="my-4 space-y-3 rounded-lg border bg-muted/30 p-3">
            {tools.map((name) => (
              <li key={name} className="text-xs leading-relaxed">
                <span className="font-medium">{t(`tools.names.${name}`)}</span>
                <p className="mt-1 text-muted-foreground">
                  {t(`settings.mcpServer.coreToolEffects.${name}`)}
                </p>
              </li>
            ))}
          </ul>
          <div className="flex justify-end gap-2">
            <AlertDialog.Cancel asChild>
              <Button size="sm">{t("settings.mcpServer.keepCoreTools")}</Button>
            </AlertDialog.Cancel>
            <AlertDialog.Action asChild>
              <Button
                size="sm"
                variant="outline"
                disabled={disabled}
                onClick={onDisable}
              >
                {t("settings.mcpServer.disableCoreTools")}
              </Button>
            </AlertDialog.Action>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Portal>
    </AlertDialog.Root>
  );
}
