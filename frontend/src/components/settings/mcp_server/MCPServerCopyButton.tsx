import { useEffect, useState } from "react";
import { Check, Copy, Loader2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { ClipboardSetText } from "@/lib/wailsjs/runtime/runtime";

interface Props {
  text: string;
  label: string;
  disabled?: boolean;
}

export default function MCPServerCopyButton({ text, label, disabled }: Props) {
  const { t } = useTranslation();
  const [copying, setCopying] = useState(false);
  const [feedback, setFeedback] = useState<{
    text: string;
    success: boolean;
  }>();
  const copied = feedback?.text === text && feedback.success;
  const failed = feedback?.text === text && !feedback.success;

  useEffect(() => {
    if (!feedback?.success || feedback.text !== text) return;
    const timer = window.setTimeout(() => setFeedback(undefined), 2000);
    return () => window.clearTimeout(timer);
  }, [feedback, text]);

  async function copy() {
    setCopying(true);
    setFeedback(undefined);
    try {
      const success = await ClipboardSetText(text);
      setFeedback({ text, success });
    } catch {
      setFeedback({ text, success: false });
    } finally {
      setCopying(false);
    }
  }

  return (
    <span className="inline-flex shrink-0 flex-col items-end gap-1">
      <Button
        variant="outline"
        size="sm"
        aria-label={label}
        disabled={disabled || copying}
        onClick={() => void copy()}
      >
        {copying ? (
          <Loader2 className="animate-spin" />
        ) : copied ? (
          <Check />
        ) : (
          <Copy />
        )}
        {copied ? t("settings.mcpServer.copied") : label}
      </Button>
      {copied && (
        <span role="status" className="sr-only">
          {t("settings.mcpServer.copied")}
        </span>
      )}
      {failed && (
        <span role="alert" className="text-xs text-destructive">
          {t("settings.mcpServer.copyFailed")}
        </span>
      )}
    </span>
  );
}
