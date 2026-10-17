import clsx from "clsx";
import { AudioLines, BookOpen, Cpu, Download, Gauge, Image as ImageIcon, ImagePlus, MemoryStick, MessageSquare, Play, Trash2, X } from "lucide-react";
import { useState } from "react";
import { api } from "../lib/api";
import { bytes, duration, gb, params } from "../lib/format";
import { errorText, useApp } from "../lib/store";
import type { ModelView, Page } from "../lib/types";
import { Button, FitBadge, Progress, Tag } from "./ui";

export function useModelActions(m: ModelView) {
  const app = useApp();
  const [busy, setBusy] = useState(false);
  const key = `model:${m.id}`;
  const download = app.downloads.find((d) => d.key === key);
  const active = app.runtime?.modelId === m.id && (app.runtime.state === "running" || app.runtime.state === "starting");

  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    try {
      await fn();
    } catch (e) {
      app.toast(errorText(e), "error");
    } finally {
      setBusy(false);
    }
  };

  return {
    download,
    active,
    busy,
    startDownload: (quant = "") => run(() => api.downloadModel(m.id, quant)),
    cancel: () => api.cancelDownload(key),
    remove: () => {
      if (!confirm(`Remover ${m.name} do disco? (${bytes(m.installedBytes)})`)) return;
      run(() => api.removeModel(m.id));
    },
    load: () => run(() => api.startModel(m.id)),
    chat: () => {
      app.saveSettings({ lastModelId: m.id });
      app.newChat();
    },
  };
}

export function DownloadProgress({ d, onCancel }: { d: { progress: number; received: number; total: number; speed: number; eta: number }; onCancel: () => void }) {
  return (
    <div className="w-full">
      <div className="mb-1 flex items-center justify-between text-[11.5px] text-muted">
        <span>
          {bytes(d.received)} de {bytes(d.total)} · {bytes(d.speed)}/s
        </span>
        <span className="flex items-center gap-1.5">
          {duration(d.eta)}
          <button onClick={onCancel} title="Cancelar" className="rounded-full p-0.5 hover:bg-hover hover:text-fg">
            <X className="size-3" />
          </button>
        </span>
      </div>
      <Progress value={d.progress} />
    </div>
  );
}

/** Human estimate of how fast a model runs here, per model kind. */
export function speedText(m: ModelView): string {
  if (m.fit.secondsPerImage > 0) return `~${duration(m.fit.secondsPerImage)}/imagem`;
  if (m.fit.realtimeFactor > 0) {
    const f = m.fit.realtimeFactor;
    return f < 1 ? `${(1 / f).toFixed(1)}× tempo real` : `${f.toFixed(1)}× a duração`;
  }
  if (m.fit.tps > 0) return `~${Math.round(m.fit.tps)} tok/s`;
  return m.fit.speedLabel;
}

/** Where a non-chat model is used in the app. */
export function pageFor(m: ModelView): { page: Page; label: string; icon: React.ReactNode } | null {
  if (m.runtime === "whisperfile") return { page: "voice", label: "Transcrever", icon: <AudioLines className="size-3.5" /> };
  if (m.runtime === "diffusionfile") return { page: "images", label: "Gerar imagens", icon: <ImagePlus className="size-3.5" /> };
  if (m.categories.includes("embedding")) return { page: "documents", label: "Usar em Documentos", icon: <BookOpen className="size-3.5" /> };
  return null;
}

export function ModelCard({ m, highlight }: { m: ModelView; highlight?: boolean }) {
  const app = useApp();
  const a = useModelActions(m);
  const usePage = pageFor(m);
  const tight = m.fit.level === "tight";

  return (
    <div
      className={clsx(
        "flex flex-col rounded-xl border bg-card p-4 transition-shadow hover:shadow-mac",
        highlight ? "border-accent/40 ring-1 ring-accent/20" : "border-line",
        tight && "opacity-70",
      )}
    >
      <div className="mb-1 flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-1.5">
            <h3 className="truncate text-[14px] font-semibold">{m.name}</h3>
            {m.vision && <ImageIcon className="size-3.5 shrink-0 text-muted" aria-label="Entende imagens" />}
          </div>
          <div className="text-[12px] text-muted">
            {m.publisher} · {params(m.params_b)}
            {m.active_params_b < m.params_b && ` (${params(m.active_params_b)} ativos)`} · {m.license}
          </div>
        </div>
        <FitBadge level={m.fit.level} label={m.fit.label} />
      </div>

      <p className="mt-1.5 mb-3 line-clamp-2 min-h-[36px] text-[12.5px] leading-snug text-fg/80">{m.description}</p>

      <div className="mb-3 grid grid-cols-3 gap-2 rounded-lg bg-field px-3 py-2 text-[11.5px]">
        <div>
          <div className="flex items-center gap-1 text-muted">
            <Gauge className="size-3" /> Velocidade
          </div>
          <div className="font-semibold" title={m.fit.speedLabel}>
            {speedText(m)}
          </div>
        </div>
        <div>
          <div className="flex items-center gap-1 text-muted">
            <MemoryStick className="size-3" /> Memória
          </div>
          <div className="font-semibold">{gb(m.fit.requiredGb)}</div>
        </div>
        <div>
          <div className="flex items-center gap-1 text-muted">
            <Cpu className="size-3" /> Roda em
          </div>
          <div className="font-semibold">{m.fit.mode}</div>
        </div>
      </div>

      <div className="mb-3 flex flex-wrap gap-1">
        <Tag>{m.installed ? m.installedQuant : m.fit.quant}</Tag>
        {m.tags.map((t) => (
          <Tag key={t}>{t}</Tag>
        ))}
      </div>

      <div className="mt-auto flex items-center gap-2">
        {a.download ? (
          <DownloadProgress d={a.download} onCancel={a.cancel} />
        ) : m.installed ? (
          <>
            {usePage ? (
              <Button variant="primary" size="sm" icon={usePage.icon} onClick={() => app.setPage(usePage.page)}>
                {usePage.label}
              </Button>
            ) : (
              <>
                <Button variant="primary" size="sm" icon={<MessageSquare className="size-3.5" />} onClick={a.chat}>
                  Conversar
                </Button>
                {!a.active && (
                  <Button size="sm" icon={<Play className="size-3" />} loading={a.busy} onClick={a.load} disabled={app.runtime?.state === "missing"}>
                    Carregar
                  </Button>
                )}
                {a.active && <span className="text-[12px] font-medium text-ok">Em uso</span>}
              </>
            )}
            <div className="flex-1" />
            <button title="Remover" onClick={a.remove} className="rounded-md p-1 text-muted hover:bg-hover hover:text-bad">
              <Trash2 className="size-3.5" />
            </button>
          </>
        ) : (
          <>
            <Button
              variant={highlight ? "primary" : "secondary"}
              size="sm"
              icon={<Download className="size-3.5" />}
              loading={a.busy}
              onClick={() => a.startDownload()}
            >
              Baixar · {bytes(m.fit.downloadBytes)}
            </Button>
            {m.files.length > 1 && (
              <select
                className="h-[24px] rounded-md border border-line bg-card px-1 text-[11.5px] text-muted outline-none"
                value=""
                onChange={(e) => e.target.value && a.startDownload(e.target.value)}
                title="Escolher outra quantização"
              >
                <option value="">Outras versões</option>
                {m.files.map((f) => (
                  <option key={f.quant} value={f.quant}>
                    {f.quant} · {bytes(f.size + (m.mmproj?.size ?? 0))}
                  </option>
                ))}
              </select>
            )}
          </>
        )}
      </div>
    </div>
  );
}
