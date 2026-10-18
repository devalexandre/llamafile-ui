import { Bot, MessageSquare, MoreHorizontal, Pencil, Plus, Sparkles, Trash2 } from "lucide-react";
import { useState } from "react";
import { Button, EmptyState, Field, IconButton, Menu, Select, Sheet, Switch } from "../components/ui";
import { api } from "../lib/api";
import { errorText, useApp } from "../lib/store";
import type { Agent } from "../lib/types";

/** Ready-made agents to start from. */
const templates: Partial<Agent>[] = [
  {
    icon: "📝",
    name: "Revisor de texto",
    description: "Corrige gramática, clareza e tom, explicando as mudanças.",
    systemPrompt:
      "Você é um revisor de textos em português do Brasil. Corrija gramática, ortografia, pontuação e clareza, preservando o estilo do autor. Devolva primeiro o texto revisado e, depois, uma lista curta das principais mudanças.",
    temperature: 0.2,
  },
  {
    icon: "🧑‍💻",
    name: "Programador",
    description: "Escreve e revisa código com exemplos completos e testados.",
    systemPrompt:
      "Você é um engenheiro de software sênior. Responda com código completo e funcional, explique decisões importantes em poucas linhas, aponte riscos (segurança, desempenho, casos de borda) e sugira testes. Use blocos de código com a linguagem indicada.",
    temperature: 0.2,
  },
  {
    icon: "🌎",
    name: "Tradutor",
    description: "Traduz entre português, inglês e espanhol mantendo o tom.",
    systemPrompt:
      "Você é um tradutor profissional. Detecte o idioma do texto: se estiver em português, traduza para o inglês; caso contrário, traduza para o português do Brasil. Mantenha tom, formatação e termos técnicos. Responda só com a tradução.",
    temperature: 0.3,
  },
  {
    icon: "🎓",
    name: "Professor paciente",
    description: "Explica qualquer assunto passo a passo, com exemplos.",
    systemPrompt:
      "Você é um professor paciente. Explique o assunto em etapas curtas, do básico ao avançado, com exemplos do dia a dia. No fim, faça uma pergunta rápida para checar o entendimento.",
    temperature: 0.6,
  },
  {
    icon: "🇺🇸",
    name: "Professor de inglês",
    description: "Conversa em inglês no seu nível, corrige erros e ensina vocabulário.",
    systemPrompt:
      "Você é um professor de inglês paciente e motivador para falantes de português do Brasil. Converse em inglês, adaptando o vocabulário e a velocidade ao nível do aluno (pergunte o nível no início se não souber). " +
      "Depois de cada mensagem do aluno: 1) responda naturalmente em inglês para manter a conversa; 2) em uma seção \"✏️ Correções\", mostre os erros da mensagem dele com a forma correta e uma explicação curta em português; " +
      "3) quando útil, sugira 1 a 3 expressões ou palavras novas com exemplo. Se o aluno escrever em português, ajude a dizer aquilo em inglês. " +
      "Termine com uma pergunta para continuar a prática. Ofereça exercícios, simulações (entrevista de emprego, viagem, reunião) ou explicações de gramática quando o aluno pedir.",
    temperature: 0.6,
  },
  {
    icon: "📋",
    name: "Resumidor",
    description: "Resume textos e documentos em tópicos objetivos.",
    systemPrompt:
      "Resuma o conteúdo enviado em português: uma frase com a ideia central, depois de 3 a 7 tópicos com os pontos principais e, se houver, decisões ou próximos passos. Não invente informações.",
    temperature: 0.2,
  },
];

const icons = ["🤖", "📝", "🧑‍💻", "🌎", "🎓", "📋", "💡", "📊", "⚖️", "🩺", "🍳", "✈️", "🎨", "📣", "🔍", "🧮"];

