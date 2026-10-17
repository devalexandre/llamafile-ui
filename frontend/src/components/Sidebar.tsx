import clsx from "clsx";
import {
  AudioLines,
  BookOpen,
  Compass,
  ImagePlus,
  Download,
  FileJson,
  FileText,
  HardDrive,
  MoreHorizontal,
  Pencil,
  Pin,
  PinOff,
  Search,
  Settings as SettingsIcon,
  SquarePen,
  Trash2,
} from "lucide-react";
import { useState } from "react";
import { api } from "../lib/api";
import { groupConversations } from "../lib/format";
import { errorText, useApp } from "../lib/store";
import type { Conversation, Page } from "../lib/types";
import { Button, IconButton, Menu, Sheet, StatusDot } from "./ui";

function NavItem({ page, icon, label, badge }: { page: Page; icon: React.ReactNode; label: string; badge?: React.ReactNode }) {
  const app = useApp();
  const active = app.page === page;
  return (
    <button
      onClick={() => app.setPage(page)}
      className={clsx(
        "no-drag flex h-[30px] w-full items-center gap-2.5 rounded-[7px] px-2.5 text-[13px] transition-colors",
        active ? "bg-active font-medium text-fg" : "text-fg/85 hover:bg-hover",
      )}
    >
      <span className={clsx("flex size-4 items-center justify-center", active ? "text-accent" : "text-muted")}>{icon}</span>
      <span className="flex-1 text-left">{label}</span>
      {badge}
    </button>
  );
}

function ConversationItem({ c, onRename }: { c: Conversation; onRename: (c: Conversation) => void }) {
  const app = useApp();
  const active = app.page === "chat" && app.conversationId === c.id;
  const exportAs = async (format: "md" | "json") => {
    try {
      const path = await api.exportConversation(c.id, format);
      if (path) app.toast(`Conversa exportada para ${path}`, "success");
    } catch (e) {
      app.toast(errorText(e), "error");
    }
  };
  return (
    <div
      onClick={() => app.openConversation(c.id)}
      className={clsx(
        "no-drag group flex h-[30px] cursor-default items-center gap-1 rounded-[7px] pr-1 pl-2.5 text-[13px]",
        active ? "bg-accent text-on-accent" : "text-fg hover:bg-hover",
      )}
    >
      <span className="flex-1 truncate">{c.title}</span>
      <div className={clsx("shrink-0", active ? "" : "opacity-0 group-hover:opacity-100")} onClick={(e) => e.stopPropagation()}>
        <Menu
          trigger={(toggle) => (
            <button
              onClick={toggle}
              className={clsx("flex size-6 items-center justify-center rounded-md", active ? "hover:bg-black/10" : "hover:bg-active")}
              title="Mais opções"
            >
              <MoreHorizontal className="size-4" />
            </button>
          )}
          items={[
            { label: "Renomear", icon: <Pencil className="size-3.5" />, onClick: () => onRename(c) },
            {
              label: c.pinned ? "Desafixar" : "Fixar",
              icon: c.pinned ? <PinOff className="size-3.5" /> : <Pin className="size-3.5" />,
              onClick: () => api.pin(c.id, !c.pinned),
            },
            "separator",
            { label: "Exportar como Markdown", icon: <FileText className="size-3.5" />, onClick: () => exportAs("md") },
            { label: "Exportar como JSON", icon: <FileJson className="size-3.5" />, onClick: () => exportAs("json") },
            "separator",
            {
              label: "Excluir",
              icon: <Trash2 className="size-3.5" />,
              danger: true,
              onClick: async () => {
                if (!confirm(`Excluir “${c.title}”? Essa ação não pode ser desfeita.`)) return;
                await api.deleteConversation(c.id);
                if (app.conversationId === c.id) app.newChat();
              },
            },
          ]}
        />
      </div>
    </div>
  );
}

