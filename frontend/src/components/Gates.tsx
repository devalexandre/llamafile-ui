import { Download, Sparkles } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { api } from "../lib/api";
import { bytes } from "../lib/format";
import { errorText, useApp } from "../lib/store";
import type { ModelView, Release, ToolStatus } from "../lib/types";
import { DownloadProgress } from "./ModelCard";
import { Button, Card, Select } from "./ui";

const toolInfo: Record<ToolStatus["tool"], { title: string; text: string }> = {
  llamafile: { title: "Instale o llamafile", text: "Motor que roda os modelos de linguagem." },
  whisperfile: {
    title: "Instale o whisperfile",
    text: "Motor de transcrição de voz do projeto llamafile (whisper.cpp). É um único executável pequeno.",
  },
  diffusionfile: {
    title: "Instale o diffusionfile",
    text: "Motor de geração de imagens do projeto llamafile (stable-diffusion.cpp). É um único executável.",
  },
  "sd-cpp": {
    title: "Instale o stable-diffusion.cpp",
    text: "Versão do motor de imagens com GPU (Vulkan, ou Metal no Mac): gera imagens bem mais rápido.",
  },
  piper: {
    title: "Instale o Piper",
    text: "Motor de leitura em voz alta (texto para fala) que roda na CPU, com vozes naturais em português.",
  },
};

/** Shows a download card while a llamafile-release tool is missing. */
export function ToolGate({
  tool,
  alternative,
  icon,
  children,
}: {
  tool: ToolStatus["tool"];
  /** Another tool that does the same job (e.g. the GPU image engine). */
  alternative?: { tool: ToolStatus["tool"]; label: string };
  icon: ReactNode;
  children: ReactNode;
}) {
  const app = useApp();
  const status = app.tool(tool);
  const alt = alternative ? app.tool(alternative.tool) : undefined;
  const altDownload = alternative && app.downloads.find((d) => d.key === `tool:${alternative.tool}`);
  const [release, setRelease] = useState<Release | null>(null);
  const [busy, setBusy] = useState(false);
  const download = app.downloads.find((d) => d.key === `tool:${tool}`);

  useEffect(() => {
    if (status && !status.installed) api.latestRelease().then(setRelease);
  }, [status]);

  if (!status || status.installed || alt?.installed) return <>{children}</>;
  const info = toolInfo[tool];
  const size = release?.assets[tool]?.size;
  return (
    <div className="flex flex-1 items-start justify-center overflow-y-auto px-6 py-12">
      <Card className="w-full max-w-lg p-6">
        <div className="flex gap-4">
          <div className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-accent-soft text-accent">{icon}</div>
          <div className="min-w-0 flex-1">
            <div className="text-[15px] font-semibold">{info.title}</div>
            <p className="mt-1 text-[12.5px] text-muted">
              {info.text} Baixamos a versão {release?.version ?? "mais recente"} direto do GitHub oficial da Mozilla AI.
            </p>
            <div className="mt-4">
              {download || altDownload ? (
                <DownloadProgress d={(download ?? altDownload)!} onCancel={() => api.cancelDownload((download ?? altDownload)!.key)} />
              ) : (
                <div className="flex flex-wrap gap-2">
                <Button
                  variant="primary"
                  icon={<Download className="size-3.5" />}
                  loading={busy}
                  onClick={async () => {
                    setBusy(true);
                    try {
                      await api.downloadTool(tool);
                    } catch (e) {
                      app.toast(errorText(e), "error");
                    } finally {
                      setBusy(false);
                    }
                  }}
                >
                  Baixar {tool}
                  {size ? ` · ${bytes(size)}` : ""}
                </Button>
                {alternative && (
                  <Button
                    icon={<Download className="size-3.5" />}
                    onClick={() => api.downloadTool(alternative.tool).catch((e) => app.toast(errorText(e), "error"))}
                  >
                    {alternative.label}
                  </Button>
                )}
                </div>
              )}
            </div>
          </div>
        </div>
      </Card>
    </div>
  );
}

const categoryText: Record<string, { title: string; text: string }> = {
  voice: { title: "Nenhum modelo de voz instalado", text: "Baixe um modelo Whisper recomendado para o seu computador." },
  image: { title: "Nenhum modelo de imagem instalado", text: "Baixe um modelo de imagem recomendado para o seu computador." },
  video: {
    title: "Nenhum modelo de vídeo instalado",
    text: "Baixe um modelo de vídeo. Eles são grandes (5–8 GB com o codificador de texto) e funcionam bem melhor com a versão com GPU do motor de imagens.",
  },
  embedding: {
    title: "Nenhum modelo de embeddings instalado",
    text: "Os documentos são indexados com um modelo de embeddings. Baixe um recomendado — são pequenos.",
  },
};

/** Shows a call to action while no model of a category is installed. */
export function ModelGate({ category, models, icon, children }: { category: string; models: ModelView[]; icon: ReactNode; children: ReactNode }) {
  const app = useApp();
  const [pick, setPick] = useState<ModelView | null>(null);
  useEffect(() => {
    if (models.length === 0) api.topPicks().then((p) => setPick(p?.[category] ?? null));
  }, [models.length, category, app.catalogRevision]);
  if (models.length > 0) return <>{children}</>;
  const t = categoryText[category];
  const download = pick ? app.downloads.find((d) => d.key === `model:${pick.id}`) : undefined;
  return (
    <div className="flex flex-1 items-start justify-center overflow-y-auto px-6 py-12">
      <Card className="w-full max-w-lg p-6">
        <div className="flex gap-4">
          <div className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-accent-soft text-accent">{icon}</div>
          <div className="min-w-0 flex-1">
            <div className="text-[15px] font-semibold">{t.title}</div>
            <p className="mt-1 text-[12.5px] text-muted">{t.text}</p>
            {pick && (
              <div className="mt-4 rounded-lg bg-field p-3">
                <div className="flex items-center gap-1.5 text-[11.5px] font-semibold text-accent">
                  <Sparkles className="size-3" /> Melhor escolha para você
                </div>
                <div className="mt-0.5 text-[13px] font-semibold">{pick.name}</div>
                <div className="text-[12px] text-muted">{pick.description}</div>
                <div className="mt-3">
                  {download ? (
                    <DownloadProgress d={download} onCancel={() => api.cancelDownload(download.key)} />
                  ) : (
                    <Button
                      size="sm"
                      variant="primary"
                      icon={<Download className="size-3.5" />}
                      onClick={() => api.downloadModel(pick.id).catch((e) => app.toast(errorText(e), "error"))}
                    >
                      Baixar · {bytes(pick.fit.downloadBytes)}
                    </Button>
                  )}
                </div>
              </div>
            )}
            <Button className="mt-3" size="sm" variant="plain" onClick={() => app.openDiscover(category)}>
              Ver todos os modelos
            </Button>
          </div>
        </div>
      </Card>
    </div>
  );
}

export function ModelSelect({ models, value, onChange }: { models: ModelView[]; value: string; onChange: (id: string) => void }) {
  return (
    <Select
      value={value}
      onChange={onChange}
      options={models.map((m) => ({ value: m.id, label: `${m.name}${m.installedQuant ? ` · ${m.installedQuant}` : ""}` }))}
      className="max-w-[240px]"
    />
  );
}
