import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { setLanguage, systemLanguage, t } from "../i18n";
import { api, on } from "./api";
import type {
  Agent,
  Project,
  ChatDraft,
  Collection,
  Conversation,
  Download,
  Hardware,
  ModelView,
  Page,
  RuntimeStatus,
  Settings,
  ToolStatus,
} from "./types";

interface Toast {
  id: number;
  text: string;
  kind: "info" | "error" | "success";
}

interface AppState {
  page: Page;
  setPage: (p: Page) => void;
  conversationId: string;
  openConversation: (id: string) => void;
  newChat: (draft?: ChatDraft) => void;
  /** Content another page handed to a new chat; consumed by the chat page. */
  chatDraft: ChatDraft | null;
  takeChatDraft: () => ChatDraft | null;
  /** Category Discover should open on. */
  discoverCategory: string;
  openDiscover: (category?: string) => void;

  platform: string;
  hardware: Hardware | null;
  settings: Settings | null;
  saveSettings: (patch: Partial<Settings>) => Promise<Settings | undefined>;
  runtime: RuntimeStatus | null;
  embedder: RuntimeStatus | null;
  tools: ToolStatus[];
  tool: (name: ToolStatus["tool"]) => ToolStatus | undefined;
  downloads: Download[];
  installed: ModelView[];
  catalogRevision: number;
  conversations: Conversation[];
  collections: Collection[];
  agents: Agent[];
  projects: Project[];
  /** Project shown in the Projects page ("" = list). */
  projectId: string;
  openProject: (id: string) => void;
  search: string;
  setSearch: (s: string) => void;

  /** Installed models usable in the chat. */
  chatModels: ModelView[];
  voiceModels: ModelView[];
  ttsModels: ModelView[];
  imageModels: ModelView[];
  videoModels: ModelView[];
  embedModels: ModelView[];
  toast: (text: string, kind?: Toast["kind"]) => void;
  toasts: Toast[];
}

const Ctx = createContext<AppState | null>(null);

export function useApp(): AppState {
  const v = useContext(Ctx);
  if (!v) throw new Error("useApp outside provider");
  return v;
}

export function errorText(e: unknown): string {
  if (typeof e === "string") return e;
  if (e instanceof Error) return e.message;
  return String(e);
}