export function Sidebar() {
  const app = useApp();
  const [renaming, setRenaming] = useState<Conversation | null>(null);
  const [title, setTitle] = useState("");
  const groups = groupConversations(app.conversations);
  const rt = app.runtime;
  const isMac = app.platform === "darwin";

  return (
    <aside className="flex h-full w-[260px] shrink-0 flex-col border-r border-line bg-sidebar">
      {/* Title bar area (traffic lights live here on macOS). */}
      <div className={clsx("drag flex shrink-0 items-center justify-end px-3", isMac ? "h-[52px]" : "h-[46px]")}>
        <IconButton title="Nova conversa (Ctrl+N)" onClick={() => app.newChat()}>
          <SquarePen className="size-[17px]" />
        </IconButton>
      </div>

      <nav className="space-y-0.5 px-2.5">
        <NavItem page="images" icon={<ImagePlus className="size-4" />} label="Imagens" />
        <NavItem page="voice" icon={<AudioLines className="size-4" />} label="Transcrição" />
        <NavItem
          page="documents"
          icon={<BookOpen className="size-4" />}
          label="Documentos"
          badge={<span className="text-[11px] text-muted">{app.collections.length || ""}</span>}
        />
        <div className="mx-2.5 my-1.5 h-px bg-line" />
        <NavItem page="discover" icon={<Compass className="size-4" />} label="Descobrir modelos" />
        <NavItem
          page="library"
          icon={<HardDrive className="size-4" />}
          label="Meus modelos"
          badge={
            app.downloads.length > 0 ? (
              <span className="flex items-center gap-1 text-[11px] text-accent">
                <Download className="size-3 animate-bounce" />
                {app.downloads.length}
              </span>
            ) : (
              <span className="text-[11px] text-muted">{app.installed.length || ""}</span>
            )
          }
        />
        <NavItem page="settings" icon={<SettingsIcon className="size-4" />} label="Configurações" />
      </nav>

      <div className="px-2.5 pt-3 pb-1">
        <div className="no-drag flex h-[28px] items-center gap-1.5 rounded-[7px] bg-field px-2">
          <Search className="size-3.5 text-muted" />
          <input
            value={app.search}
            onChange={(e) => app.setSearch(e.target.value)}
            placeholder="Buscar conversas"
            className="w-full bg-transparent text-[13px] outline-none placeholder:text-faint"
          />
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-2.5 pb-3">
        {groups.length === 0 && (
          <p className="px-2 pt-6 text-center text-[12px] text-muted">
            {app.search ? "Nenhuma conversa encontrada." : "Suas conversas aparecerão aqui."}
          </p>
        )}
        {groups.map((g) => (
          <div key={g.label} className="mt-3">
            <div className="px-2.5 pb-1 text-[11px] font-semibold text-muted">{g.label}</div>
            <div className="space-y-px">
              {g.items.map((c) => (
                <ConversationItem
                  key={c.id}
                  c={c}
                  onRename={(c) => {
                    setRenaming(c);
                    setTitle(c.title);
                  }}
                />
              ))}
            </div>
          </div>
        ))}
      </div>

      {/* Server status */}
      <button
        onClick={() => app.setPage("settings")}
        className="no-drag mx-2.5 mb-2.5 flex items-center gap-2.5 rounded-[9px] px-2.5 py-2 text-left hover:bg-hover"
      >
        <StatusDot state={rt?.state ?? "stopped"} />
        <div className="min-w-0 flex-1">
          <div className="truncate text-[12px] font-medium">
            {rt?.state === "running" || rt?.state === "starting" ? rt.modelName : "llamafile"}
          </div>
          <div className="truncate text-[11px] text-muted">
            {rt?.state === "running" && `Pronto · porta ${rt.port}`}
            {rt?.state === "starting" && "Carregando modelo…"}
            {rt?.state === "stopped" && `Parado · v${rt.version}`}
            {rt?.state === "missing" && "Não instalado — configurar"}
            {rt?.state === "error" && (rt.message || "Erro")}
          </div>
        </div>
      </button>

      <Sheet open={!!renaming} onClose={() => setRenaming(null)}>
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            if (!renaming) return;
            await api.rename(renaming.id, title);
            setRenaming(null);
          }}
        >
          <div className="mb-3 text-[14px] font-semibold">Renomear conversa</div>
          <input
            autoFocus
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className="mb-4 h-[30px] w-full rounded-[7px] border border-line bg-card px-2.5 outline-none focus:border-accent focus:ring-3 focus:ring-accent-soft"
          />
          <div className="flex justify-end gap-2">
            <Button type="button" onClick={() => setRenaming(null)}>
              Cancelar
            </Button>
            <Button type="submit" variant="primary">
              Renomear
            </Button>
          </div>
        </form>
      </Sheet>
    </aside>
  );
}
