import { Brain, Code2, Cpu, Eye, HardDrive, ImagePlus, Layers, MemoryStick, MessageSquare, Mic, MonitorSmartphone, RefreshCw, Sparkles, Volume2, Zap } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { ModelCard, speedText } from "../components/ModelCard";
import { Button, Card, IconButton, Segmented, Switch } from "../components/ui";
import { api } from "../lib/api";
import { gb } from "../lib/format";
import { useApp } from "../lib/store";
import type { ModelView } from "../lib/types";

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
  { id: "chat", label: "Chat", description: "Assistentes para conversar, escrever e resumir." },
  { id: "code", label: "Código", description: "Gerar, explicar e refatorar código." },
  { id: "reasoning", label: "Raciocínio", description: "Lógica, matemática e problemas passo a passo." },
  { id: "vision", label: "Visão", description: "Entendem imagens, capturas de tela e documentos." },
  { id: "image", label: "Imagens", description: "Geram imagens a partir de uma descrição em texto." },
  { id: "voice", label: "Voz", description: "Transcrição de áudio e ditado com Whisper." },
  { id: "embedding", label: "Embeddings", description: "Indexam seus documentos para o chat responder com base neles." },
  { id: "tts", label: "Leitura em voz alta", description: "Vozes para ouvir as respostas do chat." },
];

const categoryNotes: Record<string, string> = {
  image: "Modelos de imagem rodam com o diffusionfile, na CPU (ou Metal no Mac). Os tempos abaixo são estimados para este computador.",
  voice: "Modelos de voz rodam com o whisperfile, na CPU. \u201cTempo real\u201d significa que 1 minuto de áudio leva ~1 minuto para transcrever.",
  tts: "Vozes do Piper: leves (~60 MB), rodam na CPU bem mais rápido que o tempo real. Precisam do Piper, instalado em Configurações → Ferramentas.",
  embedding: "Usados na página Documentos. Modelos multilíngues funcionam melhor com textos em português.",
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
        Você está usando a versão leve do llamafile (só CPU), então as recomendações abaixo não usam sua {hw.gpuName || "GPU"}. Instale a versão
        completa para respostas bem mais rápidas.
      </span>
      <Button
        size="sm"
        variant="primary"
        loading={downloading}
        onClick={() => api.downloadLlamafile(false).catch((e) => app.toast(String(e), "error"))}
      >
        {downloading ? "Baixando…" : "Usar a GPU"}
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
    ? `${gb(hw.usableRamGb)} memória unificada`
    : hw.usableVramGb > 0
      ? `${gb(hw.usableVramGb)} VRAM + ${gb(hw.usableRamGb)} RAM`
      : `${gb(hw.usableRamGb)} RAM`;

  const items = [
    { icon: <Cpu className="size-4" />, label: "Processador", value: hw.cpuName, sub: `${hw.cpuThreads} threads` },
    { icon: <MemoryStick className="size-4" />, label: "Memória", value: gb(hw.totalRamGb), sub: `${gb(hw.availableRamGb)} livres agora` },
    {
      icon: <MonitorSmartphone className="size-4" />,
      label: "Placa de vídeo",
      value: hw.gpuName || "Sem GPU dedicada",
      sub: hw.vramGb > 0 ? `${gb(hw.vramGb)} VRAM · ${hw.backend.toUpperCase()}` : hw.unifiedMemory ? "Metal" : "Modelos rodam na CPU",
    },
    { icon: <HardDrive className="size-4" />, label: "Disco livre", value: disk === null ? "—" : gb(disk), sub: "na pasta de modelos" },
  ];

  return (
    <Card className="p-4">
      <div className="mb-3 flex items-center justify-between">
        <div>
          <div className="text-[12px] font-medium text-muted">Seu computador</div>
          <div className="text-[15px] font-semibold">
            Máquina {hw.tier.toLowerCase()} · até {memoryForModels} para modelos
          </div>
        </div>
        <IconButton
          title="Detectar hardware novamente"
          onClick={async () => {
            setRefreshing(true);
            await api.refreshHardware().then(() => app.toast("Hardware atualizado."));
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
        <h1 className="text-[14px] font-semibold">Descobrir modelos</h1>
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
                    {c.label}
                  </>
                ),
              }))}
            />
            <label className="flex items-center gap-2 text-[12.5px] text-muted">
              Mostrar só os que rodam aqui
              <Switch checked={onlyFits} onChange={setOnlyFits} />
            </label>
          </div>
          <p className="mb-4 text-[12.5px] text-muted">{cat.description}</p>

          {best && (
            <div className="mb-6">
              <div className="mb-2 flex items-center gap-1.5 text-[12px] font-semibold text-accent">
                <Sparkles className="size-3.5" /> Melhor escolha para o seu computador
              </div>
              <div className="grid grid-cols-2 gap-3">
                <ModelCard m={best} highlight />
                <div className="flex flex-col justify-center rounded-xl border border-dashed border-line p-4 text-[12.5px] text-muted">
                  <div className="mb-2 font-semibold text-fg">Por que este modelo?</div>
                  <ul className="list-disc space-y-1 pl-4">
                    <li>
                      Usa {gb(best.fit.requiredGb)} de memória ({Math.round(best.fit.utilization * 100)}% do disponível em{" "}
                      {best.fit.mode}).
                    </li>
                    <li>
                      {best.fit.measured ? "Velocidade medida aqui" : best.fit.calibrated ? "Velocidade estimada, calibrada pelas suas medições" : "Velocidade estimada"}:{" "}
                      {speedText(best)} ({best.fit.speedLabel.toLowerCase()}).
                    </li>
                    <li>Qualidade {Math.round(best.fit.qualityScore)}/100 para {cat.label.toLowerCase()} na quantização {best.fit.quant}.</li>
                    <li>Pontuação geral {Math.round(best.fit.score)} — equilíbrio entre qualidade, velocidade e folga de memória.</li>
                  </ul>
                </div>
              </div>
            </div>
          )}

          {categoryNotes[category] && (
            <div className="mb-4 rounded-lg bg-accent-soft px-3.5 py-2.5 text-[12.5px] text-fg/85">{categoryNotes[category]}</div>
          )}

          <div className="grid grid-cols-2 gap-3 xl:grid-cols-3">
            {rest.map((m) => (
              <ModelCard key={m.id} m={m} />
            ))}
          </div>
          {onlyFits && hidden > 0 && (
            <button onClick={() => setOnlyFits(false)} className="mt-4 text-[12.5px] text-accent hover:underline">
              Mostrar {hidden} {hidden === 1 ? "modelo que não cabe" : "modelos que não cabem"} na sua máquina
            </button>
          )}
          <p className="mt-8 text-center text-[11px] text-faint">
            Estimativas baseadas na largura de banda de memória e no tamanho dos arquivos (método inspirado no llmfit). A velocidade real varia
            com o tamanho do contexto e outros programas abertos.
          </p>
        </div>
      </div>
    </div>
  );
}
