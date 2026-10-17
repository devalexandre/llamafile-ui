import { Component, StrictMode, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import "./index.css";
import { AppProvider } from "./lib/store";

/** Shows the error instead of a blank window if rendering crashes. */
class ErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null };
  static getDerivedStateFromError(error: Error) {
    return { error };
  }
  render() {
    if (!this.state.error) return this.props.children;
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 p-8 text-center">
        <div className="text-[15px] font-semibold">Algo deu errado na interface</div>
        <pre className="selectable max-w-xl overflow-auto rounded-lg bg-field p-3 text-left font-mono text-[11.5px] text-muted">
          {this.state.error.message}
        </pre>
        <button className="rounded-[7px] bg-accent px-3.5 py-1.5 text-[13px] font-medium text-on-accent" onClick={() => location.reload()}>
          Recarregar
        </button>
      </div>
    );
  }
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ErrorBoundary>
      <AppProvider>
        <App />
      </AppProvider>
    </ErrorBoundary>
  </StrictMode>,
);
