import { FolderOpen, Gauge, HardDrive, MessageSquare, Play, Square, Trash2 } from "lucide-react";
import { DownloadProgress, pageFor, SpeedSource, speedText, useModelActions } from "../components/ModelCard";
import { Button, EmptyState, FitBadge, StatusDot, Tag } from "../components/ui";
import { api } from "../lib/api";
import { bytes, gb } from "../lib/format";
import { useApp } from "../lib/store";
import type { ModelView } from "../lib/types";
import { msg, t } from "../i18n";

const categoryLabel: Record<string, string> = {
  chat: msg("Chat"),
  code: msg("Código"),
  reasoning: msg("Raciocínio"),
  vision: msg("Visão"),
  voice: msg("Voz"),
  image: msg("Imagens"),
  embedding: msg("Embeddings"),
  tts: msg("Leitura em voz alta"),
};

function InstalledRow({ m }: { m: ModelView }) {
  const app = useApp();
  const a = useModelActions(m);
  const usable = !pageFor(m);
  const state = a.active ? app.runtime!.state : "stopped";
  return (
    <div className="flex items-center gap-4 px-4 py-3">
      <StatusDot state={state} />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate text-[13.5px] font-semibold">{m.name}</span>
          <Tag>{m.installedQuant}</Tag>
          {m.categories.map((c) => (
            <Tag key={c} className="hidden lg:inline">
              {categoryLabel[c] ? t(categoryLabel[c]) : c}
            </Tag>
          ))}
        </div>
        <div className="mt-0.5 text-[12px] text-muted">
          {t("{size} no disco · precisa de ~{required}", { size: bytes(m.installedBytes), required: gb(m.fit.requiredGb) })} · {m.fit.mode}
          {` · ${speedText(m)} `}
          <SpeedSource m={m} />
        </div>
      </div>
      <FitBadge level={m.fit.level} label={m.fit.label} />
      <div className="flex w-[340px] items-center justify-end gap-2">
        {usable ? (
          <>
            {a.active ? (
              <Button size="sm" icon={<Square className="size-3" />} onClick={() => api.stopServer()}>
                {t("Descarregar")}
              </Button>
            ) : (
              <Button size="sm" icon={<Play className="size-3" />} loading={a.busy} onClick={a.load} disabled={app.runtime?.state === "missing"}>
                {t("Carregar")}
              </Button>
            )}
            <Button size="sm" icon={<Gauge className="size-3.5" />} loading={a.busy} onClick={a.measure} title={t("Medir a velocidade real neste computador")}>
              {t("Medir")}
            </Button>
            <Button size="sm" variant="primary" icon={<MessageSquare className="size-3.5" />} onClick={a.chat}>
              {t("Conversar")}
            </Button>
          </>
        ) : (
          (() => {
            const p = pageFor(m);
            return (
              p && (
                <Button size="sm" variant="primary" icon={p.icon} onClick={() => app.setPage(p.page)}>
                  {p.label}
                </Button>
              )
            );
          })()
        )}
        <button title={t("Remover do disco")} onClick={a.remove} className="rounded-md p-1.5 text-muted hover:bg-hover hover:text-bad">
          <Trash2 className="size-3.5" />
        </button>
      </div>
    </div>
  );
}

export function LibraryPage() {
  const app = useApp();
  const total = app.installed.reduce((s, m) => s + m.installedBytes, 0);
  const modelDownloads = app.downloads;

  return (
    <div className="flex h-full min-w-0 flex-1 flex-col bg-bg">
      <header className="drag flex h-[52px] shrink-0 items-center gap-3 border-b border-line px-6">
        <h1 className="text-[14px] font-semibold">{t("Meus modelos")}</h1>
        <span className="text-[12px] text-muted">
          {app.installed.length === 1
            ? t("{n} instalado · {size}", { n: app.installed.length, size: bytes(total) })
            : t("{n} instalados · {size}", { n: app.installed.length, size: bytes(total) })}
        </span>
        <div className="flex-1" />
        <Button size="sm" icon={<FolderOpen className="size-3.5" />} onClick={() => api.openModelsFolder()}>
          {t("Abrir pasta")}
        </Button>
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto max-w-[1040px] px-6 py-5">
          {modelDownloads.length > 0 && (
            <section className="mb-6">
              <h3 className="mb-1.5 px-1 text-[12px] font-semibold text-muted">{t("Baixando")}</h3>
              <div className="divide-y divide-line rounded-[10px] border border-line bg-card">
                {modelDownloads.map((d) => (
                  <div key={d.key} className="flex items-center gap-4 px-4 py-3">
                    <div className="w-[220px] truncate text-[13px] font-medium">
                      {d.label}
                      {d.files > 1 && <span className="ml-1 text-[11px] text-muted">({d.file}/{d.files})</span>}
                    </div>
                    <DownloadProgress d={d} onCancel={() => api.cancelDownload(d.key)} />
                  </div>
                ))}
              </div>
            </section>
          )}

          {app.installed.length === 0 ? (
            <EmptyState
              icon={<HardDrive className="size-10" />}
              title={t("Nenhum modelo instalado")}
              text={t("Baixe um modelo recomendado para o seu computador ou coloque arquivos .gguf na pasta de modelos.")}
              action={
                <Button variant="primary" onClick={() => app.setPage("discover")}>
                  {t("Descobrir modelos")}
                </Button>
              }
            />
          ) : (
            <section>
              <h3 className="mb-1.5 px-1 text-[12px] font-semibold text-muted">{t("Instalados")}</h3>
              <div className="divide-y divide-line rounded-[10px] border border-line bg-card">
                {app.installed.map((m) => (
                  <InstalledRow key={m.id} m={m} />
                ))}
              </div>
              <p className="mt-2 px-1 text-[11.5px] text-muted">
                {t("Arquivos .gguf colocados manualmente em {dir} também aparecem aqui.", { dir: app.settings?.modelsDir ?? "" })}
              </p>
            </section>
          )}
        </div>
      </div>
    </div>
  );
}
