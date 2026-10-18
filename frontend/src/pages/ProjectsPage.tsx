import { ArrowLeft, BookOpen, FolderKanban, MessageSquare, MoreHorizontal, Pencil, Plus, Trash2 } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Button, EmptyState, Field, IconButton, Menu, Select, Sheet } from "../components/ui";
import { api, on } from "../lib/api";
import { errorText, useApp } from "../lib/store";
import type { Agent, Conversation, Project } from "../lib/types";
import { AgentEditor } from "./AgentsPage";
import { locale, t } from "../i18n";

const icons = ["📁", "📚", "💼", "🚀", "🏠", "🎯", "🧪", "✍️", "📈", "🛠️", "🎓", "🌱"];

function ProjectEditor({ initial, onClose }: { initial: Partial<Project>; onClose: () => void }) {
  const app = useApp();
  const [p, setP] = useState<Partial<Project>>({ icon: "📁", agentId: "", collectionId: "", ...initial });
  const [busy, setBusy] = useState(false);
  const [newAgent, setNewAgent] = useState(false);
  const set = (patch: Partial<Project>) => setP((cur) => ({ ...cur, ...patch }));

  const save = async () => {
    setBusy(true);
    try {
      const saved = await api.saveProject(p);
      if (!initial.id) app.openProject(saved.id);
      onClose();
    } catch (e) {
      app.toast(errorText(e), "error");
    } finally {
      setBusy(false);
    }
  };

  if (newAgent) return <AgentEditor initial={{}} onClose={() => setNewAgent(false)} onSaved={(a: Agent) => set({ agentId: a.id })} />;
  return (
    <Sheet open onClose={onClose} width={520}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          save();
        }}
        className="space-y-3.5"
      >
        <div className="text-[14px] font-semibold">{initial.id ? t("Editar projeto") : t("Novo projeto")}</div>
        <div className="flex gap-2">
          <Menu
            align="left"
            trigger={(toggle) => (
              <button
                type="button"
                onClick={toggle}
                title={t("Ícone")}
                className="flex size-[28px] items-center justify-center rounded-[7px] border border-line bg-card text-[16px] hover:bg-hover"
              >
                {p.icon || "📁"}
              </button>
            )}
            items={icons.map((i) => ({ label: i, onClick: () => set({ icon: i }) }))}
          />
          <Field autoFocus className="flex-1" placeholder={t("Nome do projeto")} value={p.name ?? ""} onChange={(e) => set({ name: e.target.value })} />
        </div>
        <Field className="w-full" placeholder={t("Descrição (opcional)")} value={p.description ?? ""} onChange={(e) => set({ description: e.target.value })} />
        <div className="grid grid-cols-2 gap-3 text-[12px]">
          <label className="space-y-1">
            <span className="font-semibold text-muted">{t("Agente do projeto")}</span>
            <Select
              className="w-full"
              value={p.agentId ?? ""}
              onChange={(v) => (v === "__new" ? setNewAgent(true) : set({ agentId: v }))}
              options={[
                { value: "", label: t("Assistente padrão") },
                ...app.agents.map((a) => ({ value: a.id, label: `${a.icon} ${a.name}` })),
                { value: "__new", label: t("＋ Criar agente…") },
              ]}
            />
          </label>
          <label className="space-y-1">
            <span className="font-semibold text-muted">{t("Documentos")}</span>
            <Select
              className="w-full"
              value={p.collectionId ?? ""}
              onChange={(v) => set({ collectionId: v })}
              options={[{ value: "", label: t("Os do agente / nenhum") }, ...app.collections.map((c) => ({ value: c.id, label: c.name }))]}
            />
          </label>
        </div>
        <p className="text-[11.5px] text-muted">{t("Toda conversa do projeto é respondida por esse agente e consulta esses documentos.")}</p>
        <div className="flex justify-end gap-2 pt-1">
          <Button type="button" onClick={onClose}>
            {t("Cancelar")}
          </Button>
          <Button type="submit" variant="primary" loading={busy} disabled={!p.name?.trim()}>
            {initial.id ? t("Salvar") : t("Criar projeto")}
          </Button>
        </div>
      </form>
    </Sheet>
  );
}

