import clsx from "clsx";
import {
  ArrowUp,
  BookOpen,
  Brain,
  Check,
  ChevronDown,
  Copy,
  FileText,
  Image as ImageIcon,
  Loader2,
  Mic,
  Pencil,
  RotateCcw,
  Settings2,
  Sparkles,
  Square,
  X,
} from "lucide-react";
import { memo, useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import rehypeHighlight from "rehype-highlight";
import remarkGfm from "remark-gfm";
import { Button, EmptyState, Menu } from "../components/ui";
import { api, attachmentURL, on } from "../lib/api";
import { errorText, useApp } from "../lib/store";
import type { ChatDraft, Message, ModelView, Source } from "../lib/types";
import { clock, useRecorder } from "../lib/useRecorder";
import { LevelBars } from "./VoicePage";

const suggestions = [
  "Explique como funciona a quantização de modelos",
  "Escreva um e-mail curto pedindo uma reunião",
  "Crie uma função em Go que inverta uma string",
  "Resuma os prós e contras de rodar IA localmente",
];

function ModelPicker({ models, value, onChange }: { models: ModelView[]; value: string; onChange: (id: string) => void }) {
  const current = models.find((m) => m.id === value);
  return (
    <Menu
      align="left"
      trigger={(toggle) => (
        <button
          onClick={toggle}
          className="no-drag flex h-[28px] items-center gap-1.5 rounded-[7px] px-2.5 text-[14px] font-semibold hover:bg-hover"
        >
          {current ? current.name : "Escolher modelo"}
          {current?.vision && <ImageIcon className="size-3.5 text-muted" />}
          <ChevronDown className="size-3.5 text-muted" />
        </button>
      )}
      items={models.map((m) => ({
        label: `${m.name}${m.installedQuant ? ` · ${m.installedQuant}` : ""}`,
        icon: m.id === value ? <Check className="size-3.5" /> : undefined,
        onClick: () => onChange(m.id),
      }))}
    />
  );
}

function CopyButton({ text }: { text: string }) {
  const [done, setDone] = useState(false);
  return (
    <button
      title="Copiar"
      onClick={() => {
        navigator.clipboard.writeText(text);
        setDone(true);
        setTimeout(() => setDone(false), 1200);
      }}
      className="flex size-6 items-center justify-center rounded-md text-muted hover:bg-hover hover:text-fg"
    >
      {done ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
    </button>
  );
}

const Markdown = memo(function Markdown({ text }: { text: string }) {
  return (
    <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[[rehypeHighlight, { detect: true, ignoreMissing: true }]]}>
      {text}
    </ReactMarkdown>
  );
});

function Reasoning({ text, streaming }: { text: string; streaming: boolean }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="mb-2">
      <button onClick={() => setOpen((o) => !o)} className="flex items-center gap-1.5 text-[12px] font-medium text-muted hover:text-fg">
        <Brain className={clsx("size-3.5", streaming && "animate-pulse")} />
        {streaming ? "Pensando…" : "Raciocínio"}
        <ChevronDown className={clsx("size-3 transition-transform", open && "rotate-180")} />
      </button>
      {open && (
        <div className="selectable mt-1.5 border-l-2 border-line pl-3 text-[12.5px] whitespace-pre-wrap text-muted">{text}</div>
      )}
    </div>
  );
}

