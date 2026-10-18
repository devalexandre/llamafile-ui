import clsx from "clsx";
import { AlertCircle, CheckCircle2, Info, Zap } from "lucide-react";
import { useEffect } from "react";
import { Sidebar } from "./components/Sidebar";
import { Button } from "./components/ui";
import { useApp } from "./lib/store";
import { AgentsPage } from "./pages/AgentsPage";
import { ChatPage } from "./pages/ChatPage";
import { ProjectsPage } from "./pages/ProjectsPage";
import { DiscoverPage } from "./pages/DiscoverPage";
import { DocumentsPage } from "./pages/DocumentsPage";
import { ImagesPage } from "./pages/ImagesPage";
import { LibraryPage } from "./pages/LibraryPage";
import { SettingsPage } from "./pages/SettingsPage";
import { VoicePage } from "./pages/VoicePage";

function MissingRuntimeBanner() {
  const app = useApp();
  if (app.runtime?.state !== "missing" || app.page === "settings") return null;
  const downloading = app.downloads.some((d) => d.key === "runtime:llamafile");
  return (
    <div className="flex items-center gap-3 border-b border-line bg-accent-soft px-5 py-2 text-[12.5px]">
      <Zap className="size-4 text-accent" />
      <span className="flex-1">
        {downloading ? "Baixando o llamafile…" : "O llamafile ainda não está instalado. Ele é necessário para rodar os modelos."}
      </span>
      <Button size="sm" variant="primary" onClick={() => app.setPage("settings")}>
        {downloading ? "Ver progresso" : "Instalar agora"}
      </Button>
    </div>
  );
}

function Toasts() {
  const { toasts } = useApp();
  return (
    <div className="pointer-events-none fixed right-4 bottom-4 z-[60] flex w-[340px] flex-col gap-2">
      {toasts.map((t) => (
        <div
          key={t.id}
          className="pointer-events-auto flex items-start gap-2.5 rounded-xl border border-line bg-elevated/95 px-3.5 py-2.5 text-[12.5px] shadow-xl backdrop-blur-xl"
        >
          {t.kind === "error" && <AlertCircle className="mt-px size-4 shrink-0 text-bad" />}
          {t.kind === "success" && <CheckCircle2 className="mt-px size-4 shrink-0 text-ok" />}
          {t.kind === "info" && <Info className="mt-px size-4 shrink-0 text-accent" />}
          <span className="selectable break-words">{t.text}</span>
        </div>
      ))}
    </div>
  );
}

export default function App() {
  const app = useApp();

  useEffect(() => {
    const fn = (e: KeyboardEvent) => {
      const mod = e.metaKey || e.ctrlKey;
      if (mod && e.key.toLowerCase() === "n") {
        e.preventDefault();
        app.newChat();
      } else if (mod && e.key === ",") {
        e.preventDefault();
        app.setPage("settings");
      }
    };
    window.addEventListener("keydown", fn);
    return () => window.removeEventListener("keydown", fn);
  }, [app]);

  return (
    <div className="flex h-full">
      <Sidebar key={app.settings?.language ?? ""} />
      {/* Remount the pages when the language changes so every text is re-translated. */}
      <main key={app.settings?.language ?? ""} className="flex min-w-0 flex-1 flex-col">
        <MissingRuntimeBanner />
        <div className={clsx("flex min-h-0 flex-1")}>
          {app.page === "chat" && <ChatPage />}
          {app.page === "projects" && <ProjectsPage />}
          {app.page === "agents" && <AgentsPage />}
          {app.page === "images" && <ImagesPage />}
          {app.page === "voice" && <VoicePage />}
          {app.page === "documents" && <DocumentsPage />}
          {app.page === "discover" && <DiscoverPage />}
          {app.page === "library" && <LibraryPage />}
          {app.page === "settings" && <SettingsPage />}
        </div>
      </main>
      <Toasts />
    </div>
  );
}
