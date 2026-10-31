import { useTranslation } from "react-i18next";

/** Placeholder screen during startup: the period after receiving the
 * snapshot but before it is clear which view to render. */
export default function LoadingScreen() {
  const { t } = useTranslation();

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <p className="text-muted-foreground">{t("app.loading")}</p>
    </div>
  );
}
