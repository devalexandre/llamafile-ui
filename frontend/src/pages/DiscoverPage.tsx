import { Brain, Code2, Cpu, Eye, HardDrive, ImagePlus, Layers, MemoryStick, MessageSquare, Mic, MonitorSmartphone, RefreshCw, Sparkles, Volume2, Zap } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { ModelCard, speedText } from "../components/ModelCard";
import { Button, Card, IconButton, Segmented, Switch } from "../components/ui";
import { api } from "../lib/api";
import { gb } from "../lib/format";
import { useApp } from "../lib/store";
import type { ModelView } from "../lib/types";
import { msg, t } from "../i18n";

const categoryIcons: Record<string, React.ReactNode> = {
  chat: <MessageSquare className="size-3.5" />,
  code: <Code2 className="size-3.5" />,
  reasoning: <Brain className="size-3.5" />,
  vision: <Eye className="size-3.5" />,
  image: <ImagePlus className="size-3.5" />,
  voice: <Mic className="size-3.5" />,
  embedding: <Layers className="size-3.5" />,
  tts: <Volume2 className="size-3.5" />,
};

const categories = [
  { id: "chat", label: msg("Chat"), description: msg("Assistentes para conversar, escrever e resumir.") },
  { id: "code", label: msg("Código"), description: msg("Gerar, explicar e refatorar código.") },
  { id: "reasoning", label: msg("Raciocínio"), description: msg("Lógica, matemática e problemas passo a passo.") },
  { id: "vision", label: msg("Visão"), description: msg("Entendem imagens, capturas de tela e documentos.") },
  { id: "image", label: msg("Imagens"), description: msg("Geram imagens a partir de uma descrição em texto.") },
  { id: "voice", label: msg("Voz"), description: msg("Transcrição de áudio e ditado com Whisper.") },
  { id: "embedding", label: msg("Embeddings"), description: msg("Indexam seus documentos para o chat responder com base neles.") },
  { id: "tts", label: msg("Leitura em voz alta"), description: msg("Vozes para ouvir as respostas do chat.") },
];

const categoryNotes: Record<string, string> = {
  image: msg("Modelos de imagem rodam com o diffusionfile, na CPU (ou Metal no Mac). Os tempos abaixo são estimados para este computador."),
  voice: msg("Modelos de voz rodam com o whisperfile, na CPU. “Tempo real” significa que 1 minuto de áudio leva ~1 minuto para transcrever."),
  tts: msg("Vozes do Piper: leves (~60 MB), rodam na CPU bem mais rápido que o tempo real. Precisam do Piper, instalado em Configurações → Ferramentas."),
  embedding: msg("Usados na página Documentos. Modelos multilíngues funcionam melhor com textos em português."),
};

/** The thin llamafile build has no GPU support: recommendations assume CPU. */
function ThinNotice() {
  const app = useApp();
  const hw = app.hardware;
  if (!app.runtime?.thin || !hw || (hw.vramGb <= 0 && !hw.unifiedMemory)) return null;
  const downloading = app.downloads.some((d) => d.key === "tool:llamafile");
  return (
    <div className="mt-3 flex items-center gap-3 rounded-xl bg-warn-soft px-4 py-2.5 text-[12.5px]">
      <Zap className="size-4 shrink-0 text-warn" />
      <span className="flex-1">
        {t(
          "Você está usando a versão leve do llamafile (só CPU), então as recomendações abaixo não usam sua {gpu}. Instale a versão completa para respostas bem mais rápidas.",
          { gpu: hw.gpuName || "GPU" },
        )}
      </span>
      <Button
        size="sm"
        variant="primary"
        loading={downloading}
        onClick={() => api.downloadLlamafile(false).catch((e) => app.toast(String(e), "error"))}
      >
        {downloading ? t("Baixando…") : t("Usar a GPU")}
      </Button>
    </div>
  );
}

function HardwareCard() {
  const app = useApp();
  const hw = app.hardware;
  const [disk, setDisk] = useState<number | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  useEffect(() => {
    api.diskFree().then(setDisk);
  }, [app.catalogRevision]);
  if (!hw) return <Card className="h-[108px] animate-pulse">{null}</Card>;

  const memoryForModels = hw.unifiedMemory
    ? t("{size} memória unificada", { size: gb(hw.usableRamGb) })
    : hw.usableVramGb > 0
      ? `${gb(hw.usableVramGb)} VRAM + ${gb(hw.usableRamGb)} RAM`
      : `${gb(hw.usableRamGb)} RAM`;

  const items = [
    { icon: <Cpu className="size-4" />, label: t("Processador"), value: hw.cpuName, sub: t("{n} threads", { n: hw.cpuThreads }) },
    { icon: <MemoryStick className="size-4" />, label: t("Memória"), value: gb(hw.totalRamGb), sub: t("{size} livres agora", { size: gb(hw.availableRamGb) }) },
    {
      icon: <MonitorSmartphone className="size-4" />,
      label: t("Placa de vídeo"),
      value: hw.gpuName || t("Sem GPU dedicada"),
      sub: hw.vramGb > 0 ? `${gb(hw.vramGb)} VRAM · ${hw.backend.toUpperCase()}` : hw.unifiedMemory ? "Metal" : t("Modelos rodam na CPU"),
    },
    { icon: <HardDrive className="size-4" />, label: t("Disco livre"), value: disk === null ? "—" : gb(disk), sub: t("na pasta de modelos") },
  ];

  return (
    <Card className="p-4">
      <div className="mb-3 flex items-center justify-between">
        <div>
          <div className="text-[12px] font-medium text-muted">{t("Seu computador")}</div>
          <div className="text-[15px] font-semibold">
            {t("Máquina {tier} · até {memory} para modelos", { tier: hw.tier.toLowerCase(), memory: memoryForModels })}
          </div>
        </div>
        <IconButton
          title={t("Detectar hardware novamente")}
          onClick={async () => {
            setRefreshing(true);
            await api.refreshHardware().then(() => app.toast(t("Hardware atualizado.")));
            setRefreshing(false);
          }}
        >
          <RefreshCw className={`size-4 ${refreshing ? "animate-spin" : ""}`} />
        </IconButton>
      </div>
      <div className="grid grid-cols-4 gap-3">
        {items.map((it) => (
          <div key={it.label} className="min-w-0 rounded-lg bg-field px-3 py-2">
            <div className="flex items-center gap-1.5 text-[11.5px] text-muted">
              {it.icon}
              {it.label}
            </div>
            <div className="mt-0.5 truncate text-[13px] font-semibold" title={it.value}>
              {it.value}
            </div>
            <div className="truncate text-[11px] text-muted">{it.sub}</div>
          </div>
        ))}
      </div>
    </Card>
  );
}