function ProjectDetail({ project, onEdit }: { project: Project; onEdit: () => void }) {
  const app = useApp();
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const agent = app.agents.find((a) => a.id === project.agentId);
  const collection = app.collections.find((c) => c.id === project.collectionId);

  const load = useCallback(
    () =>
      api
        .projectConversations(project.id)
        .then((l) => setConversations(l ?? []))
        .catch(() => {}),
    [project.id],
  );
  useEffect(() => {
    load();
    const offs = [on("chat:conversations", load), on("projects:changed", load)];
    return () => offs.forEach((off) => off());
  }, [load]);

  const start = () => {
    if (agent?.modelId && app.chatModels.some((m) => m.id === agent.modelId)) app.saveSettings({ lastModelId: agent.modelId });
    app.newChat({ projectId: project.id, agentId: project.agentId });
  };

  return (
    <>
      <header className="drag flex h-[52px] shrink-0 items-center gap-2 border-b border-line px-4">
        <IconButton title={t("Todos os projetos")} onClick={() => app.openProject("")}>
          <ArrowLeft className="size-4" />
        </IconButton>
        <span className="text-[16px]">{project.icon}</span>
        <h1 className="truncate text-[14px] font-semibold">{project.name}</h1>
        <div className="flex-1" />
        <div className="no-drag flex items-center gap-1">
          <Button size="sm" variant="primary" icon={<Plus className="size-3.5" />} onClick={start}>
            {t("Nova conversa")}
          </Button>
          <Menu
            trigger={(toggle) => (
              <IconButton title={t("Mais opções")} onClick={toggle}>
                <MoreHorizontal className="size-4" />
              </IconButton>
            )}
            items={[
              { label: t("Editar projeto"), icon: <Pencil className="size-3.5" />, onClick: onEdit },
              "separator",
              {
                label: t("Excluir projeto"),
                icon: <Trash2 className="size-3.5" />,
                danger: true,
                onClick: () => {
                  if (!confirm(t("Excluir o projeto “{name}”?", { name: project.name }))) return;
                  const withChats =
                    conversations.length > 0 &&
                    confirm(t("Excluir também as {n} conversas? (Cancelar = mover para a lista principal)", { n: conversations.length }));
                  api
                    .deleteProject(project.id, withChats)
                    .then(() => app.openProject(""))
                    .catch((e) => app.toast(errorText(e), "error"));
                },
              },
            ]}
          />
        </div>
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto max-w-[760px] px-6 py-6">
          {project.description && <p className="mb-3 text-[13px] text-fg/85">{project.description}</p>}
          <div className="mb-6 flex flex-wrap gap-2 text-[12px]">
            <span className="flex items-center gap-1.5 rounded-full border border-line bg-card px-2.5 py-1">
              {agent ? (
                <>
                  <span>{agent.icon}</span> {agent.name}
                </>
              ) : (
                t("Assistente padrão")
              )}
            </span>
            {collection && (
              <span className="flex items-center gap-1.5 rounded-full border border-line bg-card px-2.5 py-1">
                <BookOpen className="size-3.5 text-muted" /> {collection.name}
              </span>
            )}
          </div>
          {conversations.length === 0 ? (
            <EmptyState
              icon={<MessageSquare className="size-10" />}
              title={t("Nenhuma conversa neste projeto")}
              text={agent ? t("Comece uma conversa com {name}.", { name: agent.name }) : t("Comece a primeira conversa do projeto.")}
              action={
                <Button variant="primary" icon={<Plus className="size-3.5" />} onClick={start}>
                  {t("Nova conversa")}
                </Button>
              }
            />
          ) : (
            <div className="divide-y divide-line overflow-hidden rounded-xl border border-line bg-card">
              {conversations.map((c) => (
                <button key={c.id} onClick={() => app.openConversation(c.id)} className="block w-full px-4 py-3 text-left hover:bg-hover">
                  <div className="flex items-center justify-between gap-3">
                    <span className="truncate text-[13.5px] font-medium">{c.title}</span>
                    <span className="shrink-0 text-[11.5px] text-faint">{new Date(c.updatedAt).toLocaleDateString(locale())}</span>
                  </div>
                  {c.preview && <div className="mt-0.5 truncate text-[12px] text-muted">{c.preview}</div>}
                </button>
              ))}
            </div>
          )}
        </div>
      </div>
    </>
  );
}

export function ProjectsPage() {
  const app = useApp();
  const [editing, setEditing] = useState<Partial<Project> | null>(null);
  const current = app.projects.find((p) => p.id === app.projectId);

  return (
    <div className="flex h-full min-w-0 flex-1 flex-col bg-bg">
      {current ? (
        <ProjectDetail project={current} onEdit={() => setEditing(current)} />
      ) : (
        <>
          <header className="drag flex h-[52px] shrink-0 items-center justify-between border-b border-line px-6">
            <h1 className="text-[14px] font-semibold">{t("Projetos")}</h1>
            <div className="no-drag">
              <Button size="sm" variant="primary" icon={<Plus className="size-3.5" />} onClick={() => setEditing({})}>
                {t("Novo projeto")}
              </Button>
            </div>
          </header>
          <div className="min-h-0 flex-1 overflow-y-auto">
            <div className="mx-auto max-w-[920px] px-6 py-6">
              <p className="mb-5 text-[12.5px] text-muted">
                {t("Um projeto reúne conversas sobre o mesmo assunto, todas respondidas pelo mesmo agente e com os mesmos documentos.")}
              </p>
              {app.projects.length === 0 ? (
                <EmptyState
                  icon={<FolderKanban className="size-10" />}
                  title={t("Nenhum projeto ainda")}
                  text={t("Crie um projeto e escolha o agente que vai responder nele.")}
                  action={
                    <Button variant="primary" icon={<Plus className="size-3.5" />} onClick={() => setEditing({})}>
                      {t("Criar projeto")}
                    </Button>
                  }
                />
              ) : (
                <div className="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-3">
                  {app.projects.map((p) => {
                    const agent = app.agents.find((a) => a.id === p.agentId);
                    return (
                      <button
                        key={p.id}
                        onClick={() => app.openProject(p.id)}
                        className="flex flex-col rounded-xl border border-line bg-card p-4 text-left transition-shadow hover:shadow-mac"
                      >
                        <div className="flex items-center gap-2.5">
                          <span className="flex size-10 items-center justify-center rounded-xl bg-accent-soft text-[20px]">{p.icon || "📁"}</span>
                          <span className="min-w-0">
                            <span className="block truncate text-[14px] font-semibold">{p.name}</span>
                            <span className="block text-[11.5px] text-muted">
                              {agent ? `${agent.icon} ${agent.name}` : t("Assistente padrão")} ·{" "}
                              {p.conversations === 1 ? t("{n} conversa", { n: p.conversations }) : t("{n} conversas", { n: p.conversations })}
                            </span>
                          </span>
                        </div>
                        {p.description && <p className="mt-2 line-clamp-2 text-[12.5px] text-fg/80">{p.description}</p>}
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
        </>
      )}
      {editing && <ProjectEditor initial={editing} onClose={() => setEditing(null)} />}
    </div>
  );
}