export function AgentEditor({ initial, onClose, onSaved }: { initial: Partial<Agent>; onClose: () => void; onSaved?: (a: Agent) => void }) {
  const app = useApp();
  const [a, setA] = useState<Partial<Agent>>({ icon: "🤖", temperature: -1, modelId: "", collectionId: "", ...initial });
  const [busy, setBusy] = useState(false);
  const set = (patch: Partial<Agent>) => setA((cur) => ({ ...cur, ...patch }));
  const customTemp = (a.temperature ?? -1) >= 0;

  const save = async () => {
    setBusy(true);
    try {
      const saved = await api.saveAgent(a);
      app.toast(initial.id ? "Agente atualizado." : `Agente “${saved.name}” criado.`, "success");
      onSaved?.(saved);
      onClose();
    } catch (e) {
      app.toast(errorText(e), "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Sheet open onClose={onClose} width={560}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          save();
        }}
        className="space-y-3.5"
      >
        <div className="text-[14px] font-semibold">{initial.id ? "Editar agente" : "Novo agente"}</div>
        <div className="flex gap-2">
          <Menu
            align="left"
            trigger={(toggle) => (
              <button
                type="button"
                onClick={toggle}
                title="Ícone"
                className="flex size-[28px] items-center justify-center rounded-[7px] border border-line bg-card text-[16px] hover:bg-hover"
              >
                {a.icon || "🤖"}
              </button>
            )}
            items={icons.map((i) => ({ label: i, onClick: () => set({ icon: i }) }))}
          />
          <Field autoFocus className="flex-1" placeholder="Nome (ex.: Revisor de contratos)" value={a.name ?? ""} onChange={(e) => set({ name: e.target.value })} />
        </div>
        <Field
          className="w-full"
          placeholder="Descrição curta (opcional)"
          value={a.description ?? ""}
          onChange={(e) => set({ description: e.target.value })}
        />
        <div>
          <div className="mb-1 text-[12px] font-semibold text-muted">Instruções</div>
          <textarea
            value={a.systemPrompt ?? ""}
            onChange={(e) => set({ systemPrompt: e.target.value })}
            rows={7}
            placeholder="Diga quem o agente é, como deve responder, o que evitar e o formato das respostas."
            className="w-full resize-y rounded-[8px] border border-line bg-card px-2.5 py-2 text-[13px] outline-none focus:border-accent focus:ring-3 focus:ring-accent-soft"
          />
        </div>
        <div className="grid grid-cols-2 gap-3 text-[12px]">
          <label className="space-y-1">
            <span className="font-semibold text-muted">Modelo</span>
            <Select
              className="w-full"
              value={a.modelId ?? ""}
              onChange={(v) => set({ modelId: v })}
              options={[{ value: "", label: "O escolhido no chat" }, ...app.chatModels.map((m) => ({ value: m.id, label: m.name }))]}
            />
          </label>
          <label className="space-y-1">
            <span className="font-semibold text-muted">Documentos</span>
            <Select
              className="w-full"
              value={a.collectionId ?? ""}
              onChange={(v) => set({ collectionId: v })}
              options={[{ value: "", label: "Nenhum" }, ...app.collections.map((c) => ({ value: c.id, label: c.name }))]}
            />
          </label>
        </div>
        <div className="flex items-center gap-3 text-[12px]">
          <span className="font-semibold text-muted">Criatividade própria</span>
          <Switch checked={customTemp} onChange={(v) => set({ temperature: v ? 0.7 : -1 })} />
          {customTemp ? (
            <>
              <input
                type="range"
                min={0}
                max={1.5}
                step={0.1}
                value={a.temperature}
                onChange={(e) => set({ temperature: Number(e.target.value) })}
                className="flex-1 accent-[var(--accent)]"
              />
              <span className="w-7 text-right tabular-nums">{(a.temperature ?? 0).toFixed(1)}</span>
            </>
          ) : (
            <span className="text-faint">usa a das Configurações</span>
          )}
        </div>
        <div className="flex justify-end gap-2 pt-1">
          <Button type="button" onClick={onClose}>
            Cancelar
          </Button>
          <Button type="submit" variant="primary" loading={busy} disabled={!a.name?.trim() || !a.systemPrompt?.trim()}>
            {initial.id ? "Salvar" : "Criar agente"}
          </Button>
        </div>
      </form>
    </Sheet>
  );
}