export function DiscoverPage() {
  const app = useApp();
  const [category, setCategory] = useState(app.discoverCategory);
  useEffect(() => setCategory(app.discoverCategory), [app.discoverCategory]);
  const [onlyFits, setOnlyFits] = useState(true);
  const [models, setModels] = useState<ModelView[]>([]);

  useEffect(() => {
    api.recommended(category).then((l) => setModels(l ?? []));
  }, [category, app.catalogRevision, app.hardware]);

  const runnable = models.filter((m) => m.fit.level !== "tight");
  const best = runnable[0];
  const rest = useMemo(
    () => models.filter((m) => m.id !== best?.id && (!onlyFits || m.fit.level !== "tight")),
    [models, best, onlyFits],
  );
  const hidden = models.length - runnable.length;
  const cat = categories.find((c) => c.id === category)!;

  return (
    <div className="flex h-full min-w-0 flex-1 flex-col bg-bg">
      <header className="drag flex h-[52px] shrink-0 items-center border-b border-line px-6">
        <h1 className="text-[14px] font-semibold">{t("Descobrir modelos")}</h1>
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto max-w-[1040px] px-6 py-5">
          <HardwareCard />
          <ThinNotice />

          <div className="mt-6 mb-4 flex flex-wrap items-center justify-between gap-3">
            <Segmented
              value={category}
              onChange={setCategory}
              options={categories.map((c) => ({
                value: c.id,
                label: (
                  <>
                    {categoryIcons[c.id]}
                    {t(c.label)}
                  </>
                ),
              }))}
            />
            <label className="flex items-center gap-2 text-[12.5px] text-muted">
              {t("Mostrar só os que rodam aqui")}
              <Switch checked={onlyFits} onChange={setOnlyFits} />
            </label>
          </div>
          <p className="mb-4 text-[12.5px] text-muted">{t(cat.description)}</p>

          {best && (
            <div className="mb-6">
              <div className="mb-2 flex items-center gap-1.5 text-[12px] font-semibold text-accent">
                <Sparkles className="size-3.5" /> {t("Melhor escolha para o seu computador")}
              </div>
              <div className="grid grid-cols-2 gap-3">
                <ModelCard m={best} highlight />
                <div className="flex flex-col justify-center rounded-xl border border-dashed border-line p-4 text-[12.5px] text-muted">
                  <div className="mb-2 font-semibold text-fg">{t("Por que este modelo?")}</div>
                  <ul className="list-disc space-y-1 pl-4">
                    <li>
                      {t("Usa {size} de memória ({percent}% do disponível em {mode}).", {
                        size: gb(best.fit.requiredGb),
                        percent: Math.round(best.fit.utilization * 100),
                        mode: best.fit.mode,
                      })}
                    </li>
                    <li>
                      {t(
                        best.fit.measured
                          ? "Velocidade medida aqui: {speed} ({label})."
                          : best.fit.calibrated
                            ? "Velocidade estimada, calibrada pelas suas medições: {speed} ({label})."
                            : "Velocidade estimada: {speed} ({label}).",
                        { speed: speedText(best), label: best.fit.speedLabel.toLowerCase() },
                      )}
                    </li>
                    <li>
                      {t("Qualidade {score}/100 para {category} na quantização {quant}.", {
                        score: Math.round(best.fit.qualityScore),
                        category: t(cat.label).toLowerCase(),
                        quant: best.fit.quant,
                      })}
                    </li>
                    <li>
                      {t("Pontuação geral {score} — equilíbrio entre qualidade, velocidade e folga de memória.", { score: Math.round(best.fit.score) })}
                    </li>
                  </ul>
                </div>
              </div>
            </div>
          )}

          {categoryNotes[category] && (
            <div className="mb-4 rounded-lg bg-accent-soft px-3.5 py-2.5 text-[12.5px] text-fg/85">{t(categoryNotes[category])}</div>
          )}

          <div className="grid grid-cols-2 gap-3 xl:grid-cols-3">
            {rest.map((m) => (
              <ModelCard key={m.id} m={m} />
            ))}
          </div>
          {onlyFits && hidden > 0 && (
            <button onClick={() => setOnlyFits(false)} className="mt-4 text-[12.5px] text-accent hover:underline">
              {hidden === 1
                ? t("Mostrar {n} modelo que não cabe na sua máquina", { n: hidden })
                : t("Mostrar {n} modelos que não cabem na sua máquina", { n: hidden })}
            </button>
          )}
          <p className="mt-8 text-center text-[11px] text-faint">
            {t(
              "Estimativas baseadas na largura de banda de memória e no tamanho dos arquivos (método inspirado no llmfit). A velocidade real varia com o tamanho do contexto e outros programas abertos.",
            )}
          </p>
        </div>
      </div>
    </div>
  );
}
