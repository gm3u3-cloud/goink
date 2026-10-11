import { useTranslation } from "react-i18next";

interface Props {
  name: string;
}

export default function ToolSummary({ name }: Props) {
  const { t } = useTranslation();
  return (
    <span className="block min-w-0 space-y-1">
      <span className="block text-sm font-medium text-foreground">
        {t(`tools.names.${name}`, { defaultValue: name })}
      </span>
      <span className="block text-xs leading-relaxed text-muted-foreground">
        {t(`tools.descriptions.${name}`, {
          defaultValue: t("tools.unknownDescription"),
        })}
      </span>
      <code className="block break-all text-[10px] text-muted-foreground/70">
        {name}
      </code>
    </span>
  );
}
