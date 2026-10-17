import clsx from "clsx";
import { AlertTriangle, CheckCircle2, Cpu, Download, FileSearch, FolderOpen, Play, RotateCcw, Square, Terminal, Volume2, Zap } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { DownloadProgress } from "../components/ModelCard";
import { Button, Card, Field, Row, Section, Segmented, Select, StatusDot, Switch } from "../components/ui";
import { api, on } from "../lib/api";
import { bytes } from "../lib/format";
import { errorText, useApp } from "../lib/store";
import type { BenchRecord, Release, Settings } from "../lib/types";

/** Shown when llamafile is missing: download (GPU or CPU build) or pick a binary. */
function LlamafileSetup() {
  const app = useApp();
  const [release, setRelease] = useState<Release | null>(null);
  const [busy, setBusy] = useState(false);
  const download = app.downloads.find((d) => d.key === "runtime:llamafile");
  const hasGpu = !!app.hardware && (app.hardware.vramGb > 0 || app.hardware.unifiedMemory);

  useEffect(() => {
    api.latestRelease().then(setRelease);
  }, []);

  const start = async (thin: boolean) => {
    setBusy(true);
    try {
      await api.downloadLlamafile(thin);
    } catch (e) {
      app.toast(errorText(e), "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card className="mb-6 overflow-hidden">
      <div className="flex gap-4 p-5">
        <div className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-accent-soft text-accent">
          <Zap className="size-6" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="text-[15px] font-semibold">Instale o llamafile para começar</div>
          <p className="mt-1 text-[12.5px] text-muted">
            O llamafile é o motor que roda os modelos no seu computador (Mozilla AI). Ele é um único executável — baixamos a versão
            {release ? ` ${release.version}` : " mais recente"} direto do GitHub oficial.
          </p>
          {download ? (
            <div className="mt-4 max-w-md">
              <div className="mb-1 text-[12px] font-medium">Baixando {download.label}…</div>
              <DownloadProgress d={download} onCancel={() => api.cancelDownload(download.key)} />
            </div>
          ) : (
            <div className="mt-4 flex flex-wrap items-center gap-2">
              <Button variant="primary" icon={<Download className="size-3.5" />} loading={busy} onClick={() => start(false)}>
                Baixar llamafile{release?.assets.llamafile ? ` · ${bytes(release.assets.llamafile.size)}` : ""}
              </Button>
              <Button icon={<Cpu className="size-3.5" />} disabled={busy} onClick={() => start(true)}>
                Versão leve, só CPU{release?.assets["llamafile-thin"] ? ` · ${bytes(release.assets["llamafile-thin"].size)}` : ""}
              </Button>
              <Button
                variant="plain"
                icon={<FileSearch className="size-3.5" />}
                onClick={() => api.chooseLlamafile().catch((e) => app.toast(errorText(e), "error"))}
              >
                Já tenho o arquivo…
              </Button>
            </div>
          )}
          <p className="mt-3 text-[11.5px] text-muted">
            {hasGpu
              ? "Recomendado: versão completa, que usa sua GPU para respostas bem mais rápidas."
              : "Sem GPU dedicada detectada: a versão leve é suficiente e baixa mais rápido."}
          </p>
        </div>
      </div>
    </Card>
  );
}

function ServerControl() {
  const app = useApp();
  const rt = app.runtime;
  const [busy, setBusy] = useState(false);
  const models = app.chatModels;
  const [modelId, setModelId] = useState("");
  useEffect(() => {
    if (!modelId || !models.some((m) => m.id === modelId)) {
      setModelId(models.find((m) => m.id === app.settings?.lastModelId)?.id ?? models[0]?.id ?? "");
    }
  }, [models, modelId, app.settings?.lastModelId]);
  if (!rt) return null;

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
  const running = rt.state === "running" || rt.state === "starting";

  return (
    <Section title="Servidor" footer={rt.state === "running" && <>API compatível com OpenAI em <span className="selectable font-mono">{rt.baseUrl}/v1</span></>}>
      <Row
        label={
          <span className="flex items-center gap-2">
            <StatusDot state={rt.state} />
            {rt.state === "running" && <>Rodando {rt.modelName}</>}
            {rt.state === "starting" && <>Carregando {rt.modelName}…</>}
            {rt.state === "stopped" && "Parado"}
            {rt.state === "error" && (rt.message || "Erro")}
          </span>
        }
        hint={
          rt.state === "error" ? (
            <span className="text-bad">{rt.error}</span>
          ) : (
            <span className="selectable block truncate" title={rt.binaryPath}>
              llamafile {rt.version} · <span className="font-mono">{rt.binaryPath}</span>
            </span>
          )
        }
      >
        {running ? (
          <>
            {rt.needsRestart && (
              <Button size="sm" variant="primary" icon={<RotateCcw className="size-3" />} loading={busy} onClick={() => run(api.restartServer)}>
                Reiniciar para aplicar
              </Button>
            )}
            <Button size="sm" icon={<Square className="size-3" />} onClick={() => run(api.stopServer)}>
              Parar
            </Button>
          </>
        ) : (
          <>
            {models.length > 0 ? (
              <>
                <Select value={modelId} onChange={setModelId} options={models.map((m) => ({ value: m.id, label: m.name }))} className="max-w-[200px]" />
                <Button size="sm" variant="primary" icon={<Play className="size-3" />} loading={busy} onClick={() => run(() => api.startModel(modelId))}>
                  Iniciar
                </Button>
              </>
            ) : (
              <Button size="sm" variant="primary" onClick={() => app.setPage("discover")}>
                Baixar um modelo
              </Button>
            )}
          </>
        )}
      </Row>
      <Row label="Executável" hint="Use outro binário do llamafile (ex.: versão personalizada).">
        <Button size="sm" onClick={() => api.chooseLlamafile().catch((e) => app.toast(errorText(e), "error"))}>
          Escolher…
        </Button>
        <Button size="sm" variant="plain" onClick={() => run(() => api.downloadLlamafile(false))}>
          Atualizar
        </Button>
      </Row>
    </Section>
  );
}

/** Compares dotted versions ("0.10.7" > "0.10.6"). */
function newer(a: string, b: string): boolean {
  const pa = a.split(".").map(Number);
  const pb = b.split(".").map(Number);
  if (pa.some(isNaN) || pb.some(isNaN)) return false;
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const d = (pa[i] ?? 0) - (pb[i] ?? 0);
    if (d) return d > 0;
  }
  return false;
}

/** Speeds measured on this machine, used to calibrate the recommendations. */
function BenchSection() {
  const app = useApp();
  const [records, setRecords] = useState<BenchRecord[]>([]);
  const load = () =>
    api
      .benchmarks()
      .then((r) => setRecords(r ?? []))
      .catch(() => {});
  useEffect(() => {
    load();
    return on("bench:updated", load);
  }, []);
  const name = (id: string) => app.installed.find((m) => m.id === id)?.name ?? id;
  const value = (r: BenchRecord) =>
    r.kind === "llm" ? `${r.value.toFixed(1)} tok/s` : r.kind === "image" ? `${r.value.toFixed(1)} s/imagem` : `${r.value.toFixed(2)}× a duração`;
  return (
    <Section
      title="Medições de velocidade"
      footer="Vêm das suas conversas, imagens e transcrições (ou do botão Medir na Biblioteca) e ajustam as estimativas de todos os modelos."
    >
      {records.length === 0 ? (
        <Row label="Nenhuma medição ainda" hint="Use um modelo ou clique em Medir na Biblioteca para calibrar as recomendações." />
      ) : (
        records.map((r) => (
          <Row
            key={`${r.modelId}|${r.quant}|${r.mode}`}
            label={
              <span>
                {name(r.modelId)}{" "}
                <span className="text-[11.5px] text-muted">
                  {r.quant} · {r.mode}
                </span>
              </span>
            }
            hint={`${r.samples} ${r.samples === 1 ? "amostra" : "amostras"} · ${Math.round(r.ratio * 100)}% da estimativa${
              r.promptTps > 0 ? ` · leitura do prompt ${Math.round(r.promptTps)} tok/s` : ""
            }`}
          >
            <span className="text-[12.5px] font-semibold">{value(r)}</span>
          </Row>
        ))
      )}
      {records.length > 0 && (
        <Row label="Recomeçar calibração" hint="Apaga as medições e volta às estimativas pelo hardware.">
          <Button size="sm" variant="plain" icon={<RotateCcw className="size-3.5" />} onClick={() => api.resetBenchmarks().then(load)}>
            Apagar
          </Button>
        </Row>
      )}
    </Section>
  );
}

const toolLabels: Record<string, { name: string; hint: string }> = {
  whisperfile: { name: "whisperfile", hint: "Transcrição de voz e ditado (whisper.cpp)." },
  diffusionfile: { name: "diffusionfile", hint: "Geração de imagens (stable-diffusion.cpp)." },
  piper: { name: "Piper", hint: "Leitura das respostas em voz alta (projeto Piper, roda na CPU)." },
};

/** Piper voice, speed and auto-read for chat answers. */
function SpeechSection() {
  const app = useApp();
  const s = app.settings!;
  const piper = app.tools.find((t) => t.tool === "piper");
  const voices = app.ttsModels;
  const voice = voices.find((v) => v.id === s.ttsVoiceId) ?? voices[0];
  const ready = !!piper?.installed && voices.length > 0;
  return (
    <Section
      title="Leitura em voz alta"
      footer={
        !piper?.installed
          ? "Instale o Piper em Ferramentas para ouvir as respostas."
          : voices.length === 0
            ? "Baixe uma voz em Descobrir → Leitura em voz alta."
            : "Clique no ícone de alto-falante em uma resposta para ouvi-la."
      }
    >
      <Row label="Voz" hint={voice ? voice.description : "Nenhuma voz instalada."}>
        {voices.length > 0 ? (
          <>
            <Select
              value={voice?.id ?? ""}
              onChange={(v) => app.saveSettings({ ttsVoiceId: v })}
              options={voices.map((v) => ({ value: v.id, label: v.name }))}
            />
            <Button
              size="sm"
              variant="plain"
              icon={<Volume2 className="size-3.5" />}
              disabled={!ready}
              onClick={() => voice && api.previewVoice(voice.id).catch((e) => app.toast(errorText(e), "error"))}
            >
              Testar
            </Button>
          </>
        ) : (
          <Button size="sm" onClick={() => app.setPage("discover")}>
            Ver vozes
          </Button>
        )}
      </Row>
      <Row label="Velocidade" hint="Ritmo da fala.">
        <input
          type="range"
          min={0.5}
          max={2}
          step={0.1}
          defaultValue={s.ttsSpeed}
          onMouseUp={(e) => app.saveSettings({ ttsSpeed: Number((e.target as HTMLInputElement).value) })}
          className="w-[140px] accent-[var(--accent)]"
        />
        <span className="w-9 text-right text-[12px] text-muted tabular-nums">{s.ttsSpeed.toFixed(1)}×</span>
      </Row>
      <Row label="Ler respostas automaticamente" hint="Cada nova resposta do chat é lida assim que termina.">
        <Switch checked={s.autoSpeak} disabled={!ready} onChange={(v) => app.saveSettings({ autoSpeak: v })} />
      </Row>
    </Section>
  );
}

/** Other executables from the llamafile release + the embeddings server. */
function ToolsSection() {
  const app = useApp();
  const emb = app.embedder;
  const [release, setRelease] = useState<Release | null>(null);
  const [probing, setProbing] = useState(false);
  useEffect(() => {
    api
      .latestRelease()
      .then(setRelease)
      .catch(() => {});
  }, []);
  const probe = () => {
    setProbing(true);
    api
      .detectToolGPU()
      .catch((e) => app.toast(errorText(e), "error"))
      .finally(() => setProbing(false));
  };
  return (
    <Section title="Ferramentas" footer="whisperfile e diffusionfile vêm da release oficial do llamafile; o Piper, do projeto Piper. Tudo roda localmente.">
      {app.tools
        .filter((t) => t.tool !== "llamafile")
        .map((t) => {
          const d = app.downloads.find((x) => x.key === `tool:${t.tool}`);
          return (
            <Row
              key={t.tool}
              label={
                <span className="flex items-center gap-2">
                  <StatusDot state={t.installed ? "running" : "stopped"} />
                  {toolLabels[t.tool].name}
                  {t.installed && <span className="text-[11.5px] text-muted">v{t.version}</span>}
                  {t.installed && t.probed && (
                    <span
                      title={`Backends: ${t.backends.join(", ")}`}
                      className={clsx("rounded px-1.5 py-px text-[10.5px] font-semibold", t.gpu ? "bg-ok/15 text-ok" : "bg-hover text-muted")}
                    >
                      {t.gpu ? `GPU · ${t.backends.filter((b) => b.toUpperCase() !== "CPU").join(", ")}` : "só CPU"}
                    </span>
                  )}
                  {t.installed && t.tool !== "piper" && release && newer(release.version, t.version) && (
                    <span className="rounded bg-accent/15 px-1.5 py-px text-[10.5px] font-semibold text-accent">v{release.version} disponível</span>
                  )}
                </span>
              }
              hint={
                <>
                  {toolLabels[t.tool].hint}
                  {t.installed && t.probed && !t.gpu && " Esta versão roda só na CPU; quando uma release trouxer GPU, atualize para acelerar."}
                </>
              }
            >
              {d ? (
                <div className="w-[220px]">
                  <DownloadProgress d={d} onCancel={() => api.cancelDownload(d.key)} />
                </div>
              ) : (
                <Button
                  size="sm"
                  variant={t.installed ? "plain" : "primary"}
                  icon={<Download className="size-3.5" />}
                  onClick={() => api.downloadTool(t.tool).catch((e) => app.toast(errorText(e), "error"))}
                >
                  {t.installed ? "Atualizar" : "Baixar"}
                </Button>
              )}
              {t.installed && !d && t.tool !== "piper" && (
                <Button size="sm" variant="plain" icon={<Cpu className="size-3.5" />} loading={probing} onClick={probe} title="Detectar de novo se a ferramenta usa a GPU">
                  Detectar GPU
                </Button>
              )}
            </Row>
          );
        })}
      {emb && emb.state !== "missing" && (
        <Row
          label={
            <span className="flex items-center gap-2">
              <StatusDot state={emb.state} />
              Servidor de embeddings
            </span>
          }
          hint={
            emb.state === "running"
              ? `${emb.modelName} · porta ${emb.port} (porta do chat + 1)`
              : emb.state === "error"
                ? emb.error
                : "Inicia sozinho quando você indexa ou pesquisa documentos."
          }
        >
          {(emb.state === "running" || emb.state === "starting") && (
            <Button size="sm" icon={<Square className="size-3" />} onClick={() => api.stopEmbedder()}>
              Parar
            </Button>
          )}
        </Row>
      )}
    </Section>
  );
}

function Logs() {
  const [open, setOpen] = useState(false);
  const [lines, setLines] = useState<string[]>([]);
  const ref = useRef<HTMLPreElement>(null);
  useEffect(() => {
    if (!open) return;
    api.logs().then(setLines);
    return on<string>("runtime:log", (l) => setLines((x) => [...x.slice(-399), l]));
  }, [open]);
  useEffect(() => {
    if (ref.current) ref.current.scrollTop = ref.current.scrollHeight;
  }, [lines]);
  return (
    <Section title="Diagnóstico">
      <Row label="Log do servidor" hint="Saída do llamafile, útil para investigar problemas.">
        <Button size="sm" icon={<Terminal className="size-3.5" />} onClick={() => setOpen((o) => !o)}>
          {open ? "Ocultar" : "Mostrar"}
        </Button>
      </Row>
      {open && (
        <pre ref={ref} className="selectable max-h-[280px] overflow-auto bg-field px-3.5 py-2.5 font-mono text-[11px] leading-relaxed text-fg/80">
          {lines.length ? lines.join("\n") : "Sem saída ainda."}
        </pre>
      )}
    </Section>
  );
}

export function SettingsPage() {
  const app = useApp();
  const s = app.settings;
  const [port, setPort] = useState("");
  const [host, setHost] = useState("");
  const [prompt, setPrompt] = useState("");
  useEffect(() => {
    if (!s) return;
    setPort(String(s.port));
    setHost(s.host);
    setPrompt(s.systemPrompt);
  }, [s?.port, s?.host, s?.systemPrompt]); // eslint-disable-line react-hooks/exhaustive-deps

  if (!s || !app.runtime) return null;
  const save = (patch: Partial<Settings>) => app.saveSettings(patch);
  const missing = app.runtime.state === "missing";
  const portNumber = Number(port);
  const portInvalid = !Number.isInteger(portNumber) || portNumber < 1024 || portNumber > 65535;

  return (
    <div className="flex h-full min-w-0 flex-1 flex-col bg-bg">
      <header className="drag flex h-[52px] shrink-0 items-center border-b border-line px-6">
        <h1 className="text-[14px] font-semibold">Configurações</h1>
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto max-w-[680px] px-6 py-6">
          {missing ? <LlamafileSetup /> : <ServerControl />}
          <ToolsSection />
          <BenchSection />

          <Section
            title="Rede"
            footer={
              app.runtime.needsRestart
                ? "A nova porta/endereço será usada quando o servidor reiniciar."
                : "Use 127.0.0.1 para aceitar conexões só deste computador; 0.0.0.0 expõe na rede local."
            }
          >
            <Row label="Porta" hint="Porta HTTP da API do llamafile (padrão 8686).">
              <Field
                value={port}
                inputMode="numeric"
                onChange={(e) => setPort(e.target.value.replace(/\D/g, ""))}
                onBlur={() => !portInvalid && portNumber !== s.port && save({ port: portNumber })}
                onKeyDown={(e) => e.key === "Enter" && (e.target as HTMLInputElement).blur()}
                className={`w-[90px] text-right ${portInvalid ? "border-bad" : ""}`}
              />
            </Row>
            <Row label="Endereço" hint="Interface onde o servidor escuta.">
              <Select
                value={host}
                onChange={(v) => {
                  setHost(v);
                  save({ host: v });
                }}
                options={[
                  { value: "127.0.0.1", label: "Somente este computador" },
                  { value: "0.0.0.0", label: "Rede local (0.0.0.0)" },
                ]}
              />
            </Row>
          </Section>

          <Section title="Desempenho" footer="Mudanças valem no próximo carregamento do modelo.">
            <Row label="Aceleração" hint="Qual GPU o llamafile deve usar.">
              <Select
                value={s.gpu}
                onChange={(v) => save({ gpu: v })}
                options={[
                  { value: "auto", label: "Automático" },
                  { value: "nvidia", label: "NVIDIA (CUDA)" },
                  { value: "amd", label: "AMD (ROCm)" },
                  { value: "apple", label: "Apple (Metal)" },
                  { value: "disable", label: "Somente CPU" },
                ]}
              />
            </Row>
            <Row label="Tamanho do contexto" hint="Quanto da conversa o modelo lembra. Mais contexto usa mais memória.">
              <Select
                value={String(s.contextSize)}
                onChange={(v) => save({ contextSize: Number(v) })}
                options={[2048, 4096, 8192, 16384, 32768, 65536].map((n) => ({ value: String(n), label: `${n / 1024}K tokens` }))}
              />
            </Row>
            <Row label="Threads da CPU" hint={`0 = automático (${app.hardware?.cpuThreads ?? "?"} disponíveis).`}>
              <Field
                type="number"
                min={0}
                max={256}
                defaultValue={s.threads}
                onBlur={(e) => save({ threads: Number(e.target.value) || 0 })}
                className="w-[70px] text-right"
              />
            </Row>
            <Row label="Iniciar com o app" hint="Carrega o último modelo usado ao abrir o aplicativo.">
              <Switch checked={s.autoStart} onChange={(v) => save({ autoStart: v })} />
            </Row>
            <Row
              label="Manter na bandeja ao fechar"
              hint="Fechar a janela deixa o app e o servidor rodando no ícone da bandeja (a API continua disponível). Use “Sair” no menu da bandeja para encerrar."
            >
              <Switch checked={s.closeToTray} onChange={(v) => save({ closeToTray: v })} />
            </Row>
            <Row label="Restaurar padrões do servidor">
              <Button size="sm" onClick={() => api.resetServerSettings()}>
                Restaurar
              </Button>
            </Row>
          </Section>

          <Section title="Chat">
            <div className="px-3.5 py-3">
              <div className="mb-1 text-[13px]">Instruções do sistema</div>
              <div className="mb-2 text-[11.5px] text-muted">Enviadas no início de toda conversa nova.</div>
              <textarea
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                onBlur={() => prompt !== s.systemPrompt && save({ systemPrompt: prompt })}
                rows={3}
                className="w-full resize-y rounded-[7px] border border-line bg-card px-2.5 py-2 text-[13px] outline-none focus:border-accent focus:ring-3 focus:ring-accent-soft"
              />
            </div>
            <Row label="Criatividade (temperatura)" hint="Baixa = respostas mais previsíveis; alta = mais variadas.">
              <input
                type="range"
                min={0}
                max={1.5}
                step={0.1}
                defaultValue={s.temperature}
                onMouseUp={(e) => save({ temperature: Number((e.target as HTMLInputElement).value) })}
                className="w-[140px] accent-[var(--accent)]"
              />
              <span className="w-7 text-right text-[12px] text-muted tabular-nums">{s.temperature.toFixed(1)}</span>
            </Row>
            <Row label="Gerar títulos automaticamente" hint="Usa o próprio modelo para nomear novas conversas.">
              <Switch checked={s.autoTitle} onChange={(v) => save({ autoTitle: v })} />
            </Row>
          </Section>

          <SpeechSection />

          <Section title="Aparência">
            <Row label="Tema" hint="Nocturne — tema próprio inspirado no Dracula e no One Dark.">
              <Segmented
                value={s.theme}
                onChange={(v) => save({ theme: v })}
                options={[
                  { value: "system", label: "Sistema" },
                  { value: "light", label: "Claro" },
                  { value: "dark", label: "Escuro" },
                ]}
              />
            </Row>
          </Section>

          <Section title="Armazenamento">
            <Row label="Pasta de modelos" hint={<span className="selectable font-mono">{s.modelsDir}</span>}>
              <Button size="sm" onClick={() => api.chooseModelsDir().catch((e) => app.toast(errorText(e), "error"))}>
                Alterar…
              </Button>
              <Button size="sm" variant="plain" icon={<FolderOpen className="size-3.5" />} onClick={() => api.openModelsFolder()}>
                Abrir
              </Button>
            </Row>
            <Row label="Dados do aplicativo" hint="Conversas, anexos e binários do llamafile.">
              <Button size="sm" variant="plain" icon={<FolderOpen className="size-3.5" />} onClick={() => api.openDataFolder()}>
                Abrir
              </Button>
            </Row>
            <Row label="Apagar todas as conversas" hint="Remove o histórico permanentemente.">
              <Button
                size="sm"
                variant="danger"
                onClick={async () => {
                  if (!confirm("Apagar todas as conversas? Essa ação não pode ser desfeita.")) return;
                  await api.deleteAllConversations();
                  app.newChat();
                  app.toast("Conversas apagadas.");
                }}
              >
                Apagar tudo
              </Button>
            </Row>
          </Section>

          <Logs />

          <p className="flex items-center justify-center gap-1.5 pb-4 text-[11px] text-faint">
            {missing ? <AlertTriangle className="size-3" /> : <CheckCircle2 className="size-3" />}
            Llamafile UI · seus dados nunca saem deste computador
          </p>
        </div>
      </div>
    </div>
  );
}
