import { Component, type ReactNode } from "react";
import i18n from "@/i18n";

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
  error?: Error;
}

// ErrorBoundary catches JavaScript errors during the render phase of the
// child component tree, preventing the whole React tree from crashing to a
// blank screen.
// Note: ErrorBoundary must be implemented as a class component; React does not
// provide a hook equivalent.
export default class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error("ErrorBoundary caught:", error, errorInfo);
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback;
      return (
        <div className="flex h-full items-center justify-center p-8">
          <div className="text-center max-w-sm">
            <p className="text-sm font-medium text-rose-500 mb-2">
              {i18n.t("common.viewLoadFailed")}
            </p>
            <p className="text-xs text-muted-foreground mb-4 break-all">
              {this.state.error?.message || i18n.t("common.unknownError")}
            </p>
            <button
              onClick={() =>
                this.setState({ hasError: false, error: undefined })
              }
              className="text-xs text-tag-blue-foreground hover:underline"
            >
              {i18n.t("common.retry")}
            </button>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}