function MessageRow({
  m,
  streaming,
  isLast,
  onRegenerate,
  onEdit,
  busy,
}: {
  m: Message;
  streaming: boolean;
  isLast: boolean;
  onRegenerate: () => void;
  onEdit: (text: string) => void;
  busy: boolean;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(m.content);

  if (m.role === "user") {
    return (
      <div className="group flex flex-col items-end">
        {m.attachments.length > 0 && (
          <div className="mb-1.5 flex flex-wrap justify-end gap-1.5">
            {m.attachments.map((a) => (
              <img key={a} src={attachmentURL(a)} className="max-h-40 rounded-xl border border-line object-cover" />
            ))}
          </div>
        )}
        {editing ? (
          <div className="w-full max-w-[85%] rounded-2xl border border-line bg-card p-2">
            <textarea
              autoFocus
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              rows={Math.min(10, draft.split("\n").length + 1)}
              className="w-full resize-none bg-transparent px-1.5 text-[14px] outline-none"
            />
            <div className="flex justify-end gap-2">
              <Button size="sm" onClick={() => setEditing(false)}>
                Cancelar
              </Button>
              <Button
                size="sm"
                variant="primary"
                onClick={() => {
                  setEditing(false);
                  onEdit(draft);
                }}
              >
                Enviar
              </Button>
            </div>
          </div>
        ) : (
          m.content && (
            <div className="selectable max-w-[85%] rounded-[18px] bg-bubble px-3.5 py-2 text-[14px] leading-relaxed whitespace-pre-wrap">
              {m.content}
            </div>
          )
        )}
        {!editing && (
          <div className="mt-1 flex gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
            <CopyButton text={m.content} />
            {!busy && (
              <button
                title="Editar"
                onClick={() => {
                  setDraft(m.content);
                  setEditing(true);
                }}
                className="flex size-6 items-center justify-center rounded-md text-muted hover:bg-hover hover:text-fg"
              >
                <Pencil className="size-3.5" />
              </button>
            )}
          </div>
        )}
      </div>
    );
  }

  const waiting = streaming && !m.content && !m.reasoning;
  return (
    <div className="group">
      {m.reasoning && <Reasoning text={m.reasoning} streaming={streaming && !m.content} />}
      {waiting ? (
        <div className="flex items-center gap-2 py-1 text-[13px] text-muted">
          <Loader2 className="size-3.5 animate-spin" /> Gerando…
        </div>
      ) : (
        <div
          className={clsx(
            "selectable prose prose-sm max-w-none text-[14px] leading-relaxed dark:prose-invert prose-headings:font-semibold prose-p:my-2 prose-pre:my-3",
            streaming && "caret",
          )}
        >
          <Markdown text={m.content} />
        </div>
      )}
      {m.error && (
        <div className="mt-2 rounded-lg bg-bad-soft px-3 py-2 text-[12.5px] text-bad">{m.error}</div>
      )}
      {m.sources?.length > 0 && <Sources sources={m.sources} />}
      {!streaming && (
        <div className="mt-1 flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
          <CopyButton text={m.content} />
          {isLast && !busy && (
            <button
              title="Gerar novamente"
              onClick={onRegenerate}
              className="flex size-6 items-center justify-center rounded-md text-muted hover:bg-hover hover:text-fg"
            >
              <RotateCcw className="size-3.5" />
            </button>
          )}
          {m.tps > 0 && (
            <span className="ml-1.5 text-[11px] text-faint">
              {m.tps.toFixed(1)} tok/s · {m.tokens} tokens
            </span>
          )}
        </div>
      )}
    </div>
  );
}

/** Document passages the answer was grounded on (RAG). */
function Sources({ sources }: { sources: Source[] }) {
  const [open, setOpen] = useState<number | null>(null);
  const current = sources.find((s) => s.n === open);
  return (
    <div className="mt-2.5">
      <div className="flex flex-wrap items-center gap-1.5">
        <span className="text-[11.5px] font-medium text-muted">Fontes</span>
        {sources.map((s) => (
          <button
            key={s.n}
            onClick={() => setOpen(open === s.n ? null : s.n)}
            title={s.text.slice(0, 300)}
            className={clsx(
              "flex max-w-[220px] items-center gap-1 rounded-full border px-2 py-[2px] text-[11.5px]",
              open === s.n ? "border-accent bg-accent-soft text-accent" : "border-line bg-card text-fg/80 hover:bg-hover",
            )}
          >
            <span className="font-semibold">[{s.n}]</span>
            <FileText className="size-3 shrink-0" />
            <span className="truncate">{s.document}</span>
          </button>
        ))}
      </div>
      {current && (
        <div className="selectable mt-2 rounded-lg border border-line bg-field p-3 text-[12.5px] whitespace-pre-wrap text-fg/85">
          <div className="mb-1 text-[11px] font-semibold text-muted">
            {current.document} · trecho {current.chunk + 1} · {Math.round(current.score * 100)}% similar
          </div>
          {current.text}
        </div>
      )}
    </div>
  );
}

function CollectionPicker({ value, onChange }: { value: string; onChange: (id: string) => void }) {
  const app = useApp();
  const current = app.collections.find((c) => c.id === value);
  return (
    <Menu
      align="left"
      trigger={(toggle) => (
        <button
          onClick={toggle}
          title="Conversar com documentos"
          className={clsx(
            "flex h-8 items-center gap-1.5 rounded-full px-2.5 text-[12.5px]",
            current ? "bg-accent-soft text-accent" : "text-muted hover:bg-hover hover:text-fg",
          )}
        >
          <BookOpen className="size-[17px]" />
          {current && <span className="max-w-[160px] truncate font-medium">{current.name}</span>}
        </button>
      )}
      items={[
        { label: "Sem documentos", icon: !value ? <Check className="size-3.5" /> : undefined, onClick: () => onChange("") },
        ...app.collections.map((c) => ({
          label: `${c.name} (${c.documents})`,
          icon: c.id === value ? <Check className="size-3.5" /> : <BookOpen className="size-3.5" />,
          onClick: () => onChange(c.id),
        })),
        "separator" as const,
        { label: "Gerenciar documentos…", icon: <Settings2 className="size-3.5" />, onClick: () => app.setPage("documents") },
      ]}
    />
  );
}

function Composer({
  disabled,
  busy,
  vision,
  collectionId,
  onCollection,
  draft,
  onSend,
  onStop,
}: {
  disabled: boolean;
  busy: boolean;
  vision: boolean;
  collectionId: string;
  onCollection: (id: string) => void;
  draft: ChatDraft | null;
  onSend: (text: string, attachments: string[]) => void;
  onStop: () => void;
}) {
  const app = useApp();
  const [text, setText] = useState("");
  const [attachments, setAttachments] = useState<string[]>([]);
  const ref = useRef<HTMLTextAreaElement>(null);
  const rec = useRecorder();

  // Content handed over by another page ("Resumir no chat", "Perguntar no chat"…).
  useEffect(() => {
    if (!draft) return;
    if (draft.text) setText(draft.text);
    if (draft.attachments) setAttachments(draft.attachments);
  }, [draft]);

  const dictate = async () => {
    const t = await rec.stop(false);
    if (t?.text) {
      setText((cur) => (cur.trim() ? `${cur.trimEnd()} ${t.text}` : t.text));
      ref.current?.focus();
    } else if (t) app.toast("Nenhuma fala detectada.");
  };

  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${Math.min(el.scrollHeight, 220)}px`;
  }, [text]);

  useEffect(() => ref.current?.focus(), [app.conversationId]);

  const submit = () => {
    if (busy || disabled || (!text.trim() && attachments.length === 0)) return;
    onSend(text, attachments);
    setText("");
    setAttachments([]);
  };

  const onPaste = async (e: React.ClipboardEvent) => {
    if (!vision) return;
    for (const item of Array.from(e.clipboardData.items)) {
      if (!item.type.startsWith("image/")) continue;
      e.preventDefault();
      const file = item.getAsFile();
      if (!file) continue;
      const data = await new Promise<string>((resolve) => {
        const r = new FileReader();
        r.onload = () => resolve(String(r.result).split(",")[1] ?? "");
        r.readAsDataURL(file);
      });
      const name = await api.saveAttachment(data, item.type.split("/")[1] || "png");
      setAttachments((a) => [...a, name]);
    }
  };

  return (
    <div className="mx-auto w-full max-w-[760px] px-6 pb-5">
      <div className="rounded-[20px] border border-line bg-card shadow-mac focus-within:border-accent/50">
        {attachments.length > 0 && (
          <div className="flex flex-wrap gap-2 px-3 pt-3">
            {attachments.map((a) => (
              <div key={a} className="relative">
                <img src={attachmentURL(a)} className="size-14 rounded-lg border border-line object-cover" />
                <button
                  onClick={() => setAttachments((list) => list.filter((x) => x !== a))}
                  className="absolute -top-1.5 -right-1.5 flex size-5 items-center justify-center rounded-full bg-fg text-bg"
                >
                  <X className="size-3" />
                </button>
              </div>
            ))}
          </div>
        )}
        <textarea
          ref={ref}
          rows={1}
          value={text}
          disabled={disabled}
          onChange={(e) => setText(e.target.value)}
          onPaste={onPaste}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
              e.preventDefault();
              submit();
            }
          }}
          placeholder={disabled ? "Baixe um modelo para começar a conversar" : "Pergunte qualquer coisa"}
          className="block w-full resize-none bg-transparent px-4 pt-3 pb-1 text-[14px] leading-relaxed outline-none placeholder:text-faint"
        />
        <div className="flex items-center justify-between px-2.5 pb-2.5">
          <div className="flex items-center gap-0.5">
            <CollectionPicker value={collectionId} onChange={onCollection} />
            {vision && (
              <button
                title="Anexar imagem"
                onClick={async () => {
                  try {
                    const names = await api.pickImages();
                    setAttachments((a) => [...a, ...names]);
                  } catch (e) {
                    app.toast(errorText(e), "error");
                  }
                }}
                className="flex size-8 items-center justify-center rounded-full text-muted hover:bg-hover hover:text-fg"
              >
                <ImageIcon className="size-[18px]" />
              </button>
            )}
          </div>
          {rec.state === "recording" ? (
            <div className="flex items-center gap-2">
              <span className="text-bad">
                <LevelBars level={rec.level} bars={18} className="h-6" />
              </span>
              <span className="w-9 text-[12px] text-muted tabular-nums">{clock(rec.elapsed)}</span>
              <button title="Descartar" onClick={rec.cancel} className="flex size-8 items-center justify-center rounded-full text-muted hover:bg-hover">
                <X className="size-4" />
              </button>
              <button title="Concluir ditado" onClick={dictate} className="flex size-8 items-center justify-center rounded-full bg-bad text-white">
                <Check className="size-4" strokeWidth={2.5} />
              </button>
            </div>
          ) : rec.state === "transcribing" ? (
            <span className="flex items-center gap-1.5 pr-1 text-[12px] text-muted">
              <Loader2 className="size-3.5 animate-spin" /> Transcrevendo… {rec.progress > 0 && `${rec.progress}%`}
            </span>
          ) : busy ? (
            <button
              title="Parar"
              onClick={onStop}
              className="flex size-8 items-center justify-center rounded-full bg-fg text-bg hover:opacity-85"
            >
              <Square className="size-3 fill-current" />
            </button>
          ) : (
            <div className="flex items-center gap-1">
              <button
                title={rec.ready ? "Ditar com o microfone" : "Para ditar, instale o whisperfile e um modelo de voz (página Transcrição)"}
                onClick={() => (rec.ready ? rec.start() : app.setPage("voice"))}
                disabled={disabled}
                className="flex size-8 items-center justify-center rounded-full text-muted hover:bg-hover hover:text-fg disabled:opacity-30"
              >
                <Mic className="size-[18px]" />
              </button>
              <button
                title="Enviar (Enter)"
                onClick={submit}
                disabled={disabled || (!text.trim() && attachments.length === 0)}
                className="flex size-8 items-center justify-center rounded-full bg-accent text-on-accent transition-opacity disabled:opacity-30"
              >
                <ArrowUp className="size-[18px]" strokeWidth={2.5} />
              </button>
            </div>
          )}
        </div>
      </div>
      <p className="mt-2 text-center text-[11px] text-faint">
        Roda 100% no seu computador. Modelos podem cometer erros — confira informações importantes.
      </p>
    </div>
  );
}

export function ChatPage() {
  const app = useApp();
  const [messages, setMessages] = useState<Message[]>([]);
  const [title, setTitle] = useState("");
  const [modelId, setModelId] = useState("");
  const [status, setStatus] = useState(""); // loading | searching | generating | ""
  const [streamingId, setStreamingId] = useState<number | null>(null);
  const [collectionId, setCollectionId] = useState("");
  const [draft, setDraft] = useState<ChatDraft | null>(null);
  const scroller = useRef<HTMLDivElement>(null);
  const stick = useRef(true);
  const convId = app.conversationId;
  const convRef = useRef(convId);
  convRef.current = convId;

  // Follow the last used model ("Conversar" in Discover/Library sets it).
  const lastModelId = app.settings?.lastModelId ?? "";
  useEffect(() => {
    if (lastModelId && app.chatModels.some((m) => m.id === lastModelId)) setModelId(lastModelId);
  }, [lastModelId, app.chatModels]);

  // Fall back to any installed model.
  useEffect(() => {
    if (modelId && app.chatModels.some((m) => m.id === modelId)) return;
    setModelId(app.chatModels[0]?.id ?? "");
  }, [app.chatModels, modelId]);

  // Load the selected conversation.
  useEffect(() => {
    stick.current = true;
    if (!convId) {
      setMessages([]);
      setTitle("");
      setStatus("");
      setStreamingId(null);
      const d = app.takeChatDraft();
      setDraft(d);
      setCollectionId(d?.collectionId ?? "");
      return;
    }
    api
      .conversation(convId)
      .then((d) => {
        setMessages(d.messages);
        setTitle(d.conversation.title);
        setCollectionId(d.conversation.collectionId ?? "");
        if (d.conversation.modelId && app.chatModels.some((m) => m.id === d.conversation.modelId)) {
          setModelId(d.conversation.modelId);
        }
        if (d.generating) {
          setStatus("generating");
          const last = d.messages[d.messages.length - 1];
          setStreamingId(last?.role === "assistant" ? last.id : null);
        } else {
          setStatus("");
          setStreamingId(null);
        }
      })
      .catch(() => app.newChat());
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [convId]);

  // Keep the title fresh (auto-title arrives after the first answer).
  useEffect(() => {
    const c = app.conversations.find((c) => c.id === convId);
    if (c) setTitle(c.title);
  }, [app.conversations, convId]);

  // Streaming events.
  useEffect(() => {
    const offs = [
      on<Message>("chat:message", (m) => {
        if (m.conversationId !== convRef.current) return;
        setMessages((list) => {
          const i = list.findIndex((x) => x.id === m.id);
          if (i >= 0) return list.map((x) => (x.id === m.id ? m : x));
          // An edit/regenerate removed later messages on the backend.
          return [...list.filter((x) => x.id < m.id), m];
        });
        if (m.role === "assistant") setStreamingId(m.id);
      }),
      on<{ conversationId: string; messageId: number; content: string; reasoning: string }>("chat:delta", (d) => {
        if (d.conversationId !== convRef.current) return;
        setMessages((list) =>
          list.map((x) =>
            x.id === d.messageId ? { ...x, content: d.content, reasoning: d.reasoning } : x,
          ),
        );
      }),
      on<Message>("chat:done", (m) => {
        if (m.conversationId !== convRef.current) return;
        setMessages((list) => list.map((x) => (x.id === m.id ? m : x)));
        setStreamingId(null);
      }),
      on<{ conversationId: string; status: string }>("chat:status", (s) => {
        if (s.conversationId === convRef.current) setStatus(s.status);
      }),
    ];
    return () => offs.forEach((off) => off());
  }, []);

  // Auto-scroll while the user is at the bottom.
  useEffect(() => {
    const el = scroller.current;
    if (el && stick.current) el.scrollTop = el.scrollHeight;
  }, [messages]);

  const busy = status !== "" || streamingId !== null;
  const model = app.chatModels.find((m) => m.id === modelId);

  const send = useCallback(
    async (text: string, attachments: string[]) => {
      if (!modelId) return;
      stick.current = true;
      try {
        if (!convId) setStatus("loading");
        const conv = await api.send(convId, modelId, collectionId, text, attachments);
        if (!convId) {
          convRef.current = conv.id;
          app.openConversation(conv.id);
        }
      } catch (e) {
        setStatus("");
        app.toast(errorText(e), "error");
      }
    },
    [convId, modelId, collectionId, app],
  );

  const changeModel = (id: string) => {
    setModelId(id);
    app.saveSettings({ lastModelId: id });
    if (convId) api.setConversationModel(convId, id);
  };

  const changeCollection = (id: string) => {
    setCollectionId(id);
    if (convId) api.setConversationCollection(convId, id);
  };
  const collection = app.collections.find((c) => c.id === collectionId);

  const lastAssistant = [...messages].reverse().find((m) => m.role === "assistant");
  const empty = messages.length === 0;
  const noModels = app.chatModels.length === 0;

  return (
    <div className="flex h-full min-w-0 flex-1 flex-col bg-bg">
      <header className="drag flex h-[52px] shrink-0 items-center gap-2 border-b border-line px-4">
        {app.chatModels.length > 0 && <ModelPicker models={app.chatModels} value={modelId} onChange={changeModel} />}
        {title && <span className="truncate text-[13px] text-muted">· {title}</span>}
        <div className="flex-1" />
        {status === "loading" && (
          <span className="flex items-center gap-1.5 rounded-full bg-warn-soft px-2.5 py-1 text-[11.5px] font-medium text-warn">
            <Loader2 className="size-3 animate-spin" /> Carregando modelo…
          </span>
        )}
        {status === "searching" && (
          <span className="flex items-center gap-1.5 rounded-full bg-accent-soft px-2.5 py-1 text-[11.5px] font-medium text-accent">
            <Loader2 className="size-3 animate-spin" /> Buscando nos documentos…
          </span>
        )}
        {collection && status !== "searching" && (
          <span className="flex items-center gap-1 text-[11.5px] text-muted" title="As respostas usam estes documentos">
            <BookOpen className="size-3.5" /> {collection.name}
          </span>
        )}
      </header>

      <div
        ref={scroller}
        onScroll={(e) => {
          const el = e.currentTarget;
          stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 80;
        }}
        className="min-h-0 flex-1 overflow-y-auto"
      >
        {empty ? (
          noModels ? (
            <EmptyState
              icon={<Sparkles className="size-10" />}
              title="Nenhum modelo instalado"
              text="Veja quais modelos rodam melhor no seu computador e baixe um para começar a conversar."
              action={
                <Button variant="primary" onClick={() => app.setPage("discover")}>
                  Descobrir modelos
                </Button>
              }
            />
          ) : (
            <div className="mx-auto flex h-full max-w-[680px] flex-col items-center justify-center px-6 pb-10">
              <div className="mb-1 text-[26px] font-semibold tracking-tight">Como posso ajudar?</div>
              <div className="mb-7 text-[13px] text-muted">
                {model ? `Conversando com ${model.name}` : "Escolha um modelo"} — tudo fica no seu computador.
              </div>
              <div className="grid w-full grid-cols-2 gap-2">
                {suggestions.map((s) => (
                  <button
                    key={s}
                    onClick={() => send(s, [])}
                    className="rounded-xl border border-line bg-card px-3.5 py-2.5 text-left text-[13px] text-fg/85 transition-colors hover:bg-hover"
                  >
                    {s}
                  </button>
                ))}
              </div>
            </div>
          )
        ) : (
          <div className="mx-auto max-w-[760px] space-y-6 px-6 py-6">
            {messages.map((m) => (
              <MessageRow
                key={m.id}
                m={m}
                streaming={m.id === streamingId}
                isLast={m.id === lastAssistant?.id}
                busy={busy}
                onRegenerate={() => api.regenerate(convId).catch((e) => app.toast(errorText(e), "error"))}
                onEdit={(text) => api.edit(convId, m.id, text).catch((e) => app.toast(errorText(e), "error"))}
              />
            ))}
          </div>
        )}
      </div>

      <Composer
        disabled={noModels || !modelId}
        busy={busy}
        vision={!!model?.vision}
        collectionId={collectionId}
        onCollection={changeCollection}
        draft={draft}
        onSend={send}
        onStop={() => convId && api.stop(convId)}
      />
    </div>
  );
}