function applyTheme(theme: Settings["theme"] | undefined) {
  const dark = theme === "dark" || (theme !== "light" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.classList.toggle("dark", dark);
}

const toolNames: Record<string, string> = {
  "tool:llamafile": "llamafile",
  "tool:whisperfile": "whisperfile",
  "tool:whisper-gpu": "whisper.cpp (GPU)",
  "tool:diffusionfile": "diffusionfile",
  "tool:sd-cpp": "stable-diffusion.cpp (GPU)",
  "tool:piper": "Piper",
};

export function AppProvider({ children }: { children: ReactNode }) {
  const [page, setPage] = useState<Page>("chat");
  const [conversationId, setConversationId] = useState("");
  const [chatDraft, setChatDraft] = useState<ChatDraft | null>(null);
  const [discoverCategory, setDiscoverCategory] = useState("chat");
  const [platform, setPlatform] = useState("linux");
  const [hardware, setHardware] = useState<Hardware | null>(null);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [runtime, setRuntime] = useState<RuntimeStatus | null>(null);
  const [embedder, setEmbedder] = useState<RuntimeStatus | null>(null);
  const [tools, setTools] = useState<ToolStatus[]>([]);
  const [downloads, setDownloads] = useState<Download[]>([]);
  const [installed, setInstalled] = useState<ModelView[]>([]);
  const [catalogRevision, setCatalogRevision] = useState(0);
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [collections, setCollections] = useState<Collection[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [projectId, setProjectId] = useState("");
  const [search, setSearch] = useState("");
  const [toasts, setToasts] = useState<Toast[]>([]);

  const toast = useCallback((text: string, kind: Toast["kind"] = "info") => {
    const id = Date.now() + Math.random();
    setToasts((t) => [...t, { id, text, kind }]);
    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), kind === "error" ? 7000 : 4000);
  }, []);

  const loadConversations = useCallback(() => {
    api.conversations(search).then((l) => setConversations(l ?? [])).catch(() => {});
  }, [search]);

  const loadCollections = useCallback(() => {
    api.collections().then((l) => setCollections(l ?? [])).catch(() => {});
  }, []);

  const loadAgents = useCallback(() => {
    api.agents().then((l) => setAgents(l ?? [])).catch(() => {});
  }, []);

  const loadProjects = useCallback(() => {
    api.projects().then((l) => setProjects(l ?? [])).catch(() => {});
  }, []);

  const loadTools = useCallback(() => {
    api.tools().then((l) => setTools(l ?? [])).catch(() => {});
  }, []);

  const refreshCatalog = useCallback(() => {
    api.installed().then((l) => setInstalled(l ?? [])).catch(() => {});
    setCatalogRevision((r) => r + 1);
  }, []);

  // Initial load.
  useEffect(() => {
    api.platform().then((p) => {
      setPlatform(p);
      document.documentElement.dataset.platform = p;
    });
    api.settings().then((s) => {
      setLanguage(s.language || systemLanguage());
      setSettings(s);
      applyTheme(s.theme);
      // First launch: remember the system language so the backend uses it too.
      if (!s.language) api.saveSettings({ ...s, language: systemLanguage() }).then(setSettings).catch(() => {});
    });
    api.runtime().then((r) => {
      setRuntime(r);
      // First launch without llamafile: take the user straight to the setup.
      if (r.state === "missing") setPage("settings");
    });
    api.embedder().then(setEmbedder);
    api.downloads().then((l) => setDownloads(l ?? []));
    api.hardware().then(setHardware);
    loadTools();
    loadCollections();
    loadAgents();
    loadProjects();
    refreshCatalog();
  }, [refreshCatalog, loadTools, loadCollections, loadAgents, loadProjects]);

  useEffect(loadConversations, [loadConversations]);

  // Backend events.
  useEffect(() => {
    const offs = [
      on<RuntimeStatus>("runtime:status", setRuntime),
      on<RuntimeStatus>("embedder:status", setEmbedder),
      on<Download[] | null>("downloads:progress", (l) => setDownloads(l ?? [])),
      on<{ key: string; error: string }>("downloads:done", ({ key, error }) => {
        if (error) toast(t("Falha no download: {error}", { error }), "error");
        else if (toolNames[key]) toast(t("{tool} instalado e pronto para uso.", { tool: toolNames[key] }), "success");
        else if (key.startsWith("model:")) toast(t("Modelo baixado com sucesso."), "success");
      }),
      on("tools:changed", loadTools),
      on("catalog:changed", refreshCatalog),
      on<Settings>("settings:changed", (s) => {
        if (s.language) setLanguage(s.language);
        setSettings(s);
        applyTheme(s.theme);
      }),
      on("chat:conversations", loadConversations),
      on("docs:changed", loadCollections),
      on("agents:changed", loadAgents),
      on("projects:changed", loadProjects),
      // "Nova conversa" in the system tray menu.
      on("app:new-chat", () => {
        setChatDraft(null);
        setConversationId("");
        setPage("chat");
      }),
    ];
    return () => offs.forEach((off) => off());
  }, [refreshCatalog, loadConversations, loadCollections, loadTools, loadAgents, loadProjects, toast]);

  // Follow the OS theme when set to "system".
  useEffect(() => {
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const fn = () => applyTheme(settings?.theme);
    mq.addEventListener("change", fn);
    return () => mq.removeEventListener("change", fn);
  }, [settings?.theme]);

  const saveSettings = useCallback(
    async (patch: Partial<Settings>) => {
      if (!settings) return;
      try {
        const saved = await api.saveSettings({ ...settings, ...patch });
        if (saved.language) setLanguage(saved.language);
        setSettings(saved);
        applyTheme(saved.theme);
        return saved;
      } catch (e) {
        toast(errorText(e), "error");
      }
    },
    [settings, toast],
  );

  const by = useCallback(
    (fn: (m: ModelView) => boolean, preferred?: string) => {
      const list = installed.filter(fn);
      const i = list.findIndex((m) => m.id === preferred);
      if (i > 0) list.unshift(...list.splice(i, 1));
      return list;
    },
    [installed],
  );
  const chatModels = useMemo(() => by((m) => m.runtime === "llamafile" && !m.categories.includes("embedding")), [by]);
  const voiceModels = useMemo(() => by((m) => m.categories.includes("voice"), settings?.voiceModelId), [by, settings?.voiceModelId]);
  const ttsModels = useMemo(() => by((m) => m.categories.includes("tts"), settings?.ttsVoiceId), [by, settings?.ttsVoiceId]);
  const imageModels = useMemo(() => by((m) => m.categories.includes("image"), settings?.imageModelId), [by, settings?.imageModelId]);
  const videoModels = useMemo(() => by((m) => m.categories.includes("video"), settings?.videoModelId), [by, settings?.videoModelId]);
  const embedModels = useMemo(() => by((m) => m.categories.includes("embedding"), settings?.embedModelId), [by, settings?.embedModelId]);

  const value: AppState = {
    page,
    setPage,
    conversationId,
    openConversation: (id) => {
      setConversationId(id);
      setPage("chat");
    },
    newChat: (draft) => {
      setChatDraft(draft ?? null);
      setConversationId("");
      setPage("chat");
    },
    chatDraft,
    takeChatDraft: () => {
      const d = chatDraft;
      if (d) setChatDraft(null);
      return d;
    },
    discoverCategory,
    openDiscover: (category) => {
      if (category) setDiscoverCategory(category);
      setPage("discover");
    },
    platform,
    hardware,
    settings,
    saveSettings,
    runtime,
    embedder,
    tools,
    tool: (name) => tools.find((t) => t.tool === name),
    downloads,
    installed,
    catalogRevision,
    conversations,
    collections,
    agents,
    projects,
    projectId,
    openProject: (id) => {
      setProjectId(id);
      setPage("projects");
    },
    search,
    setSearch,
    chatModels,
    voiceModels,
    ttsModels,
    imageModels,
    videoModels,
    embedModels,
    toast,
    toasts,
  };
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}
