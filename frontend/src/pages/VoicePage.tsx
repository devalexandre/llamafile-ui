import clsx from "clsx";
import { AudioLines, Check, ChevronDown, Copy, FileAudio, FileText, Loader2, MessageSquare, Mic, Square, Subtitles, Trash2, X } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { ModelGate, ModelSelect, ToolGate } from "../components/Gates";
import { Button, Card, IconButton, Progress, Select } from "../components/ui";
import { api, onFileDrop } from "../lib/api";
import { duration } from "../lib/format";
import { errorText, useApp } from "../lib/store";
import type { Transcript } from "../lib/types";
import { clock, useRecorder } from "../lib/useRecorder";
import { locale, msg, t } from "../i18n";

export const languages = [
  { value: "auto", label: msg("Detectar idioma") },
  { value: "pt", label: msg("Português") },
  { value: "en", label: msg("Inglês") },
  { value: "es", label: msg("Espanhol") },
  { value: "fr", label: msg("Francês") },
  { value: "de", label: msg("Alemão") },
  { value: "it", label: msg("Italiano") },
  { value: "ja", label: msg("Japonês") },
  { value: "zh", label: msg("Chinês") },
];

/** Animated bars driven by the microphone level. */
export function LevelBars({ level, bars = 24, className }: { level: number; bars?: number; className?: string }) {
  return (
    <div className={clsx("flex h-8 items-center gap-[3px]", className)}>
      {Array.from({ length: bars }).map((_, i) => {
        const wave = Math.sin((i / bars) * Math.PI);
        const h = Math.max(0.12, Math.min(1, level * (0.5 + wave) * (0.75 + Math.random() * 0.5)));
        return <span key={i} className="w-[3px] rounded-full bg-current transition-[height] duration-75" style={{ height: `${h * 100}%` }} />;
      })}
    </div>
  );
}

