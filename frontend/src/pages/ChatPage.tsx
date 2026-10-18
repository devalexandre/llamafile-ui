import clsx from "clsx";
import {
  ArrowUp,
  BookOpen,
  Bot,
  Brain,
  Check,
  ChevronDown,
  Copy,
  FileText,
  Image as ImageIcon,
  Film,
  ImagePlus,
  Loader2,
  Mic,
  Paperclip,
  Pencil,
  Plus,
  RotateCcw,
  Settings2,
  Sparkles,
  Square,
  Volume2,
  X,
} from "lucide-react";
import { memo, useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import rehypeHighlight from "rehype-highlight";
import remarkGfm from "remark-gfm";
import { Button, EmptyState, Menu } from "../components/ui";
import { api, attachmentURL, on, onFileDrop } from "../lib/api";
import { msg, t } from "../i18n";
import { errorText, useApp } from "../lib/store";
import type { ChatDraft, ImageProgress, Message, ModelView, Source } from "../lib/types";
import { clock, useRecorder } from "../lib/useRecorder";
import { phaseLabel } from "./ImagesPage";
import { LevelBars } from "./VoicePage";

const audioExts = [".wav", ".mp3", ".ogg", ".flac"];
const isAudio = (p: string) => audioExts.some((e) => p.toLowerCase().endsWith(e));
const isDoc = (name: string) => name.endsWith(".doc.txt");
/** Original name of a document attachment ("<uuid>__report.pdf.doc.txt"). */
const docName = (name: string) => name.replace(/\.doc\.txt$/, "").replace(/^[^_]+__/, "");

/** Same rule as the backend: "crie uma imagem de…", "desenhe um logo…". */
const imageIntent =
  /^\s*(por favor,?\s*)?(crie|cria|criar|gere|gera|gerar|desenhe|desenha|fa[çc]a|faz|pinte|ilustre|create|generate|draw|make|paint)\b.{0,40}?\b(imagem|imagens|foto|fotografia|ilustra[çc][ãa]o|desenho|pintura|arte|logo|image|picture|photo|illustration|drawing|painting)\b/i;

/** With an image attached: "transforme esta foto em…", "deixe o céu roxo". */
/** "crie um vídeo de…", "anime esta imagem". */
const videoIntent = /\b(v[ií]deo|anima[çc][ãa]o|clipe|video|animation|clip)\b|^\s*(anime|animate)\b/i;

const editIntent =
  /^\s*(por favor,?\s*)?(transforme|transforma|edite|edita|altere|altera|modifique|modifica|mude|muda|converta|converte|deixe|deixa|coloque|coloca|adicione|adiciona|remova|remove|troque|troca|transform|edit|change|turn|make|add|remove|replace)\b/i;

function Attachments({ names, align, onRemove }: { names: string[]; align?: "end"; onRemove?: (n: string) => void }) {
  return (
    <div className={clsx("flex flex-wrap gap-1.5", align === "end" && "justify-end")}>
      {names.map((a) => (
        <div key={a} className="relative">
          {isDoc(a) ? (
            <div className="flex h-14 max-w-[220px] items-center gap-2 rounded-lg border border-line bg-field px-3 text-[12px]">
              <FileText className="size-4 shrink-0 text-accent" />
              <span className="truncate">{docName(a)}</span>
            </div>
          ) : onRemove ? (
            <img src={attachmentURL(a)} className="size-14 rounded-lg border border-line object-cover" />
          ) : (
            <img src={attachmentURL(a)} className="max-h-40 rounded-xl border border-line object-cover" />
          )}
          {onRemove && (
            <button
              onClick={() => onRemove(a)}
              className="absolute -top-1.5 -right-1.5 flex size-5 items-center justify-center rounded-full bg-fg text-bg"
            >
              <X className="size-3" />
            </button>
          )}
        </div>
      ))}
    </div>
  );
}

const suggestions = [
  msg("Explique como funciona a quantização de modelos"),
  msg("Escreva um e-mail curto pedindo uma reunião"),
  msg("Crie uma função em Go que inverta uma string"),
  msg("Resuma os prós e contras de rodar IA localmente"),
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
          {current ? current.name : t("Escolher modelo")}
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

function AgentPicker({ value, onChange }: { value: string; onChange: (id: string) => void }) {
  const app = useApp();
  const current = app.agents.find((a) => a.id === value);
  return (
    <Menu
      align="left"
      trigger={(toggle) => (
        <button
          onClick={toggle}
          title={t("Agente que responde nesta conversa")}
          className={clsx(
            "no-drag flex h-[26px] items-center gap-1.5 rounded-full px-2.5 text-[12.5px]",
            current ? "bg-accent-soft font-medium text-accent" : "text-muted hover:bg-hover hover:text-fg",
          )}
        >
          {current ? <span>{current.icon}</span> : <Bot className="size-3.5" />}
          {current ? current.name : t("Agente")}
          <ChevronDown className="size-3" />
        </button>
      )}
      items={[
        { label: t("Assistente padrão"), icon: !value ? <Check className="size-3.5" /> : undefined, onClick: () => onChange("") },
        ...app.agents.map((a) => ({
          label: `${a.icon} ${a.name}`,
          icon: a.id === value ? <Check className="size-3.5" /> : undefined,
          onClick: () => onChange(a.id),
        })),
        "separator" as const,
        { label: t("Gerenciar agentes…"), icon: <Settings2 className="size-3.5" />, onClick: () => app.setPage("agents") },
      ]}
    />
  );
}

function CopyButton({ text }: { text: string }) {
  const [done, setDone] = useState(false);
  return (
    <button
      title={t("Copiar")}
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
        {streaming ? t("Pensando…") : t("Raciocínio")}
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
  waitingLabel,
  speaking,
}: {
  /** This message is being read aloud. */
  speaking?: boolean;
  m: Message;
  waitingLabel?: string;
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
          <div className="mb-1.5">
            <Attachments names={m.attachments} align="end" />
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
                {t("Cancelar")}
              </Button>
              <Button
                size="sm"
                variant="primary"
                onClick={() => {
                  setEditing(false);
                  onEdit(draft);
                }}
              >
                {t("Enviar")}
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
                title={t("Editar")}
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
          <Loader2 className="size-3.5 animate-spin" /> {waitingLabel ?? t("Gerando…")}
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
      {m.attachments.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-2">
          {m.attachments.map((a) =>
            a.endsWith(".webm") ? (
              <video key={a} src={attachmentURL(a)} controls autoPlay muted loop playsInline className="max-h-[360px] rounded-xl border border-line" />
            ) : (
              <a key={a} href={attachmentURL(a)} target="_blank" rel="noreferrer">
                <img src={attachmentURL(a)} className="max-h-[360px] rounded-xl border border-line object-cover" />
              </a>
            ),
          )}
        </div>
      )}
      {m.error && (
        <div className="mt-2 rounded-lg bg-bad-soft px-3 py-2 text-[12.5px] text-bad">{m.error}</div>
      )}
      {m.sources?.length > 0 && <Sources sources={m.sources} />}
      {!streaming && (
        <div className={clsx("mt-1 flex items-center gap-0.5 transition-opacity group-hover:opacity-100", speaking ? "opacity-100" : "opacity-0")}>
          <CopyButton text={m.content} />
          {m.content && (
            <SpeakButton id={`msg:${m.id}`} text={m.content} speaking={!!speaking} />
          )}
          {isLast && !busy && m.attachments.length === 0 && (
            <button
              title={t("Gerar novamente")}
              onClick={onRegenerate}
              className="flex size-6 items-center justify-center rounded-md text-muted hover:bg-hover hover:text-fg"
            >
              <RotateCcw className="size-3.5" />
            </button>
          )}
          {m.tps > 0 && (
            <span className="ml-1.5 text-[11px] text-faint">
              {t("{tps} tok/s · {tokens} tokens", { tps: m.tps.toFixed(1), tokens: m.tokens })}
            </span>
          )}
        </div>
      )}
    </div>
  );
}

/** Reads every new answer aloud (same as Settings → Leitura em voz alta). */
function AutoSpeakToggle() {
  const app = useApp();
  const on = !!app.settings?.autoSpeak;
  const ready = !!app.tool("piper")?.installed && app.ttsModels.length > 0;
  return (
    <label
      title={ready ? t("Ler cada resposta em voz alta assim que terminar") : t("Instale o Piper e uma voz para ouvir as respostas")}
      className={clsx(
        "flex h-8 cursor-pointer items-center gap-1.5 rounded-full px-2.5 text-[12.5px] select-none",
        on ? "bg-accent-soft font-medium text-accent" : "text-muted hover:bg-hover hover:text-fg",
      )}
    >
      <input
        type="checkbox"
        checked={on}
        onChange={(e) => {
          const v = e.target.checked;
          if (v && !ready) {
            app.toast(t("Para ouvir as respostas, instale o Piper e uma voz em Configurações → Leitura em voz alta."));
            app.setPage("settings");
            return;
          }
          app.saveSettings({ autoSpeak: v });
          if (!v) api.stopSpeaking();
        }}
        className="size-3.5 accent-[var(--accent)]"
      />
      <Volume2 className="size-[16px]" />
      {t("Ler respostas")}
    </label>
  );
}

function SpeakButton({ id, text, speaking }: { id: string; text: string; speaking: boolean }) {
  const app = useApp();
  return (
    <button
      title={speaking ? t("Parar leitura") : t("Ouvir")}
      onClick={() => (speaking ? api.stopSpeaking() : api.speak(id, text).catch((e) => app.toast(errorText(e), "error")))}
      className={clsx(
        "flex size-6 items-center justify-center rounded-md hover:bg-hover",
        speaking ? "text-accent" : "text-muted hover:text-fg",
      )}
    >
      {speaking ? <Square className="size-3 fill-current" /> : <Volume2 className="size-3.5" />}
    </button>
  );
}

/** Document passages the answer was grounded on (RAG). */
function Sources({ sources }: { sources: Source[] }) {
  const [open, setOpen] = useState<number | null>(null);
  const current = sources.find((s) => s.n === open);
  return (
    <div className="mt-2.5">
      <div className="flex flex-wrap items-center gap-1.5">
        <span className="text-[11.5px] font-medium text-muted">{t("Fontes")}</span>
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
            {t("{document} · trecho {n} · {score}% similar", { document: current.document, n: current.chunk + 1, score: Math.round(current.score * 100) })}
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
      side="top"
      trigger={(toggle) => (
        <button
          onClick={toggle}
          title={t("Conversar com documentos")}
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
        { label: t("Sem documentos"), icon: !value ? <Check className="size-3.5" /> : undefined, onClick: () => onChange("") },
        ...app.collections.map((c) => ({
          label: `${c.name} (${c.documents})`,
          icon: c.id === value ? <Check className="size-3.5" /> : <BookOpen className="size-3.5" />,
          onClick: () => onChange(c.id),
        })),
        "separator" as const,
        { label: t("Gerenciar documentos…"), icon: <Settings2 className="size-3.5" />, onClick: () => app.setPage("documents") },
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
  onImage,
  videoReady,
  imageReady,
  onStop,
}: {
  onImage: (text: string, reference: string, video: boolean) => void;
  /** A video model is installed (and an image engine). */
  videoReady: boolean;
  /** An image model and diffusionfile are installed. */
  imageReady: boolean;
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
  const [mediaMode, setMediaMode] = useState<"" | "image" | "video">("");
  const imageMode = mediaMode !== "";
  const [adding, setAdding] = useState(false);
  const ref = useRef<HTMLTextAreaElement>(null);
  const rec = useRecorder();

  // Audio is transcribed into the message; images and documents are attached.
  const addFiles = async (paths: string[]) => {
    if (!paths?.length) return;
    setAdding(true);
    try {
      for (const p of paths.filter(isAudio)) {
        const tr = await api.transcribeForChat(p);
        if (tr) setText((cur) => (cur.trim() ? `${cur.trimEnd()} ${tr}` : tr));
      }
      const rest = paths.filter((p) => !isAudio(p));
      if (rest.length) {
        const names = await api.attachFiles(rest);
        if (!vision && names.some((n) => !isDoc(n))) app.toast(t("O modelo atual não entende imagens — escolha um modelo de Visão para usá-las."));
        setAttachments((a) => [...a, ...names]);
      }
    } catch (e) {
      app.toast(errorText(e), "error");
    } finally {
      setAdding(false);
      ref.current?.focus();
    }
  };
  useEffect(() => onFileDrop((paths) => !disabled && addFiles(paths))); // eslint-disable-line react-hooks/exhaustive-deps

  // Content handed over by another page ("Resumir no chat", "Perguntar no chat"…).
  useEffect(() => {
    if (!draft) return;
    if (draft.text) setText(draft.text);
    if (draft.attachments) setAttachments(draft.attachments);
  }, [draft]);

  const dictate = async () => {
    const tr = await rec.stop(false);
    if (tr?.text) {
      setText((cur) => (cur.trim() ? `${cur.trimEnd()} ${tr.text}` : tr.text));
      ref.current?.focus();
    } else if (tr) app.toast(t("Nenhuma fala detectada."));
  };

  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${Math.min(el.scrollHeight, 220)}px`;
  }, [text]);

  useEffect(() => ref.current?.focus(), [app.conversationId]);

  const submit = () => {
    if (busy || disabled || adding || (!text.trim() && attachments.length === 0)) return;
    const images = attachments.filter((a) => !isDoc(a));
    const reference = images[0] ?? "";
    const create = imageIntent.test(text) || (images.length === 1 && editIntent.test(text));
    const wantsVideo = mediaMode === "video" || (mediaMode === "" && videoReady && videoIntent.test(text) && (create || /^\s*(anime|animate)\b/i.test(text)));
    const wantsImage = mediaMode === "image" || (mediaMode === "" && create);
    const ready = wantsVideo ? videoReady : imageReady;
    if (ready && text.trim() && images.length <= 1 && attachments.length === images.length && (wantsVideo || wantsImage)) {
      onImage(text, reference, wantsVideo);
      setMediaMode("");
    } else {
      onSend(text, attachments);
    }
    setText("");
    setAttachments([]);
  };

  const onPaste = async (e: React.ClipboardEvent) => {
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
      <div
        data-file-drop-target
        className="rounded-[20px] border border-line bg-card shadow-mac focus-within:border-accent/50 [&.file-drop-target-active]:border-accent [&.file-drop-target-active]:bg-accent-soft"
      >
        {attachments.length > 0 && (
          <div className="px-3 pt-3">
            <Attachments names={attachments} onRemove={(a) => setAttachments((list) => list.filter((x) => x !== a))} />
          </div>
        )}
        {adding && (
          <div className="flex items-center gap-1.5 px-4 pt-2.5 text-[12px] text-muted">
            <Loader2 className="size-3.5 animate-spin" /> {t("Preparando arquivos…")}
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
          placeholder={
            disabled
              ? t("Baixe um modelo para começar a conversar")
              : mediaMode === "video"
                ? attachments.some((a) => !isDoc(a))
                  ? t("Diga como animar a imagem anexada")
                  : t("Descreva o vídeo que você quer (ou anexe uma imagem para animar)")
                : imageMode
                ? attachments.some((a) => !isDoc(a))
                  ? t("Diga como alterar a imagem anexada")
                  : t("Descreva a imagem que você quer (ou anexe uma de referência)")
                : t("Pergunte qualquer coisa")
          }
          className="block w-full resize-none bg-transparent px-4 pt-3 pb-1 text-[14px] leading-relaxed outline-none placeholder:text-faint"
        />
        <div className="flex items-center justify-between px-2.5 pb-2.5">
          <div className="flex items-center gap-0.5">
            <CollectionPicker value={collectionId} onChange={onCollection} />
            <Menu
              align="left"
              side="top"
              trigger={(toggle) => (
                <button
                  title={t("Anexar arquivos ou gerar imagem")}
                  onClick={toggle}
                  disabled={disabled}
                  className="flex size-8 items-center justify-center rounded-full text-muted hover:bg-hover hover:text-fg disabled:opacity-30"
                >
                  <Plus className="size-[19px]" />
                </button>
              )}
              items={[
                {
                  label: t("Arquivos, imagens ou áudio…"),
                  icon: <Paperclip className="size-3.5" />,
                  onClick: async () => {
                    try {
                      addFiles(await api.pickChatFiles());
                    } catch (e) {
                      app.toast(errorText(e), "error");
                    }
                  },
                },
                {
                  label: imageReady ? t("Gerar imagem") : t("Gerar imagem (baixe um modelo de imagem)"),
                  icon: <ImagePlus className="size-3.5" />,
                  onClick: () => (imageReady ? (setMediaMode("image"), ref.current?.focus()) : app.setPage("images")),
                },
                {
                  label: videoReady ? t("Gerar vídeo") : t("Gerar vídeo (baixe um modelo de vídeo)"),
                  icon: <Film className="size-3.5" />,
                  onClick: () => (videoReady ? (setMediaMode("video"), ref.current?.focus()) : app.setPage("images")),
                },
              ]}
            />
            {imageMode && (
              <button
                onClick={() => setMediaMode("")}
                title={t("Voltar ao chat")}
                className="flex h-8 items-center gap-1.5 rounded-full bg-accent-soft px-2.5 text-[12.5px] font-medium text-accent"
              >
                {mediaMode === "video" ? <Film className="size-[16px]" /> : <ImagePlus className="size-[16px]" />}
                {mediaMode === "video" ? t("Vídeo") : t("Imagem")} <X className="size-3" />
              </button>
            )}
            <AutoSpeakToggle />
          </div>
          {rec.state === "recording" ? (
            <div className="flex items-center gap-2">
              <span className="text-bad">
                <LevelBars level={rec.level} bars={18} className="h-6" />
              </span>
              <span className="w-9 text-[12px] text-muted tabular-nums">{clock(rec.elapsed)}</span>
              <button title={t("Descartar")} onClick={rec.cancel} className="flex size-8 items-center justify-center rounded-full text-muted hover:bg-hover">
                <X className="size-4" />
              </button>
              <button title={t("Concluir ditado")} onClick={dictate} className="flex size-8 items-center justify-center rounded-full bg-bad text-white">
                <Check className="size-4" strokeWidth={2.5} />
              </button>
            </div>
          ) : rec.state === "transcribing" ? (
            <span className="flex items-center gap-1.5 pr-1 text-[12px] text-muted">
              <Loader2 className="size-3.5 animate-spin" /> {t("Transcrevendo…")} {rec.progress > 0 && `${rec.progress}%`}
            </span>
          ) : busy ? (
            <button
              title={t("Parar")}
              onClick={onStop}
              className="flex size-8 items-center justify-center rounded-full bg-fg text-bg hover:opacity-85"
            >
              <Square className="size-3 fill-current" />
            </button>
          ) : (
            <div className="flex items-center gap-1">
              <button
                title={rec.ready ? t("Ditar com o microfone") : t("Para ditar, instale o whisperfile e um modelo de voz (página Transcrição)")}
                onClick={() => (rec.ready ? rec.start() : app.setPage("voice"))}
                disabled={disabled}
                className="flex size-8 items-center justify-center rounded-full text-muted hover:bg-hover hover:text-fg disabled:opacity-30"
              >
                <Mic className="size-[18px]" />
              </button>
              <button
                title={t("Enviar (Enter)")}
                onClick={submit}
                disabled={disabled || adding || (!text.trim() && attachments.length === 0)}
                className="flex size-8 items-center justify-center rounded-full bg-accent text-on-accent transition-opacity disabled:opacity-30"
              >
                <ArrowUp className="size-[18px]" strokeWidth={2.5} />
              </button>
            </div>
          )}
        </div>
      </div>
      <p className="mt-2 text-center text-[11px] text-faint">
        {t("Roda 100% no seu computador. Modelos podem cometer erros — confira informações importantes.")}
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
  // Agent answering and project of the conversation (or of the new chat).
  const [agentId, setAgentId] = useState("");
  const [projectId, setProjectId] = useState("");
  const [draft, setDraft] = useState<ChatDraft | null>(null);
  const [imageStep, setImageStep] = useState("");
  const [speakingId, setSpeakingId] = useState("");
  const autoSpeak = useRef(false);
  autoSpeak.current = !!app.settings?.autoSpeak;
  useEffect(() => {
    api.speakingId().then((id) => setSpeakingId(id ?? ""));
  }, []);
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
      const proj = app.projects.find((p) => p.id === d?.projectId);
      const ag = app.agents.find((a) => a.id === (d?.agentId || proj?.agentId));
      setProjectId(d?.projectId ?? "");
      setAgentId(ag?.id ?? "");
      setCollectionId(d?.collectionId || proj?.collectionId || ag?.collectionId || "");
      return;
    }
    api
      .conversation(convId)
      .then((d) => {
        setMessages(d.messages);
        setTitle(d.conversation.title);
        setCollectionId(d.conversation.collectionId ?? "");
        setAgentId(d.conversation.agentId ?? "");
        setProjectId(d.conversation.projectId ?? "");
        if (d.conversation.modelId && app.chatModels.some((m) => m.id === d.conversation.modelId)) {
          setModelId(d.conversation.modelId);
        }
        if (d.generatingImage) {
          setStatus("image");
          const last = d.messages[d.messages.length - 1];
          setStreamingId(last?.role === "assistant" ? last.id : null);
        } else if (d.generating) {
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
        if (autoSpeak.current && m.role === "assistant" && m.content && !m.error && m.attachments.length === 0) {
          api.speak(`msg:${m.id}`, m.content).catch(() => {});
        }
      }),
      on<{ id: string; playing: boolean; error: string }>("tts:state", (s) => {
        setSpeakingId((cur) => (s.playing ? s.id : cur === s.id ? "" : cur));
        if (s.error) app.toast(s.error, "error");
      }),
      on<{ conversationId: string; status: string }>("chat:status", (s) => {
        if (s.conversationId === convRef.current) setStatus(s.status);
        if (s.status !== "image") setImageStep("");
      }),
      on<ImageProgress>("image:progress", (p) =>
        setImageStep(
          `${phaseLabel[p.phase] ? t(phaseLabel[p.phase]) : t("Gerando a imagem…")}${p.phase === "sampling" && p.steps > 0 ? ` ${p.step}/${p.steps}` : ""}`,
        ),
      ),
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
        const conv = await api.sendWith({ conversationId: convId, modelId, collectionId, agentId, projectId, text, attachments });
        if (!convId) {
          convRef.current = conv.id;
          app.openConversation(conv.id);
        }
      } catch (e) {
        setStatus("");
        app.toast(errorText(e), "error");
      }
    },
    [convId, modelId, collectionId, agentId, projectId, app],
  );

  const generateImage = useCallback(
    async (text: string, reference: string, video: boolean) => {
      stick.current = true;
      try {
        const conv = await (video ? api.generateChatVideo : api.generateChatImage)(convId, modelId, text, reference);
        if (!convId) {
          convRef.current = conv.id;
          app.openConversation(conv.id);
        }
      } catch (e) {
        app.toast(errorText(e), "error");
      }
    },
    [convId, modelId, app],
  );
  const engineReady = app.tools.some((t) => (t.tool === "diffusionfile" || t.tool === "sd-cpp") && t.installed);
  const imageReady = engineReady && app.imageModels.length > 0;
  const videoReady = engineReady && app.videoModels.length > 0;

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
  const agent = app.agents.find((a) => a.id === agentId);
  const project = app.projects.find((p) => p.id === projectId);

  const changeAgent = (id: string) => {
    setAgentId(id);
    if (convId) api.setConversationAgent(convId, id).catch((e) => app.toast(errorText(e), "error"));
    const a = app.agents.find((x) => x.id === id);
    if (a?.modelId && a.modelId !== modelId && app.chatModels.some((m) => m.id === a.modelId)) changeModel(a.modelId);
    if (a?.collectionId && !collectionId) changeCollection(a.collectionId);
  };

  const lastAssistant = [...messages].reverse().find((m) => m.role === "assistant");
  const empty = messages.length === 0;
  const noModels = app.chatModels.length === 0;

  return (
    <div className="flex h-full min-w-0 flex-1 flex-col bg-bg">
      <header className="drag flex h-[52px] shrink-0 items-center gap-2 border-b border-line px-4">
        {project && (
          <button
            onClick={() => app.openProject(project.id)}
            className="no-drag flex h-[26px] shrink-0 items-center gap-1 rounded-[7px] px-2 text-[12.5px] text-muted hover:bg-hover hover:text-fg"
            title={t("Abrir o projeto")}
          >
            <span>{project.icon}</span>
            <span className="max-w-[140px] truncate">{project.name}</span>
            <span className="text-faint">/</span>
          </button>
        )}
        {app.chatModels.length > 0 && <ModelPicker models={app.chatModels} value={modelId} onChange={changeModel} />}
        <AgentPicker value={agentId} onChange={changeAgent} />
        {title && <span className="truncate text-[13px] text-muted">· {title}</span>}
        <div className="flex-1" />
        {status === "loading" && (
          <span className="flex items-center gap-1.5 rounded-full bg-warn-soft px-2.5 py-1 text-[11.5px] font-medium text-warn">
            <Loader2 className="size-3 animate-spin" /> {t("Carregando modelo…")}
          </span>
        )}
        {status === "searching" && (
          <span className="flex items-center gap-1.5 rounded-full bg-accent-soft px-2.5 py-1 text-[11.5px] font-medium text-accent">
            <Loader2 className="size-3 animate-spin" /> {t("Buscando nos documentos…")}
          </span>
        )}
        {collection && status !== "searching" && (
          <span className="flex items-center gap-1 text-[11.5px] text-muted" title={t("As respostas usam estes documentos")}>
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
              title={t("Nenhum modelo instalado")}
              text={t("Veja quais modelos rodam melhor no seu computador e baixe um para começar a conversar.")}
              action={
                <Button variant="primary" onClick={() => app.setPage("discover")}>
                  {t("Descobrir modelos")}
                </Button>
              }
            />
          ) : (
            <div className="mx-auto flex h-full max-w-[680px] flex-col items-center justify-center px-6 pb-10">
              {agent && <div className="mb-2 text-[40px] leading-none">{agent.icon}</div>}
              <div className="mb-1 text-[26px] font-semibold tracking-tight">{agent ? agent.name : t("Como posso ajudar?")}</div>
              <div className="mb-7 max-w-[520px] text-center text-[13px] text-muted">
                {agent?.description ? `${agent.description} · ` : ""}
                {model
                  ? project
                    ? t("Usando {model} no projeto {project} — tudo fica no seu computador.", { model: model.name, project: project.name })
                    : t("Usando {model} — tudo fica no seu computador.", { model: model.name })
                  : project
                    ? t("Escolha um modelo no projeto {project} — tudo fica no seu computador.", { project: project.name })
                    : t("Escolha um modelo — tudo fica no seu computador.")}
              </div>
              <div className="grid w-full grid-cols-2 gap-2">
                {suggestions.map((s) => (
                  <button
                    key={s}
                    onClick={() => send(t(s), [])}
                    className="rounded-xl border border-line bg-card px-3.5 py-2.5 text-left text-[13px] text-fg/85 transition-colors hover:bg-hover"
                  >
                    {t(s)}
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
                waitingLabel={status === "image" ? imageStep || t("Preparando a imagem…") : undefined}
                speaking={speakingId === `msg:${m.id}`}
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
        onImage={generateImage}
        videoReady={videoReady}
        imageReady={imageReady}
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