function AgentCard({ a, onEdit }: { a: Agent; onEdit: () => void }) {
  const app = useApp();
  const model = app.chatModels.find((m) => m.id === a.modelId);
  const collection = app.collections.find((c) => c.id === a.collectionId);
  return (
    <div className="group flex flex-col rounded-xl border border-line bg-card p-4 transition-shadow hover:shadow-mac">
      <div className="flex items-start gap-3">
        <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-accent-soft text-[20px]">{a.icon || "🤖"}</div>
        <div className="min-w-0 flex-1">
          <div className="truncate text-[14px] font-semibold">{a.name}</div>
          <div className="text-[11.5px] text-muted">
            {model ? model.name : "Modelo do chat"}
            {collection && ` · 📚 ${collection.name}`}
            {a.uses > 0 && ` · ${a.uses} ${a.uses === 1 ? "conversa" : "conversas"}`}
          </div>
        </div>
        <Menu
          trigger={(toggle) => (
            <IconButton title="Mais opções" onClick={toggle}>
              <MoreHorizontal className="size-4" />
            </IconButton>
          )}
          items={[
            { label: "Editar", icon: <Pencil className="size-3.5" />, onClick: onEdit },
            {
              label: "Duplicar",
              icon: <Plus className="size-3.5" />,
              onClick: () => api.saveAgent({ ...a, id: "", name: `${a.name} (cópia)` }).catch((e) => app.toast(errorText(e), "error")),
            },
            "separator",
            {
              label: "Excluir",
              icon: <Trash2 className="size-3.5" />,
              danger: true,
              onClick: () => {
                if (confirm(`Excluir o agente “${a.name}”? As conversas continuam, com o assistente padrão.`))
                  api.deleteAgent(a.id).catch((e) => app.toast(errorText(e), "error"));
              },
            },
          ]}
        />
      </div>
      <p className="mt-2 mb-3 line-clamp-2 min-h-[36px] text-[12.5px] leading-snug text-fg/80">{a.description || a.systemPrompt}</p>
      <div className="mt-auto flex gap-2">
        <Button
          size="sm"
          variant="primary"
          icon={<MessageSquare className="size-3.5" />}
          onClick={() => {
            if (a.modelId && app.chatModels.some((m) => m.id === a.modelId)) app.saveSettings({ lastModelId: a.modelId });
            app.newChat({ agentId: a.id });
          }}
        >
          Conversar
        </Button>
        <Button size="sm" icon={<Pencil className="size-3.5" />} onClick={onEdit}>
          Editar
        </Button>
      </div>
    </div>
  );
}

export function AgentsPage() {
  const app = useApp();
  const [editing, setEditing] = useState<Partial<Agent> | null>(null);
  return (
    <div className="flex h-full min-w-0 flex-1 flex-col bg-bg">
      <header className="drag flex h-[52px] shrink-0 items-center justify-between border-b border-line px-6">
        <h1 className="text-[14px] font-semibold">Agentes</h1>
        <div className="no-drag">
          <Button size="sm" variant="primary" icon={<Plus className="size-3.5" />} onClick={() => setEditing({})}>
            Novo agente
          </Button>
        </div>
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto max-w-[920px] px-6 py-6">
          <p className="mb-5 text-[12.5px] text-muted">
            Agentes são assistentes com nome e instruções próprias. Use-os em qualquer conversa (pelo seletor no topo do chat) ou
            como o agente fixo de um projeto.
          </p>
          {app.agents.length === 0 ? (
            <EmptyState
              icon={<Bot className="size-10" />}
              title="Nenhum agente ainda"
              text="Crie um do zero ou comece por um dos modelos abaixo."
              action={
                <Button variant="primary" icon={<Plus className="size-3.5" />} onClick={() => setEditing({})}>
                  Criar agente
                </Button>
              }
            />
          ) : (
            <div className="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-3">
              {app.agents.map((a) => (
                <AgentCard key={a.id} a={a} onEdit={() => setEditing(a)} />
              ))}
            </div>
          )}

          <div className="mt-8 mb-2 flex items-center gap-1.5 text-[12px] font-semibold text-accent">
            <Sparkles className="size-3.5" /> Comece por um modelo
          </div>
          <div className="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-2">
            {templates.map((t) => (
              <button
                key={t.name}
                onClick={() => setEditing(t)}
                className="flex items-start gap-2.5 rounded-xl border border-dashed border-line px-3 py-2.5 text-left hover:bg-hover"
              >
                <span className="text-[18px]">{t.icon}</span>
                <span className="min-w-0">
                  <span className="block text-[13px] font-medium">{t.name}</span>
                  <span className="block text-[11.5px] text-muted">{t.description}</span>
                </span>
              </button>
            ))}
          </div>
        </div>
      </div>
      {editing && <AgentEditor initial={editing} onClose={() => setEditing(null)} />}
    </div>
  );
}