function TranscriptCard({ tr, onDelete }: { tr: Transcript; onDelete: () => void }) {
  const app = useApp();
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const exportAs = (format: "txt" | "srt") =>
    api
      .exportTranscript(tr.id, format)
      .then((p) => p && app.toast(t("Salvo em {path}", { path: p }), "success"))
      .catch((e) => app.toast(errorText(e), "error"));
  return (
    <Card className="p-4">
      <div className="flex items-start gap-3">
        <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-accent-soft text-accent">
          {tr.source === "recording" ? <Mic className="size-4" /> : <FileAudio className="size-4" />}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="truncate text-[13px] font-semibold">{tr.name}</span>
            <span className="text-[11px] text-muted">
              {new Date(tr.createdAt).toLocaleString(locale(), { dateStyle: "short", timeStyle: "short" })} ·{" "}
              {t("processado em {time}", { time: duration(tr.durationMs / 1000) })}
            </span>
          </div>
          <p className={clsx("selectable mt-1 text-[13px] leading-relaxed text-fg/90", !open && "line-clamp-3")}>{tr.text || t("(sem fala detectada)")}</p>
          {tr.text.length > 240 && (
            <button onClick={() => setOpen((o) => !o)} className="mt-1 flex items-center gap-0.5 text-[12px] text-accent">
              {open ? t("Mostrar menos") : t("Mostrar tudo")}
              <ChevronDown className={clsx("size-3 transition-transform", open && "rotate-180")} />
            </button>
          )}
          <div className="mt-2.5 flex flex-wrap gap-1.5">
            <Button
              size="sm"
              icon={copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
              onClick={() => {
                navigator.clipboard.writeText(tr.text);
                setCopied(true);
                setTimeout(() => setCopied(false), 1200);
              }}
            >
              {t("Copiar")}
            </Button>
            <Button size="sm" icon={<FileText className="size-3.5" />} onClick={() => exportAs("txt")}>
              .txt
            </Button>
            {tr.srt && (
              <Button size="sm" icon={<Subtitles className="size-3.5" />} onClick={() => exportAs("srt")}>
                {t("Legendas .srt")}
              </Button>
            )}
            <Button
              size="sm"
              icon={<MessageSquare className="size-3.5" />}
              disabled={!tr.text}
              onClick={() => app.newChat({ text: t("Resuma os pontos principais desta transcrição:\n\n{text}", { text: tr.text }) })}
            >
              {t("Resumir no chat")}
            </Button>
            <div className="flex-1" />
            <IconButton title={t("Excluir")} onClick={onDelete}>
              <Trash2 className="size-3.5" />
            </IconButton>
          </div>
        </div>
      </div>
    </Card>
  );
}

function Transcriber() {
  const app = useApp();
  const rec = useRecorder();
  const [items, setItems] = useState<Transcript[]>([]);
  const [fileBusy, setFileBusy] = useState("");

  const load = useCallback(() => api.transcripts().then((l) => setItems(l ?? [])), []);
  useEffect(() => {
    load();
  }, [load]);

  const transcribeFiles = useCallback(
    async (paths: string[]) => {
      for (const p of paths) {
        setFileBusy(p.split(/[\\/]/).pop() ?? p);
        rec.setProgress(0);
        try {
          const tr = await api.transcribeFile(p);
          setItems((l) => [tr, ...l]);
        } catch (e) {
          app.toast(errorText(e), "error");
        }
      }
      setFileBusy("");
    },
    [app, rec],
  );

  useEffect(() => onFileDrop((paths) => !fileBusy && rec.state === "idle" && transcribeFiles(paths)), [transcribeFiles, fileBusy, rec.state]);

  const busy = rec.state !== "idle" || !!fileBusy;
  const model = app.voiceModels[0];

  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      <div className="mx-auto max-w-[760px] px-6 py-6">
        <div className="mb-4 flex flex-wrap items-center gap-2 text-[12.5px]">
          <span className="text-muted">{t("Modelo")}</span>
          <ModelSelect models={app.voiceModels} value={model?.id ?? ""} onChange={(id) => api.setVoiceModel(id)} />
          <span className="ml-2 text-muted">{t("Idioma")}</span>
          <Select value={app.settings?.voiceLanguage ?? "auto"} onChange={(v) => api.setVoiceLanguage(v)} options={languages.map((l) => ({ value: l.value, label: t(l.label) }))} />
          {model && model.fit.realtimeFactor > 0 && (
            <span className="ml-auto text-[11.5px] text-muted">≈ {t("{factor}× o tempo do áudio para processar", { factor: model.fit.realtimeFactor.toFixed(1) })}</span>
          )}
        </div>

        <div
          data-file-drop-target
          className="mb-6 flex flex-col items-center rounded-2xl border border-dashed border-line bg-card px-6 py-8 text-center [&.file-drop-target-active]:border-accent [&.file-drop-target-active]:bg-accent-soft"
        >
          {rec.state === "recording" ? (
            <>
              <div className="text-bad">
                <LevelBars level={rec.level} bars={32} />
              </div>
              <div className="mt-2 text-[22px] font-semibold tabular-nums">{clock(rec.elapsed)}</div>
              <div className="mt-4 flex gap-2">
                <Button icon={<X className="size-3.5" />} onClick={rec.cancel}>
                  {t("Descartar")}
                </Button>
                <Button
                  variant="primary"
                  icon={<Square className="size-3 fill-current" />}
                  onClick={async () => {
                    const tr = await rec.stop(true);
                    if (tr) setItems((l) => [tr, ...l]);
                  }}
                >
                  {t("Parar e transcrever")}
                </Button>
              </div>
            </>
          ) : rec.state === "transcribing" || fileBusy ? (
            <div className="w-full max-w-sm">
              <div className="mb-2 flex items-center justify-center gap-2 text-[13px] font-medium">
                <Loader2 className="size-4 animate-spin text-accent" />
                {fileBusy ? t("Transcrevendo {name}…", { name: fileBusy }) : t("Transcrevendo gravação…")} {rec.progress > 0 && `${rec.progress}%`}
              </div>
              <Progress value={Math.max(0.03, rec.progress / 100)} />
              <Button className="mt-3" size="sm" onClick={() => api.cancelTranscription()}>
                {t("Cancelar")}
              </Button>
            </div>
          ) : (
            <>
              <button
                onClick={rec.start}
                disabled={busy}
                className="flex size-16 items-center justify-center rounded-full bg-bad text-white shadow-lg transition-transform hover:scale-105 active:scale-95"
                title={t("Gravar")}
              >
                <Mic className="size-7" />
              </button>
              <div className="mt-3 text-[14px] font-semibold">{t("Gravar do microfone")}</div>
              <p className="mt-1 text-[12.5px] text-muted">{t("ou arraste arquivos de áudio (WAV, MP3, OGG, FLAC) para cá")}</p>
              <Button
                className="mt-3"
                size="sm"
                icon={<FileAudio className="size-3.5" />}
                onClick={async () => {
                  const files = await api.pickAudioFiles().catch(() => []);
                  if (files?.length) transcribeFiles(files);
                }}
              >
                {t("Escolher arquivos…")}
              </Button>
            </>
          )}
        </div>

        {items.length > 0 && <h3 className="mb-2 px-1 text-[12px] font-semibold text-muted">{t("Transcrições")}</h3>}
        <div className="space-y-3">
          {items.map((item) => (
            <TranscriptCard
              key={item.id}
              tr={item}
              onDelete={async () => {
                await api.deleteTranscript(item.id);
                setItems((l) => l.filter((x) => x.id !== item.id));
              }}
            />
          ))}
        </div>
      </div>
    </div>
  );
}

export function VoicePage() {
  const app = useApp();
  return (
    <div className="flex h-full min-w-0 flex-1 flex-col bg-bg">
      <header className="drag flex h-[52px] shrink-0 items-center border-b border-line px-6">
        <h1 className="text-[14px] font-semibold">{t("Transcrição")}</h1>
      </header>
      <ToolGate tool="whisperfile" alternative={{ tool: "whisper-gpu", label: t("Baixar versão com GPU") }} icon={<AudioLines className="size-6" />}>
        <ModelGate category="voice" models={app.voiceModels} icon={<AudioLines className="size-6" />}>
          <Transcriber />
        </ModelGate>
      </ToolGate>
    </div>
  );
}
