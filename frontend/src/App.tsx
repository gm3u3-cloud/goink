import { Toaster } from "sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { QueryClientProvider } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { queryClient } from "@/lib/queryClient";
import InitView from "@/views/InitView";
import WorkspaceView from "@/views/WorkspaceView";
import StartupOverlay from "@/components/startup/StartupOverlay";
import LoadingScreen from "@/components/startup/LoadingScreen";
import ConfirmDialog from "@/components/ui/ConfirmDialog";
import { useStartupState } from "@/components/startup/useStartupState";
import { useWorkspaceBoot } from "@/components/startup/useWorkspaceBoot";
import { useQuitConfirm } from "@/components/startup/useQuitConfirm";

export default function App() {
  const { t } = useTranslation();
  // The order cannot be reversed: useQuitConfirm must register the
  // app:quit-confirm listener first, and only then will useStartupState call
  // FrontendReady() in its effect to complete the handshake — once the
  // backend marks the frontend as ready, close requests are intercepted and
  // that event is emitted. Hook effects run in declaration order, so which
  // of these two lines comes first is meaningful.
  const quitConfirm = useQuitConfirm();
  const startup = useStartupState();
  const boot = useWorkspaceBoot(startup.phase, startup.version);

  function renderStartupContent() {
    switch (startup.phase) {
      case "unconfigured":
        return <InitView error={startup.error} />;
      case "failed":
        return (
          <StartupOverlay
            phase="failed"
            error={startup.error}
            onRetry={startup.retry}
            retrying={startup.retrying}
          />
        );
      case "ready":
        return boot.ready ? (
          <WorkspaceView
            initialNovelId={boot.novelId}
            initialShowHelp={startup.cameFromInit}
          />
        ) : (
          <LoadingScreen />
        );
      case "initializing":
        return startup.showPreparing ? (
          <StartupOverlay phase="initializing" />
        ) : (
          <LoadingScreen />
        );
      default:
        return <LoadingScreen />;
    }
  }

  return (
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <div className="min-h-screen bg-background text-foreground">
          <Toaster
            position="top-center"
            richColors
            toastOptions={{
              actionButtonStyle: {
                backgroundColor: "var(--primary)",
                color: "var(--primary-foreground)",
                border: "none",
                padding: "2px 10px",
                borderRadius: "4px",
                fontSize: "12px",
              },
            }}
          />
          {renderStartupContent()}
          {/* The close-window confirmation is initiated by the backend during
              migration; the dialog is mounted at the root so it is not
              unmounted when the startup screen switches.
              "Keep waiting" is the safe option, so X / overlay / Esc all map
              to it; only the red "Exit" actually exits. */}
          <ConfirmDialog
            open={quitConfirm.open}
            title={t("startup.quitConfirmTitle")}
            message={t("startup.quitConfirmMessage")}
            confirmText={t("startup.quitConfirmLeave")}
            cancelText={t("startup.quitConfirmWait")}
            danger
            onConfirm={quitConfirm.confirm}
            onClose={quitConfirm.cancel}
          />
        </div>
      </TooltipProvider>
    </QueryClientProvider>
  );
}
